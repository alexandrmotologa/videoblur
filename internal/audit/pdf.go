package audit

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// GenerateCourtCertificatePDF writes a formal legal certificate of redaction in PDF 1.4 format.
func GenerateCourtCertificatePDF(manifest *domain.AuditManifest, outputPath string) error {
	pdfBytes := buildPDFContent(manifest)
	return os.WriteFile(outputPath, pdfBytes, 0644)
}

func buildPDFContent(m *domain.AuditManifest) []byte {
	var body bytes.Buffer

	// Text streams
	writeText := func(text string, x, y float64, font string, size float64, r, g, b float64) {
		body.WriteString(fmt.Sprintf("BT\n/%s %.1f Tf\n%.2f %.2f %.2f rg\n1 0 0 1 %.2f %.2f Tm\n(%s) Tj\nET\n",
			font, size, r, g, b, x, y, escapePDFText(text)))
	}

	drawRect := func(x, y, w, h float64, r, g, b float64) {
		body.WriteString(fmt.Sprintf("%.2f %.2f %.2f rg\n%.2f %.2f %.2f %.2f re f\n", r, g, b, x, y, w, h))
	}

	drawStroke := func(x, y, w, h float64, r, g, b float64) {
		body.WriteString(fmt.Sprintf("%.2f %.2f %.2f RG\n0.75 w\n%.2f %.2f %.2f %.2f re S\n", r, g, b, x, y, w, h))
	}

	// 1. Header Banner
	drawRect(0, 750, 612, 42, 0.08, 0.12, 0.22) // dark slate navy header
	writeText("CERTIFICATE OF IRREVERSIBLE MULTIMODAL REDACTION", 40, 765, "F2", 14, 1.0, 1.0, 1.0)
	writeText("CJIS / HIPAA / FOIA STATUTORY COMPLIANCE & CHAIN OF CUSTODY", 40, 753, "F1", 8, 0.6, 0.75, 0.95)

	// 2. Case & File Metadata Box
	drawStroke(40, 610, 532, 120, 0.8, 0.82, 0.86)
	drawRect(40, 710, 532, 20, 0.94, 0.95, 0.97)
	writeText("CASE & EVIDENCE IDENTIFICATION", 48, 716, "F2", 9, 0.2, 0.25, 0.35)

	currY := 695.0
	writeText(fmt.Sprintf("Source File: %s", m.SourceVideo), 48, currY, "F1", 9, 0.1, 0.1, 0.1)
	currY -= 15
	writeText(fmt.Sprintf("Redacted Target: %s", m.RedactedVideo), 48, currY, "F1", 9, 0.1, 0.1, 0.1)
	currY -= 15
	writeText(fmt.Sprintf("Timestamp of Execution: %s", m.Timestamp.Format(time.RFC1123)), 48, currY, "F1", 9, 0.1, 0.1, 0.1)
	currY -= 15
	writeText(fmt.Sprintf("Compliance Standard: %s   |   Auditor Identity: %s", m.ComplianceCode, m.AuditorIdentity), 48, currY, "F1", 9, 0.1, 0.1, 0.1)
	currY -= 15
	writeText(fmt.Sprintf("Footage Profile: %dx%d @ %.2f FPS   |   Total Frames: %d (%.2f seconds)",
		m.VideoWidth, m.VideoHeight, m.FPS, m.TotalFrames, m.DurationSec), 48, currY, "F1", 9, 0.1, 0.1, 0.1)

	// 3. Entity Redaction Tally Box
	drawStroke(40, 480, 532, 110, 0.8, 0.82, 0.86)
	drawRect(40, 570, 532, 20, 0.94, 0.95, 0.97)
	writeText("STATUTORY REDACTION INVENTORY", 48, 576, "F2", 9, 0.2, 0.25, 0.35)

	currY = 552.0
	writeText(fmt.Sprintf("- Human Faces Anonymized: %d instances", m.VisualEntities.FacesDetected), 48, currY, "F1", 9, 0.15, 0.2, 0.3)
	currY -= 16
	writeText(fmt.Sprintf("- Vehicle License Plates Masked: %d instances", m.VisualEntities.PlatesDetected), 48, currY, "F1", 9, 0.15, 0.2, 0.3)
	currY -= 16
	writeText(fmt.Sprintf("- Computer Screens & EHR Workstations Censored: %d instances", m.VisualEntities.ScreensDetected), 48, currY, "F1", 9, 0.15, 0.2, 0.3)
	currY -= 16
	writeText(fmt.Sprintf("- Spoken PII Audio Bleeps (Phone, SSN, Address, Name): %d intervals", len(m.AudioEntities)), 48, currY, "F1", 9, 0.15, 0.2, 0.3)
	currY -= 16
	writeText(fmt.Sprintf("- Human Reviewer Manual Overrides & Exemptions: %d entries", len(m.ManualOverrides)), 48, currY, "F1", 9, 0.15, 0.2, 0.3)

	// 4. Non-Repudiation Cryptographic Hashes Box
	drawStroke(40, 310, 532, 150, 0.8, 0.82, 0.86)
	drawRect(40, 440, 532, 20, 0.94, 0.95, 0.97)
	writeText("CRYPTOGRAPHIC INTEGRITY & CHAIN OF CUSTODY (FRE 902(13))", 48, 446, "F2", 9, 0.2, 0.25, 0.35)

	currY = 420.0
	writeText("Cumulative Frame Hash Root (SHA-256 Merkle Chain):", 48, currY, "F2", 8.5, 0.2, 0.2, 0.2)
	currY -= 14
	writeText(m.FrameHashRoot, 48, currY, "F3", 8, 0.1, 0.3, 0.7)

	currY -= 20
	writeText("Ed25519 Digital Signature:", 48, currY, "F2", 8.5, 0.2, 0.2, 0.2)
	currY -= 14
	sigStr := m.SignatureHex
	if len(sigStr) > 75 {
		writeText(sigStr[:75], 48, currY, "F3", 7.5, 0.1, 0.5, 0.2)
		currY -= 10
		writeText(sigStr[75:], 48, currY, "F3", 7.5, 0.1, 0.5, 0.2)
	} else {
		writeText(sigStr, 48, currY, "F3", 7.5, 0.1, 0.5, 0.2)
	}

	currY -= 18
	writeText("Legal Attestation Statement:", 48, currY, "F2", 8, 0.3, 0.3, 0.3)
	currY -= 12
	writeText("The digital video file identified above has undergone automated multimodal redaction adhering strictly to", 48, currY, "F1", 7.5, 0.4, 0.4, 0.4)
	currY -= 10
	writeText("the parameters mandated by law. All biometric facial markers and spoken PII intervals have been irreversibly altered.", 48, currY, "F1", 7.5, 0.4, 0.4, 0.4)

	// 5. Footer & Verification Command
	drawStroke(40, 160, 532, 60, 0.85, 0.88, 0.92)
	writeText("Independent Court Verification Command:", 48, 202, "F2", 8.5, 0.1, 0.1, 0.1)
	writeText("videoblur certify -m redaction_audit.json", 48, 188, "F3", 9, 0.1, 0.4, 0.8)
	writeText("Verifies bitstream SHA-256 integrity and authenticates the Ed25519 digital signature.", 48, 172, "F1", 7.5, 0.5, 0.5, 0.5)

	writeText("Generated by videoblur Autonomous Legal Redaction Engine // Zero-Cloud Architecture", 140, 60, "F1", 8, 0.6, 0.6, 0.6)

	// PDF Structure Assembly
	streamLen := body.Len()

	var doc bytes.Buffer
	doc.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	obj1Offset := doc.Len()
	doc.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	obj2Offset := doc.Len()
	doc.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	obj3Offset := doc.Len()
	doc.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n")
	doc.WriteString("/Resources << /Font << /F1 4 0 R /F2 5 0 R /F3 6 0 R >> >>\n")
	doc.WriteString("/Contents 7 0 R >>\nendobj\n")

	// Object 4: Font Helvetica
	obj4Offset := doc.Len()
	doc.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	// Object 5: Font Helvetica-Bold
	obj5Offset := doc.Len()
	doc.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\nendobj\n")

	// Object 6: Font Courier
	obj6Offset := doc.Len()
	doc.WriteString("6 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>\nendobj\n")

	// Object 7: Stream
	obj7Offset := doc.Len()
	doc.WriteString(fmt.Sprintf("7 0 obj\n<< /Length %d >>\nstream\n", streamLen))
	doc.Write(body.Bytes())
	doc.WriteString("\nendstream\nendobj\n")

	// Cross-reference table
	xrefOffset := doc.Len()
	doc.WriteString("xref\n0 8\n")
	doc.WriteString("0000000000 65535 f \n")
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj1Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj2Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj3Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj4Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj5Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj6Offset))
	doc.WriteString(fmt.Sprintf("%010d 00000 n \n", obj7Offset))

	doc.WriteString("trailer\n<< /Size 8 /Root 1 0 R >>\nstartxref\n")
	doc.WriteString(fmt.Sprintf("%d\n%%%%EOF", xrefOffset))

	return doc.Bytes()
}

func escapePDFText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `(`, `\(`)
	s = strings.ReplaceAll(s, `)`, `\)`)
	return s
}
