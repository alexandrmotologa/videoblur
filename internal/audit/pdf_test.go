package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

func TestGenerateCourtCertificatePDF(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pdf_test_*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manifest := &domain.AuditManifest{
		ManifestVersion: "1.0.0",
		SourceVideo:     "incident_bodycam.mp4",
		RedactedVideo:   "incident_redacted.mp4",
		Timestamp:       time.Now(),
		ComplianceCode:  "CJIS",
		AuditorIdentity: "Officer-J-Doe-142",
		TotalFrames:     450,
		DurationSec:     15.0,
		VideoWidth:      1920,
		VideoHeight:     1080,
		FPS:             30.0,
		VisualEntities: domain.EntityStats{
			FacesDetected:   12,
			PlatesDetected:  2,
			ScreensDetected: 1,
		},
		AudioEntities: []domain.AudioEntity{
			{Class: "person_name", Text: "Jane Smith", StartMs: 1200, EndMs: 1600},
			{Class: "phone_number", Text: "555-0199", StartMs: 3200, EndMs: 3800},
		},
		FrameHashRoot: "a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0",
		SignatureHex:  "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}

	pdfPath := filepath.Join(tempDir, "redaction_certificate.pdf")
	if err := GenerateCourtCertificatePDF(manifest, pdfPath); err != nil {
		t.Fatalf("GenerateCourtCertificatePDF failed: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("Reading generated PDF failed: %v", err)
	}

	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Errorf("PDF header missing")
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Errorf("PDF EOF trailer missing")
	}
	if !bytes.Contains(data, []byte("CERTIFICATE OF IRREVERSIBLE MULTIMODAL REDACTION")) {
		t.Errorf("Missing legal header text in PDF")
	}
}
