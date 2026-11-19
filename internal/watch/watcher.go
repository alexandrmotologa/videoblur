package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/audit"
	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/media"
)

// WatcherOptions defines parameters for folder monitoring and automated processing.
type WatcherOptions struct {
	InboxDir    string
	OutboxDir   string
	ArchiveDir  string
	Concurrency int
	PollInterval time.Duration
	Config      *config.Config
	GeneratePDF bool
}

// Watcher monitors an inbox directory and dispatches redaction jobs across a worker pool.
type Watcher struct {
	opts       WatcherOptions
	processed  map[string]bool
	procMu     sync.Mutex
	jobQueue   chan string
	wg         sync.WaitGroup
	signingKey *audit.KeyPair
}

// NewWatcher initializes a watchdog daemon.
func NewWatcher(opts WatcherOptions) (*Watcher, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 2
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.Config == nil {
		opts.Config = config.DefaultConfig()
	}

	if err := os.MkdirAll(opts.InboxDir, 0755); err != nil {
		return nil, fmt.Errorf("creating inbox directory: %w", err)
	}
	if err := os.MkdirAll(opts.OutboxDir, 0755); err != nil {
		return nil, fmt.Errorf("creating outbox directory: %w", err)
	}
	if opts.ArchiveDir != "" {
		if err := os.MkdirAll(opts.ArchiveDir, 0755); err != nil {
			return nil, fmt.Errorf("creating archive directory: %w", err)
		}
	}

	kp, err := audit.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generating watchdog keypair: %w", err)
	}

	return &Watcher{
		opts:       opts,
		processed:  make(map[string]bool),
		jobQueue:   make(chan string, 128),
		signingKey: kp,
	}, nil
}

// Start begins folder polling and worker processing until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	fmt.Printf("[Watchdog] Monitoring folder: %s\n", w.opts.InboxDir)
	fmt.Printf("[Watchdog] Outbox target: %s (Concurrency: %d workers)\n", w.opts.OutboxDir, w.opts.Concurrency)

	// Launch worker pool
	for i := 1; i <= w.opts.Concurrency; i++ {
		w.wg.Add(1)
		go w.worker(ctx, i)
	}

	ticker := time.NewTicker(w.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(w.jobQueue)
			w.wg.Wait()
			return nil
		case <-ticker.C:
			w.scanInbox()
		}
	}
}

func (w *Watcher) scanInbox() {
	entries, err := os.ReadDir(w.opts.InboxDir)
	if err != nil {
		return
	}

	validExts := map[string]bool{
		".mp4": true, ".mov": true, ".mkv": true, ".avi": true,
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !validExts[ext] {
			continue
		}

		filePath := filepath.Join(w.opts.InboxDir, e.Name())

		w.procMu.Lock()
		if w.processed[filePath] {
			w.procMu.Unlock()
			continue
		}
		w.processed[filePath] = true
		w.procMu.Unlock()

		// Verify file stability (make sure copy/write has completed)
		if !isFileStable(filePath) {
			w.procMu.Lock()
			delete(w.processed, filePath) // retry next tick
			w.procMu.Unlock()
			continue
		}

		select {
		case w.jobQueue <- filePath:
			fmt.Printf("[Watchdog] Queued file: %s\n", e.Name())
		default:
			fmt.Printf("[Watchdog] Queue full, file delayed: %s\n", e.Name())
		}
	}
}

func (w *Watcher) worker(ctx context.Context, workerID int) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case filePath, ok := <-w.jobQueue:
			if !ok {
				return
			}
			w.processFile(workerID, filePath)
		}
	}
}

func (w *Watcher) processFile(workerID int, filePath string) {
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	outVideo := filepath.Join(w.opts.OutboxDir, baseName+"_redacted.mp4")
	outAudit := filepath.Join(w.opts.OutboxDir, baseName+"_audit.json")
	outPDF := filepath.Join(w.opts.OutboxDir, baseName+"_certificate.pdf")

	fmt.Printf("[Worker #%d] Processing: %s\n", workerID, filepath.Base(filePath))

	summary, err := media.RunRedactionPipeline(media.PipelineOptions{
		InputPath:  filePath,
		OutputPath: outVideo,
		AuditPath:  outAudit,
		Config:     w.opts.Config,
		KeyPair:    w.signingKey,
	})
	if err != nil {
		fmt.Printf("[Worker #%d] ERROR on %s: %v\n", workerID, filepath.Base(filePath), err)
		return
	}

	if w.opts.GeneratePDF {
		_ = audit.GenerateCourtCertificatePDF(summary.Manifest, outPDF)
	}

	fmt.Printf("[Worker #%d] COMPLETED: %s (%d frames, %d PII bleeps, %.1f FPS)\n",
		workerID, baseName, summary.TotalFrames, summary.AudioPIIBleeps, summary.EffectiveFPS)

	// Move to archive if specified
	if w.opts.ArchiveDir != "" {
		destArchive := filepath.Join(w.opts.ArchiveDir, filepath.Base(filePath))
		_ = os.Rename(filePath, destArchive)
	}
}

// isFileStable verifies file size has not changed over a brief check interval.
func isFileStable(filePath string) bool {
	fi1, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	time.Sleep(300 * time.Millisecond)
	fi2, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return fi1.Size() == fi2.Size() && fi1.Size() > 0
}
