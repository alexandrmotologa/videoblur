package vision

import (
	"sort"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// Track represents an individual object tracked temporally across frames.
type Track struct {
	ID              int
	Class           config.TargetClass
	Box             domain.Rect
	Confidence      float64
	KF              *KalmanBoxFilter
	Age             int
	HitCount        int
	TimeSinceUpdate int
	IsInterpolated  bool
}

// MultiTracker manages multiple visual object tracks and bridges occlusions.
type MultiTracker struct {
	tracks     []*Track
	nextID     int
	maxMissed  int
	minIoU     float64
	frameIndex int
}

// NewMultiTracker initializes a tracker with temporal tolerance.
func NewMultiTracker(maxMissed int, minIoU float64) *MultiTracker {
	if maxMissed <= 0 {
		maxMissed = 6
	}
	if minIoU <= 0 {
		minIoU = 0.25
	}
	return &MultiTracker{
		tracks:    make([]*Track, 0),
		nextID:    1,
		maxMissed: maxMissed,
		minIoU:    minIoU,
	}
}

// MatchPair holds indices for an assignment between track and detection.
type MatchPair struct {
	TrackIdx int
	DetIdx   int
	IoU      float64
}

// Update processes detections for the current frame and returns all active redaction targets.
func (mt *MultiTracker) Update(detections []domain.Detection) []domain.Detection {
	mt.frameIndex++

	// Step 1: Predict new locations for all existing tracks
	for _, trk := range mt.tracks {
		trk.Box = trk.KF.Predict()
		trk.Age++
		trk.TimeSinceUpdate++
		trk.IsInterpolated = true
	}

	// Step 2: Associate detections to existing tracks using IoU matching
	matchedTracks := make(map[int]bool)
	matchedDets := make(map[int]bool)

	// Build all potential candidate pairs where class matches
	pairs := make([]MatchPair, 0)
	for tIdx, trk := range mt.tracks {
		for dIdx, det := range detections {
			if trk.Class == det.Class {
				iou := trk.Box.IoU(det.Box)
				if iou >= mt.minIoU {
					pairs = append(pairs, MatchPair{
						TrackIdx: tIdx,
						DetIdx:   dIdx,
						IoU:      iou,
					})
				}
			}
		}
	}

	// Sort pairs descending by IoU (greedy maximum weight bipartite matching)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].IoU > pairs[j].IoU
	})

	for _, p := range pairs {
		if matchedTracks[p.TrackIdx] || matchedDets[p.DetIdx] {
			continue
		}
		matchedTracks[p.TrackIdx] = true
		matchedDets[p.DetIdx] = true

		trk := mt.tracks[p.TrackIdx]
		det := detections[p.DetIdx]

		// Update Kalman state with detected measurement
		trk.KF.Update(det.Box)
		trk.Box = trk.KF.StateToRect()
		trk.Confidence = det.Confidence
		trk.HitCount++
		trk.TimeSinceUpdate = 0
		trk.IsInterpolated = false
	}

	// Step 3: Create new tracks for unmatched detections
	for dIdx, det := range detections {
		if !matchedDets[dIdx] {
			newTrack := &Track{
				ID:              mt.nextID,
				Class:           det.Class,
				Box:             det.Box,
				Confidence:      det.Confidence,
				KF:              NewKalmanBoxFilter(det.Box),
				Age:             1,
				HitCount:        1,
				TimeSinceUpdate: 0,
				IsInterpolated:  false,
			}
			mt.nextID++
			mt.tracks = append(mt.tracks, newTrack)
		}
	}

	// Step 4: Prune dead tracks and collect active output targets
	activeTracks := make([]*Track, 0, len(mt.tracks))
	outputDets := make([]domain.Detection, 0)

	for _, trk := range mt.tracks {
		// If missed for too many consecutive frames, drop it
		if trk.TimeSinceUpdate > mt.maxMissed {
			continue
		}
		activeTracks = append(activeTracks, trk)

		// Sanity check dimensions
		if trk.Box.W <= 0 || trk.Box.H <= 0 {
			continue
		}

		outputDets = append(outputDets, domain.Detection{
			Class:      trk.Class,
			Confidence: trk.Confidence,
			Box:        trk.Box,
			TrackID:    trk.ID,
		})
	}

	mt.tracks = activeTracks
	return outputDets
}

// Reset clears all active tracks.
func (mt *MultiTracker) Reset() {
	mt.tracks = make([]*Track, 0)
	mt.nextID = 1
	mt.frameIndex = 0
}
