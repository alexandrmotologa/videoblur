package media

import (
	"fmt"
	"io"
	"os/exec"
)

// Demuxer reads uncompressed RGB24 video frames directly from an FFmpeg stdout pipe.
type Demuxer struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	width  int
	height int
	frameSize int
}

// NewDemuxer initializes an FFmpeg pipe to extract raw video frames.
func NewDemuxer(inputPath string, width, height int) (*Demuxer, error) {
	frameSize := width * height * 3
	cmd := exec.Command("ffmpeg",
		"-v", "error",
		"-i", inputPath,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg demuxer: %w", err)
	}

	return &Demuxer{
		cmd:       cmd,
		stdout:    stdout,
		width:     width,
		height:    height,
		frameSize: frameSize,
	}, nil
}

// ReadFrame fills frameBuf with exactly one RGB24 frame. Returns io.EOF at stream conclusion.
func (d *Demuxer) ReadFrame(frameBuf []byte) error {
	if len(frameBuf) < d.frameSize {
		return fmt.Errorf("buffer too small: need %d bytes, got %d", d.frameSize, len(frameBuf))
	}

	_, err := io.ReadFull(d.stdout, frameBuf[:d.frameSize])
	return err
}

// Close terminates the FFmpeg reader process.
func (d *Demuxer) Close() error {
	if d.stdout != nil {
		_ = d.stdout.Close()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Wait()
	}
	return nil
}
