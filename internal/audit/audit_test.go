package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

func TestAuditSigningAndVerification(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	payload := []byte("CJIS-Evidence-Chain-Of-Custody-Frame-Hash-Root-12345")
	sigHex := kp.SignMessage(payload)

	valid, err := VerifySignature(kp.PublicKey, payload, sigHex)
	if err != nil {
		t.Fatalf("Signature verification error: %v", err)
	}
	if !valid {
		t.Fatalf("Expected valid signature")
	}

	// Tampered payload must fail
	tamperedPayload := []byte("CJIS-Evidence-Chain-Of-Custody-Frame-Hash-Root-99999")
	tamperedValid, _ := VerifySignature(kp.PublicKey, tamperedPayload, sigHex)
	if tamperedValid {
		t.Fatalf("Tampered payload must not verify")
	}
}

func TestManifestBuilderSaveAndVerify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit_test_*")
	if err != nil {
		t.Fatalf("Creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Keypair error: %v", err)
	}

	builder := NewManifestBuilder("input.mp4", "redacted.mp4", "HIPAA", "officer-42", 1920, 1080, 30.0, 10.0, kp)

	// Record a few frames
	frameBytes := make([]byte, 1920*1080*3)
	builder.RecordFrame(0, 0.0, frameBytes, []domain.Detection{
		{Class: config.TargetFace, Box: domain.Rect{X: 10, Y: 10, W: 50, H: 50}},
	})
	builder.RecordFrame(1, 0.033, frameBytes, []domain.Detection{
		{Class: config.TargetLicensePlate, Box: domain.Rect{X: 100, Y: 100, W: 60, H: 20}},
	})

	builder.SetAudioEntities([]domain.AudioEntity{
		{Class: config.AudioPIIName, Text: "John Doe", StartMs: 1000, EndMs: 1400},
	})

	manifestPath := filepath.Join(tempDir, "redaction_audit.json")
	manifest, err := builder.SaveToFile(manifestPath)
	if err != nil {
		t.Fatalf("Saving manifest failed: %v", err)
	}

	if manifest.TotalFrames != 2 {
		t.Errorf("Expected 2 total frames, got %d", manifest.TotalFrames)
	}

	// Run verifier
	res, err := VerifyManifestFile(manifestPath)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	if !res.IsValid || !res.SignatureValid {
		t.Fatalf("Manifest verification failed: %+v", res)
	}
	if res.TotalRedactions != 3 { // 1 face + 1 plate + 1 audio
		t.Errorf("Expected 3 total redactions, got %d", res.TotalRedactions)
	}
}
