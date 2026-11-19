package vision

import (
	"bytes"
	"testing"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

func TestApplyRedactionModes(t *testing.T) {
	width := 100
	height := 100
	rect := domain.Rect{X: 20, Y: 20, W: 40, H: 40}

	// 1. Blackout test: verify pixels become 0
	rgbBlackout := make([]byte, width*height*3)
	for i := range rgbBlackout {
		rgbBlackout[i] = 200 // fill with light gray
	}
	ApplyRedaction(rgbBlackout, width, height, rect, config.ModeBlackout, 0, 0)

	// Check that target inside pixel is 0
	insideIdx := (25*width + 25) * 3
	if rgbBlackout[insideIdx] != 0 || rgbBlackout[insideIdx+1] != 0 || rgbBlackout[insideIdx+2] != 0 {
		t.Fatalf("Blackout failed to zero pixels inside target rect")
	}
	// Check that outside pixel is untouched (200)
	outsideIdx := (5*width + 5) * 3
	if rgbBlackout[outsideIdx] != 200 {
		t.Fatalf("Blackout modified pixels outside target rect")
	}

	// 2. Mosaic test: verify block averaging
	rgbMosaic := make([]byte, width*height*3)
	for i := range rgbMosaic {
		rgbMosaic[i] = byte(i % 256)
	}
	originalCopy := make([]byte, len(rgbMosaic))
	copy(originalCopy, rgbMosaic)

	ApplyRedaction(rgbMosaic, width, height, rect, config.ModeMosaic, 0, 10)
	if bytes.Equal(rgbMosaic, originalCopy) {
		t.Fatalf("Mosaic should have altered the frame buffer")
	}

	// 3. Gaussian blur test
	rgbGaussian := make([]byte, width*height*3)
	for i := range rgbGaussian {
		rgbGaussian[i] = byte((i * 17) % 256)
	}
	origGauss := make([]byte, len(rgbGaussian))
	copy(origGauss, rgbGaussian)

	ApplyRedaction(rgbGaussian, width, height, rect, config.ModeGaussianBlur, 10.0, 0)
	if bytes.Equal(rgbGaussian, origGauss) {
		t.Fatalf("Gaussian blur should have smoothed and altered pixels")
	}
}
