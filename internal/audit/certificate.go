package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// ManifestBuilder creates signed forensic compliance certificates.
type ManifestBuilder struct {
	sourceVideo     string
	redactedVideo   string
	standard        string
	auditorIdentity string
	width           int
	height          int
	fps             float64
	durationSec     float64
	keyPair         *KeyPair
	cumulativeHash  [32]byte
	totalFrames     int
	visualStats     domain.EntityStats
	audioEntities   []domain.AudioEntity
	frameSamples    []domain.FrameRecord
}

// NewManifestBuilder initializes the compliance manifest generator.
func NewManifestBuilder(source, redacted, standard, auditor string, w, h int, fps, dur float64, kp *KeyPair) *ManifestBuilder {
	if standard == "" {
		standard = "CJIS"
	}
	if auditor == "" {
		auditor = "videoblur-autonomous-agent"
	}
	return &ManifestBuilder{
		sourceVideo:     source,
		redactedVideo:   redacted,
		standard:        standard,
		auditorIdentity: auditor,
		width:           w,
		height:          h,
		fps:             fps,
		durationSec:     dur,
		keyPair:         kp,
		frameSamples:    make([]domain.FrameRecord, 0),
		audioEntities:   make([]domain.AudioEntity, 0),
	}
}

// RecordFrame updates the cumulative SHA-256 chain and counts detected visual entities.
func (mb *ManifestBuilder) RecordFrame(frameIndex int, timestampSec float64, frameBytes []byte, detections []domain.Detection) {
	mb.totalFrames++
	frameHash := sha256.Sum256(frameBytes)

	// Cumulative hash update: H_k = SHA-256(H_{k-1} || FrameHash)
	chainInput := append(mb.cumulativeHash[:], frameHash[:]...)
	mb.cumulativeHash = sha256.Sum256(chainInput)

	// Update entity statistics
	for _, d := range detections {
		switch d.Class {
		case "face":
			mb.visualStats.FacesDetected++
		case "license_plate":
			mb.visualStats.PlatesDetected++
		case "computer_screen":
			mb.visualStats.ScreensDetected++
		case "id_badge":
			mb.visualStats.BadgesDetected++
		}
	}

	// Keep periodic frame records (e.g. every 30th frame or key frames) for forensic sample inspection
	if frameIndex%30 == 0 || len(detections) > 0 && len(mb.frameSamples) < 100 {
		mb.frameSamples = append(mb.frameSamples, domain.FrameRecord{
			FrameIndex:  frameIndex,
			Timestamp:   timestampSec,
			Detections:  detections,
			FrameSHA256: hex.EncodeToString(frameHash[:]),
		})
	}
}

// SetAudioEntities registers identified spoken PII entities.
func (mb *ManifestBuilder) SetAudioEntities(entities []domain.AudioEntity) {
	mb.audioEntities = entities
}

// Build generates the immutable AuditManifest and applies cryptographic signing.
func (mb *ManifestBuilder) Build() (*domain.AuditManifest, error) {
	manifest := &domain.AuditManifest{
		ManifestVersion: "1.0.0",
		SourceVideo:     mb.sourceVideo,
		RedactedVideo:   mb.redactedVideo,
		Timestamp:       time.Now().UTC(),
		ComplianceCode:  mb.standard,
		AuditorIdentity: mb.auditorIdentity,
		TotalFrames:     mb.totalFrames,
		DurationSec:     mb.durationSec,
		VideoWidth:      mb.width,
		VideoHeight:     mb.height,
		FPS:             mb.fps,
		VisualEntities:  mb.visualStats,
		AudioEntities:   mb.audioEntities,
		TotalBleeps:     len(mb.audioEntities),
		FrameHashRoot:   hex.EncodeToString(mb.cumulativeHash[:]),
		FramesSample:    mb.frameSamples,
	}

	if mb.keyPair != nil {
		pubPEM, err := mb.keyPair.PublicKeyToPEM()
		if err != nil {
			return nil, fmt.Errorf("serializing public key: %w", err)
		}
		manifest.PublicKeyPEM = pubPEM

		// Serialize unsigned manifest payload for signing
		payloadBytes, err := json.Marshal(struct {
			Version       string             `json:"version"`
			Source        string             `json:"source"`
			Redacted      string             `json:"redacted"`
			RootHash      string             `json:"root_hash"`
			TotalFrames   int                `json:"total_frames"`
			VisualStats   domain.EntityStats `json:"visual_stats"`
			AudioEntities int                `json:"audio_entities_count"`
		}{
			Version:       manifest.ManifestVersion,
			Source:        manifest.SourceVideo,
			Redacted:      manifest.RedactedVideo,
			RootHash:      manifest.FrameHashRoot,
			TotalFrames:   manifest.TotalFrames,
			VisualStats:   manifest.VisualEntities,
			AudioEntities: len(manifest.AudioEntities),
		})
		if err != nil {
			return nil, fmt.Errorf("marshaling signing payload: %w", err)
		}

		manifest.SignatureHex = mb.keyPair.SignMessage(payloadBytes)
	}

	return manifest, nil
}

// SaveToFile writes the JSON audit manifest to the specified path.
func (mb *ManifestBuilder) SaveToFile(filePath string) (*domain.AuditManifest, error) {
	manifest, err := mb.Build()
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("writing audit file: %w", err)
	}

	return manifest, nil
}
