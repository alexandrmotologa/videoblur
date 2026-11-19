package audit

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// VerificationResult contains the forensic certification audit outcome.
type VerificationResult struct {
	IsValid          bool     `json:"is_valid"`
	SignatureValid   bool     `json:"signature_valid"`
	FrameHashesValid bool     `json:"frame_hashes_valid"`
	Standard         string   `json:"compliance_standard"`
	AuditorIdentity  string   `json:"auditor_identity"`
	TotalFrames      int      `json:"total_frames"`
	TotalRedactions  int      `json:"total_redactions"`
	Errors           []string `json:"errors,omitempty"`
}

// VerifyManifestFile verifies an audit manifest JSON file and its signature.
func VerifyManifestFile(manifestPath string) (*VerificationResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file: %w", err)
	}

	var manifest domain.AuditManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest JSON: %w", err)
	}

	res := &VerificationResult{
		IsValid:         true,
		Standard:        manifest.ComplianceCode,
		AuditorIdentity: manifest.AuditorIdentity,
		TotalFrames:     manifest.TotalFrames,
		TotalRedactions: manifest.VisualEntities.FacesDetected +
			manifest.VisualEntities.PlatesDetected +
			manifest.VisualEntities.ScreensDetected +
			manifest.VisualEntities.BadgesDetected +
			len(manifest.AudioEntities),
	}

	// Verify digital signature
	if manifest.PublicKeyPEM != "" && manifest.SignatureHex != "" {
		pubKey, err := ParsePublicKeyPEM(manifest.PublicKeyPEM)
		if err != nil {
			res.IsValid = false
			res.SignatureValid = false
			res.Errors = append(res.Errors, fmt.Sprintf("invalid public key PEM: %v", err))
		} else {
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
				res.IsValid = false
				res.SignatureValid = false
				res.Errors = append(res.Errors, fmt.Sprintf("payload marshal error: %v", err))
			} else {
				valid, err := VerifySignature(pubKey, payloadBytes, manifest.SignatureHex)
				if err != nil || !valid {
					res.IsValid = false
					res.SignatureValid = false
					res.Errors = append(res.Errors, "cryptographic digital signature verification failed: manifest has been altered")
				} else {
					res.SignatureValid = true
				}
			}
		}
	} else {
		res.SignatureValid = false
		res.Errors = append(res.Errors, "manifest does not contain a cryptographic digital signature")
	}

	return res, nil
}
