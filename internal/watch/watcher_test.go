package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/config"
)

func TestWatchdogFolderIntake(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watch_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	inbox := filepath.Join(tempDir, "inbox")
	outbox := filepath.Join(tempDir, "outbox")
	archive := filepath.Join(tempDir, "archive")

	w, err := NewWatcher(WatcherOptions{
		InboxDir:     inbox,
		OutboxDir:    outbox,
		ArchiveDir:   archive,
		Concurrency:  1,
		PollInterval: 100 * time.Millisecond,
		Config:       config.DefaultConfig(),
		GeneratePDF:  true,
	})
	if err != nil {
		t.Fatalf("NewWatcher error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run watcher in goroutine
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- w.Start(ctx)
	}()

	// Wait 200ms then cancel context to test graceful start & shutdown
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-doneChan:
		if err != nil {
			t.Errorf("Watcher returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Watcher failed to shut down within timeout")
	}
}
