package media

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/audio"
	"github.com/alexandrmotologa/videoblur/internal/audit"
	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
	"github.com/alexandrmotologa/videoblur/internal/vision"
)

// PipelineOptions specifies execution parameters for redacting a media file.
type PipelineOptions struct {
	InputPath        string
	OutputPath       string
	AuditPath        string
	Config           *config.Config
	KeyPair          *audit.KeyPair
	Overrides        []domain.ManualOverride
	ProgressCallback func(frameIdx, totalFrames int, fps float64)
}

// PipelineSummary contains high-level results from a redaction run.
type PipelineSummary struct {
	TotalFrames       int                   `json:"total_frames"`
	DurationSec       float64               `json:"duration_sec"`
	ProcessingTimeSec float64               `json:"processing_time_sec"`
	EffectiveFPS      float64               `json:"effective_fps"`
	VisualDetections  int                   `json:"visual_detections"`
	AudioPIIBleeps    int                   `json:"audio_pii_bleeps"`
	Manifest          *domain.AuditManifest `json:"manifest"`
}

type framePayload struct {
	idx  int
	data []byte
}

// RunRedactionPipeline executes the end-to-end video and audio anonymization pipeline
// using a decoupled asynchronous buffer pipeline for high-throughput encoding.
func RunRedactionPipeline(opts PipelineOptions) (*PipelineSummary, error) {
	startTime := time.Now()
	cfg := opts.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// 1. Probe input video metadata
	info, err := ProbeMedia(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("probing input media: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "videoblur_pipe_*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Audio stream extraction and acoustic modulation
	var audioEntities []domain.AudioEntity
	redactedAudioPath := ""

	if info.HasAudio {
		rawWavPath := filepath.Join(tempDir, "audio_16k.wav")
		redactedAudioPath = filepath.Join(tempDir, "redacted_audio.wav")

		extractCmd := exec.Command("ffmpeg", "-y", "-v", "error",
			"-i", opts.InputPath,
			"-vn",
			"-acodec", "pcm_s16le",
			"-ar", "16000",
			"-ac", "1",
			rawWavPath,
		)
		if err := extractCmd.Run(); err == nil {
			aligner := audio.NewPIIAligner(cfg.Audio.EnabledClasses)
			bleeper := audio.NewAudioBleeper(
				cfg.Audio.CensorMode,
				cfg.Audio.TargetChannel,
				cfg.Audio.BleepFrequencyHz,
				cfg.Audio.BleepVolume,
				cfg.Audio.CrossfadeMs,
			)

			asr := audio.NewWhisperASR(cfg.Audio.WhisperModelPath)
			words, fullText, err := asr.TranscribeAudio(rawWavPath)
			if err == nil {
				audioEntities = aligner.FindPIIEntities(words, fullText)
			}

			wavData, err := os.ReadFile(rawWavPath)
			if err == nil && len(wavData) > 44 {
				pcmHeader := wavData[:44]
				pcmPayload := make([]byte, len(wavData)-44)
				copy(pcmPayload, wavData[44:])

				bleepIntervals := audio.MergeIntervalsForBleeper(audioEntities)
				bleeper.ProcessPCM16(pcmPayload, 16000, 1, bleepIntervals)

				finalWav := append(pcmHeader, pcmPayload...)
				_ = os.WriteFile(redactedAudioPath, finalWav, 0644)
			} else {
				redactedAudioPath = rawWavPath
			}
		}
	}

	// 3. Initialize visual detector and Kalman multi-object tracker
	detector, err := vision.NewVisualDetector("")
	if err != nil {
		return nil, fmt.Errorf("initializing visual detector: %w", err)
	}
	defer detector.Close()

	tracker := vision.NewMultiTracker(cfg.Visual.TrackingMaxMissed, cfg.Visual.TrackingIoUMin)

	// 4. Initialize Cryptographic Manifest Builder
	manifestBuilder := audit.NewManifestBuilder(
		opts.InputPath,
		opts.OutputPath,
		cfg.Compliance.Standard,
		cfg.Compliance.AuditorIdentity,
		info.Width,
		info.Height,
		info.FPS,
		info.DurationSec,
		opts.KeyPair,
	)
	manifestBuilder.SetAudioEntities(audioEntities)
	manifestBuilder.SetManualOverrides(opts.Overrides)

	// 5. Open video demuxer pipe and muxer encoder pipe
	demuxer, err := NewDemuxer(opts.InputPath, info.Width, info.Height)
	if err != nil {
		return nil, fmt.Errorf("opening demuxer: %w", err)
	}
	defer demuxer.Close()

	muxer, err := NewMuxer(opts.OutputPath, redactedAudioPath, info.Width, info.Height, info.FPS, cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("opening muxer: %w", err)
	}
	defer muxer.Close()

	frameSize := info.Width * info.Height * 3

	// Buffer pool for zero-allocation recycling across frames
	bufferPool := sync.Pool{
		New: func() any {
			return make([]byte, frameSize)
		},
	}

	// Asynchronous worker channels
	readChan := make(chan framePayload, 16)
	writeChan := make(chan framePayload, 16)
	errChan := make(chan error, 2)

	// Reader goroutine: continuously pumps raw uncompressed frames from demuxer
	go func() {
		defer close(readChan)
		idx := 0
		for {
			buf := bufferPool.Get().([]byte)
			err := demuxer.ReadFrame(buf)
			if err == io.EOF {
				bufferPool.Put(buf)
				break
			}
			if err != nil {
				bufferPool.Put(buf)
				errChan <- fmt.Errorf("demuxing error at frame %d: %w", idx, err)
				return
			}
			readChan <- framePayload{idx: idx, data: buf}
			idx++
		}
	}()

	// Writer goroutine: feeds redacted frames asynchronously to FFmpeg encoder stdin
	var writeWg sync.WaitGroup
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		for item := range writeChan {
			if err := muxer.WriteFrame(item.data); err != nil {
				select {
				case errChan <- fmt.Errorf("muxing error at frame %d: %w", item.idx, err):
				default:
				}
			}
			bufferPool.Put(item.data)
		}
	}()

	// Processing thread: visual detection, Kalman state tracking, blur filtering, hash chain
	totalDetectionsCount := 0
	processedFrames := 0

	for item := range readChan {
		frameIdx := item.idx
		frameBuf := item.data
		timestampSec := float64(frameIdx) / info.FPS

		// Multi-class detection
		rawDetections, _ := detector.Detect(frameBuf, info.Width, info.Height, cfg.Visual.EnabledClasses, cfg.Visual.ConfidenceThreshold)

		// Kalman filter temporal tracking (bridges occlusions, eliminates flicker)
		activeTargets := tracker.Update(rawDetections)

		// Apply human reviewer exemptions and deletions
		exemptTracks := make(map[int]bool)
		for _, ov := range opts.Overrides {
			if ov.Action == domain.ActionExempt || ov.Action == domain.ActionDelete {
				if frameIdx >= ov.StartFrame && (ov.EndFrame == 0 || frameIdx <= ov.EndFrame) {
					exemptTracks[ov.TrackID] = true
				}
			}
		}

		filteredTargets := make([]domain.Detection, 0, len(activeTargets))
		for _, target := range activeTargets {
			if !exemptTracks[target.TrackID] {
				filteredTargets = append(filteredTargets, target)
			}
		}

		// Inject user-drawn manual bounding box additions
		for _, ov := range opts.Overrides {
			if ov.Action == domain.ActionAddBox {
				if frameIdx >= ov.StartFrame && (ov.EndFrame == 0 || frameIdx <= ov.EndFrame) {
					filteredTargets = append(filteredTargets, domain.Detection{
						Class:      ov.Class,
						Confidence: 1.0,
						Box:        ov.Box,
						TrackID:    -1,
					})
				}
			}
		}

		activeTargets = filteredTargets
		totalDetectionsCount += len(activeTargets)

		// Apply visual redactions with safety margin expansion
		for _, target := range activeTargets {
			paddedBox := target.Box.Expand(cfg.Visual.PaddingRatio, info.Width, info.Height)
			vision.ApplyRedaction(
				frameBuf,
				info.Width,
				info.Height,
				paddedBox,
				cfg.Visual.DefaultMode,
				cfg.Visual.BlurSigma,
				cfg.Visual.MosaicBlockSize,
			)
		}

		// Update cumulative cryptographic audit hash
		manifestBuilder.RecordFrame(frameIdx, timestampSec, frameBuf, activeTargets)

		// Dispatch to writer pipeline
		writeChan <- item
		processedFrames++

		if opts.ProgressCallback != nil && (processedFrames%15 == 0 || processedFrames == info.TotalFrames) {
			elapsed := time.Since(startTime).Seconds()
			curFPS := float64(processedFrames) / maxFloat(0.001, elapsed)
			opts.ProgressCallback(processedFrames, info.TotalFrames, curFPS)
		}
	}

	close(writeChan)
	writeWg.Wait()

	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	// Finalize muxer container
	if err := muxer.Close(); err != nil {
		return nil, fmt.Errorf("finalizing muxer: %w", err)
	}

	// Save cryptographic audit certificate
	auditFile := opts.AuditPath
	if auditFile == "" {
		auditFile = "redaction_audit.json"
	}
	manifest, err := manifestBuilder.SaveToFile(auditFile)
	if err != nil {
		return nil, fmt.Errorf("saving audit manifest: %w", err)
	}

	processingDuration := time.Since(startTime).Seconds()
	effFPS := float64(processedFrames) / maxFloat(0.001, processingDuration)

	return &PipelineSummary{
		TotalFrames:       processedFrames,
		DurationSec:       info.DurationSec,
		ProcessingTimeSec: processingDuration,
		EffectiveFPS:      effFPS,
		VisualDetections:  totalDetectionsCount,
		AudioPIIBleeps:    len(audioEntities),
		Manifest:          manifest,
	}, nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
