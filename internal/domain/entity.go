package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/config"
)

// Rect represents an integer bounding box in image pixel coordinates.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Center returns the horizontal and vertical center coordinates.
func (r Rect) Center() (float64, float64) {
	return float64(r.X) + float64(r.W)/2.0, float64(r.Y) + float64(r.H)/2.0
}

// Area returns bounding box area in square pixels.
func (r Rect) Area() float64 {
	return float64(r.W * r.H)
}

// AspectRatio returns width / height.
func (r Rect) AspectRatio() float64 {
	if r.H == 0 {
		return 1.0
	}
	return float64(r.W) / float64(r.H)
}

// IoU calculates Intersection over Union with another rectangle.
func (r Rect) IoU(o Rect) float64 {
	x1 := max(r.X, o.X)
	y1 := max(r.Y, o.Y)
	x2 := min(r.X+r.W, o.X+o.W)
	y2 := min(r.Y+r.H, o.Y+o.H)

	interW := max(0, x2-x1)
	interH := max(0, y2-y1)
	interArea := float64(interW * interH)

	unionArea := r.Area() + o.Area() - interArea
	if unionArea <= 0 {
		return 0.0
	}
	return interArea / unionArea
}

// Expand adds a percentage padding around the rectangle, bounded by image dimensions.
func (r Rect) Expand(ratio float64, maxWidth, maxHeight int) Rect {
	padX := int(float64(r.W) * ratio)
	padY := int(float64(r.H) * ratio)

	newX := max(0, r.X-padX)
	newY := max(0, r.Y-padY)
	newW := min(maxWidth-newX, r.W+padX*2)
	newH := min(maxHeight-newY, r.H+padY*2)

	return Rect{X: newX, Y: newY, W: newW, H: newH}
}

// Detection represents an object detected in a single visual frame.
type Detection struct {
	Class      config.TargetClass `json:"class"`
	Confidence float64            `json:"confidence"`
	Box        Rect               `json:"box"`
	TrackID    int                `json:"track_id,omitempty"`
}

// AudioEntity represents an identified sensitive spoken interval.
type AudioEntity struct {
	Class      config.AudioPIIClass `json:"class"`
	Text       string               `json:"text"`
	StartMs    int64                `json:"start_ms"`
	EndMs      int64                `json:"end_ms"`
	Confidence float64              `json:"confidence"`
}

// OverrideAction defines human officer review actions.
type OverrideAction string

const (
	ActionAddBox OverrideAction = "add"
	ActionExempt OverrideAction = "exempt"
	ActionDelete OverrideAction = "delete"
)

// ManualOverride records an officer's manual bounding box edit or exemption.
type ManualOverride struct {
	ID         string             `json:"id"`
	Action     OverrideAction     `json:"action"`
	TrackID    int                `json:"track_id,omitempty"`
	Class      config.TargetClass `json:"class"`
	StartFrame int                `json:"start_frame"`
	EndFrame   int                `json:"end_frame"`
	Box        Rect               `json:"box"`
}

// FrameRecord stores verification details for a single video frame.
type FrameRecord struct {
	FrameIndex int         `json:"frame_index"`
	Timestamp  float64     `json:"timestamp_sec"`
	Detections []Detection `json:"detections"`
	FrameSHA256 string     `json:"frame_sha256"`
}

// ComputeFrameHash generates a hex-encoded SHA-256 digest of raw frame bytes.
func ComputeFrameHash(frameBytes []byte) string {
	h := sha256.Sum256(frameBytes)
	return hex.EncodeToString(h[:])
}

// AuditManifest holds the legal compliance proof for an anonymized video.
type AuditManifest struct {
	ManifestVersion string        `json:"manifest_version"`
	SourceVideo     string        `json:"source_video"`
	RedactedVideo   string        `json:"redacted_video"`
	Timestamp       time.Time     `json:"timestamp"`
	ComplianceCode  string        `json:"compliance_standard"`
	AuditorIdentity string        `json:"auditor_identity"`
	TotalFrames     int           `json:"total_frames"`
	DurationSec     float64       `json:"duration_sec"`
	VideoWidth      int           `json:"video_width"`
	VideoHeight     int           `json:"video_height"`
	FPS             float64       `json:"fps"`
	VisualEntities  EntityStats      `json:"visual_entities"`
	AudioEntities   []AudioEntity    `json:"audio_entities"`
	ManualOverrides []ManualOverride `json:"manual_overrides,omitempty"`
	TotalBleeps     int              `json:"total_bleeps"`
	FrameHashRoot   string        `json:"frame_hash_root"`
	FramesSample    []FrameRecord `json:"sample_frames,omitempty"`
	SignatureHex    string        `json:"signature_hex,omitempty"`
	PublicKeyPEM    string        `json:"public_key_pem,omitempty"`
}

// EntityStats tallies detection counts by target class.
type EntityStats struct {
	FacesDetected   int `json:"faces_detected"`
	PlatesDetected  int `json:"license_plates_detected"`
	ScreensDetected int `json:"screens_detected"`
	BadgesDetected  int `json:"id_badges_detected"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *AuditManifest) Summary() string {
	return fmt.Sprintf("AuditManifest: %d frames (%.2fs), Faces: %d, Plates: %d, Screens: %d, Audio PII bleeps: %d",
		m.TotalFrames, m.DurationSec,
		m.VisualEntities.FacesDetected,
		m.VisualEntities.PlatesDetected,
		m.VisualEntities.ScreensDetected,
		len(m.AudioEntities),
	)
}
