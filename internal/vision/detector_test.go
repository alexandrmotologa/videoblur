package vision

import (
	"testing"

	"github.com/alexandrmotologa/videoblur/internal/config"
)

func TestVisualDetectorFaceDetection(t *testing.T) {
	width := 200
	height := 200
	rgb := make([]byte, width*height*3)

	// Draw a simulated face rectangle with skin chrominance (Cb ~ 100, Cr ~ 150)
	// In RGB: R ~ 210, G ~ 160, B ~ 130
	for y := 40; y < 100; y++ {
		for x := 40; x < 100; x++ {
			idx := (y*width + x) * 3
			rgb[idx] = 210   // R
			rgb[idx+1] = 160 // G
			rgb[idx+2] = 130 // B
		}
	}

	detector, err := NewVisualDetector("")
	if err != nil {
		t.Fatalf("Failed to create visual detector: %v", err)
	}
	defer detector.Close()

	dets, err := detector.Detect(rgb, width, height, []config.TargetClass{config.TargetFace}, 0.30)
	if err != nil {
		t.Fatalf("Detection error: %v", err)
	}

	if len(dets) == 0 {
		t.Fatalf("Expected at least one face detection in simulated frame, got 0")
	}

	if dets[0].Class != config.TargetFace {
		t.Errorf("Expected class %s, got %s", config.TargetFace, dets[0].Class)
	}
}
