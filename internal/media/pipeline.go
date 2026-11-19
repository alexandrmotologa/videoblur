package media

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	ProgressCallback func(frameIdx, totalFrames int, fps float64)
}

// PipelineSummary contains high-level results from a redaction run.
type PipelineSummary struct {
	TotalFrames      int           `json:"total_frames"`
	DurationSec      float64       `json:"duration_sec"`
	ProcessingTimeSec float64      `json:"processing_time_sec"`
	EffectiveFPS     float64       `json:"effective_fps"`
	VisualDetections int           `json:"visual_detections"`
	AudioPIIBleeps   int           `json:"audio_pii_bleeps"`
	Manifest         *domain.AuditManifest `json:"manifest"`
}

// RunRedactionPipeline executes the end-to-end video and audio anonymization pipeline.
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

	// 2. Audio stream extraction and PII bleeping
	var audioEntities []domain.AudioEntity
	redactedAudioPath := ""

	if info.HasAudio {
		rawWavPath := filepath.Join(tempDir, "audio_16k.wav")
		redactedAudioPath = filepath.Join(tempDir, "redacted_audio.wav")

		// Extract 16kHz mono PCM for Whisper and bleeping
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
			bleeper := audio.NewAudioBleeper(cfg.Audio.BleepFrequencyHz, cfg.Audio.BleepVolume, cfg.Audio.MuteOnly, cfg.Audio.CrossfadeMs)

			asr := audio.NewWhisperASR(cfg.Audio.WhisperModelPath)
			words, fullText, err := asr.TranscribeAudio(rawWavPath)
			if err == nil {
				audioEntities = aligner.FindPIIEntities(words, fullText)
			}

			// Read WAV bytes and apply bleep modulation over PCM data
			wavData, err := os.ReadFile(rawWavPath)
			if err == nil && len(wavData) > 44 { // 44 bytes is standard RIFF WAV header
				pcmHeader := wavData[:44]
				pcmPayload := make([]byte, len(wavData)-44)
				copy(pcmPayload, wavData[44:])

				bleepIntervals := audio.MergeIntervalsForBleeper(audioEntities)
				bleeper.ProcessPCM16(pcmPayload, 16000, 1, bleepIntervals)

				// Write back modulated WAV
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

	// 6. Frame processing loop
	frameSize := info.Width * info.Height * 3
	frameBuf := make([]byte, frameSize)
	frameIdx := 0
	totalDetectionsCount := 0

	for {
		err := demuxer.ReadFrame(frameBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading frame %d: %w", frameIdx, err)
		}

		timestampSec := float64(frameIdx) / info.FPS

		// Run detection
		rawDetections, err := detector.Detect(frameBuf, info.Width, info.Height, cfg.Visual.EnabledClasses, cfg.Visual.ConfidenceThreshold)
		if err != nil {
			rawDetections = nil
		}

		// Update temporal Kalman tracker (bridges occlusions, eliminates flicker)
		activeTargets := tracker.Update(rawDetections)
		totalDetectionsCount += len(activeTargets)

		// Apply visual redactions with safety padding
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

		// Record cryptographic frame state
		manifestBuilder.RecordFrame(frameIdx, timestampSec, frameBuf, activeTargets)

		// Write redacted frame to encoder
		if err := muxer.WriteFrame(frameBuf); err != nil {
			return nil, fmt.Errorf("writing frame %d: %w", frameIdx, err)
		}

		frameIdx++

		if opts.ProgressCallback != nil && (frameIdx%15 == 0 || frameIdx == info.TotalFrames) {
			elapsed := time.Since(startTime).Seconds()
			curFPS := float64(frameIdx) / maxFloat(0.001, elapsed)
			opts.ProgressCallback(frameIdx, info.TotalFrames, curFPS)
		}
	}

	// Finalize muxer
	if err := muxer.Close(); err != nil {
		return nil, fmt.Errorf("finalizing muxer: %w", err)
	}

	// 7. Save cryptographic audit certificate
	auditFile := opts.AuditPath
	if auditFile == "" {
		auditFile = "redaction_audit.json"
	}
	manifest, err := manifestBuilder.SaveToFile(auditFile)
	if err != nil {
		return nil, fmt.Errorf("saving audit manifest: %w", err)
	}

	processingDuration := time.Since(startTime).Seconds()
	effFPS := float64(frameIdx) / maxFloat(0.001, processingDuration)

	return &PipelineSummary{
		TotalFrames:       frameIdx,
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
