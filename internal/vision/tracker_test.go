package vision

import (
	"testing"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

func TestKalmanFilterPredictAndUpdate(t *testing.T) {
	initialBox := domain.Rect{X: 100, Y: 100, W: 50, H: 60}
	kf := NewKalmanBoxFilter(initialBox)

	// Step forward without measurement: position should remain or project smoothly
	predicted := kf.Predict()
	if predicted.W <= 0 || predicted.H <= 0 {
		t.Fatalf("Predicted box has invalid dimensions: %+v", predicted)
	}

	// Update with slightly moved measurement
	newBox := domain.Rect{X: 105, Y: 102, W: 50, H: 60}
	kf.Update(newBox)

	updated := kf.StateToRect()
	if updated.X < 95 || updated.X > 115 {
		t.Errorf("Unexpected updated X coordinate: %d", updated.X)
	}
}

func TestMultiTrackerOcclusionBridging(t *testing.T) {
	tracker := NewMultiTracker(5, 0.25)

	// Frame 1: Detection arrives
	det1 := domain.Detection{
		Class:      config.TargetFace,
		Confidence: 0.95,
		Box:        domain.Rect{X: 100, Y: 100, W: 80, H: 80},
	}
	out1 := tracker.Update([]domain.Detection{det1})
	if len(out1) != 1 {
		t.Fatalf("Expected 1 active track on frame 1, got %d", len(out1))
	}
	trackID := out1[0].TrackID

	// Frame 2: Face moves slightly
	det2 := domain.Detection{
		Class:      config.TargetFace,
		Confidence: 0.92,
		Box:        domain.Rect{X: 105, Y: 102, W: 80, H: 80},
	}
	out2 := tracker.Update([]domain.Detection{det2})
	if len(out2) != 1 || out2[0].TrackID != trackID {
		t.Fatalf("Track ID must persist on frame 2: got %+v", out2)
	}

	// Frames 3, 4, 5: Detector completely misses the face (occlusion / motion blur)
	for f := 3; f <= 5; f++ {
		outEmpty := tracker.Update([]domain.Detection{})
		if len(outEmpty) != 1 {
			t.Fatalf("Anti-flicker failure: frame %d lost face track during occlusion!", f)
		}
		if outEmpty[0].TrackID != trackID {
			t.Errorf("Frame %d track ID mismatch: expected %d, got %d", f, trackID, outEmpty[0].TrackID)
		}
	}

	// Frame 6: Missed for 4 consecutive frames (still within maxMissed=5)
	outF6 := tracker.Update([]domain.Detection{})
	if len(outF6) != 1 {
		t.Fatalf("Expected track maintained on 4th missed frame, got %d", len(outF6))
	}

	// Frames 7, 8, 9: Missed beyond threshold -> track should be pruned
	tracker.Update([]domain.Detection{})
	tracker.Update([]domain.Detection{})
	outDead := tracker.Update([]domain.Detection{})
	if len(outDead) != 0 {
		t.Fatalf("Expected dead track to be pruned after maxMissed frames, got %d tracks", len(outDead))
	}
}
