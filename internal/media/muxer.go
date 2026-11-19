package media

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/alexandrmotologa/videoblur/internal/config"
)

// Muxer feeds uncompressed RGB24 video frames into FFmpeg via stdin pipe and remuxes with audio.
type Muxer struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

// NewMuxer initializes the FFmpeg encoder process.
func NewMuxer(outputPath, audioPath string, width, height int, fps float64, outCfg config.OutputConfig) (*Muxer, error) {
	fpsStr := fmt.Sprintf("%.2f", fps)
	sizeStr := fmt.Sprintf("%dx%d", width, height)

	args := []string{
		"-y",
		"-v", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-s", sizeStr,
		"-r", fpsStr,
		"-i", "pipe:0",
	}

	if audioPath != "" {
		args = append(args, "-i", audioPath)
	}

	// Video encoding settings
	codec := outCfg.VideoCodec
	if codec == "" {
		codec = "libx264"
	}

	crf := outCfg.CRF
	if crf <= 0 {
		crf = 18
	}

	preset := outCfg.Preset
	if preset == "" {
		preset = "fast"
	}

	args = append(args,
		"-c:v", codec,
		"-preset", preset,
		"-crf", strconv.Itoa(crf),
		"-pix_fmt", "yuv420p", // standard player compatibility
	)

	if audioPath != "" {
		audioCodec := outCfg.AudioCodec
		if audioCodec == "" {
			audioCodec = "aac"
		}
		args = append(args,
			"-c:a", audioCodec,
			"-b:a", "192k",
			"-shortest",
		)
	} else {
		args = append(args, "-an")
	}

	args = append(args, outputPath)

	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating ffmpeg stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg muxer: %w", err)
	}

	return &Muxer{
		cmd:   cmd,
		stdin: stdin,
	}, nil
}

// WriteFrame pipes an RGB24 frame into the encoder.
func (m *Muxer) WriteFrame(frameBuf []byte) error {
	_, err := m.stdin.Write(frameBuf)
	return err
}

// Close finishes writing and waits for the encoder process to exit cleanly.
func (m *Muxer) Close() error {
	var closeErr error
	if m.stdin != nil {
		closeErr = m.stdin.Close()
	}
	if m.cmd != nil {
		waitErr := m.cmd.Wait()
		if closeErr != nil {
			return closeErr
		}
		return waitErr
	}
	return closeErr
}
