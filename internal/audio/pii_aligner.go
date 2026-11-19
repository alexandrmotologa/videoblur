package audio

import (
	"regexp"
	"strings"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// TranscriptWord represents a word with start and end timestamps.
type TranscriptWord struct {
	Word    string  `json:"word"`
	StartMs int64   `json:"start_ms"`
	EndMs   int64   `json:"end_ms"`
	Conf    float64 `json:"confidence"`
}

// PIIAligner scans transcript words and extracts timestamped PII intervals.
type PIIAligner struct {
	enabledClasses  map[config.AudioPIIClass]bool
	phoneRegex      *regexp.Regexp
	ssnRegex        *regexp.Regexp
	emailRegex      *regexp.Regexp
	creditCardRegex *regexp.Regexp
	addressRegex    *regexp.Regexp
	nameRegex       *regexp.Regexp
}

// NewPIIAligner constructs an aligner configured with target audio classes.
func NewPIIAligner(enabled []config.AudioPIIClass) *PIIAligner {
	em := make(map[config.AudioPIIClass]bool)
	for _, c := range enabled {
		em[c] = true
	}

	return &PIIAligner{
		enabledClasses: em,
		// Standard statutory PII regexes
		phoneRegex: regexp.MustCompile(`(?i)\b(?:\+?1[-.\s]?)?(?:\([0-9]{3}\)|[0-9]{3})[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b|\b[0-9]{3}[-.\s][0-9]{4}\b`),
		ssnRegex:   regexp.MustCompile(`(?i)\b(?:\d{3}[- ]\d{2}[- ]\d{4}|\d{13})\b`), // US SSN (9 digits) and Romanian CNP (13 digits)
		emailRegex: regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`),
		creditCardRegex: regexp.MustCompile(`(?i)\b(?:\d{4}[- ]){3}\d{4}\b`),
		addressRegex: regexp.MustCompile(`(?i)\b\d{1,5}\s+[A-Za-z0-9.\s]+(?:Street|St|Avenue|Ave|Road|Rd|Boulevard|Blvd|Lane|Ln|Drive|Dr|Way|Strada|Str|Bulevardul|B-dul)\b`),
		nameRegex: regexp.MustCompile(`(?i)\b(?:Officer|Deputy|Trooper|Doctor|Dr\.|Mr\.|Mrs\.|Ms\.|Patient|Suspect)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)?)\b`),
	}
}

// FindPIIEntities scans transcript words and text, returning aligned audio entities.
func (pa *PIIAligner) FindPIIEntities(words []TranscriptWord, fullText string) []domain.AudioEntity {
	var entities []domain.AudioEntity

	// 1. Social Security Number / National ID
	if pa.enabledClasses[config.AudioPIISSN] {
		for _, match := range pa.ssnRegex.FindAllStringIndex(fullText, -1) {
			matchedStr := fullText[match[0]:match[1]]
			if start, end := mapTextRangeToWordTimestamps(words, fullText, match[0], match[1]); end > start {
				entities = append(entities, domain.AudioEntity{
					Class:      config.AudioPIISSN,
					Text:       matchedStr,
					StartMs:    start,
					EndMs:      end,
					Confidence: 0.98,
				})
			}
		}
	}

	// 2. Phone Numbers
	if pa.enabledClasses[config.AudioPIIPhone] {
		for _, match := range pa.phoneRegex.FindAllStringIndex(fullText, -1) {
			matchedStr := strings.TrimSpace(fullText[match[0]:match[1]])
			if len(matchedStr) >= 7 {
				if start, end := mapTextRangeToWordTimestamps(words, fullText, match[0], match[1]); end > start {
					entities = append(entities, domain.AudioEntity{
						Class:      config.AudioPIIPhone,
						Text:       matchedStr,
						StartMs:    start,
						EndMs:      end,
						Confidence: 0.95,
					})
				}
			}
		}
	}

	// 3. Email Addresses
	if pa.enabledClasses[config.AudioPIIEmail] {
		for _, match := range pa.emailRegex.FindAllStringIndex(fullText, -1) {
			matchedStr := fullText[match[0]:match[1]]
			if start, end := mapTextRangeToWordTimestamps(words, fullText, match[0], match[1]); end > start {
				entities = append(entities, domain.AudioEntity{
					Class:      config.AudioPIIEmail,
					Text:       matchedStr,
					StartMs:    start,
					EndMs:      end,
					Confidence: 0.99,
				})
			}
		}
	}

	// 4. Credit Cards
	if pa.enabledClasses[config.AudioPIICreditCard] {
		for _, match := range pa.creditCardRegex.FindAllStringIndex(fullText, -1) {
			matchedStr := fullText[match[0]:match[1]]
			if start, end := mapTextRangeToWordTimestamps(words, fullText, match[0], match[1]); end > start {
				entities = append(entities, domain.AudioEntity{
					Class:      config.AudioPIICreditCard,
					Text:       matchedStr,
					StartMs:    start,
					EndMs:      end,
					Confidence: 0.97,
				})
			}
		}
	}

	// 5. Physical Addresses
	if pa.enabledClasses[config.AudioPIIAddress] {
		for _, match := range pa.addressRegex.FindAllStringIndex(fullText, -1) {
			matchedStr := fullText[match[0]:match[1]]
			if start, end := mapTextRangeToWordTimestamps(words, fullText, match[0], match[1]); end > start {
				entities = append(entities, domain.AudioEntity{
					Class:      config.AudioPIIAddress,
					Text:       matchedStr,
					StartMs:    start,
					EndMs:      end,
					Confidence: 0.92,
				})
			}
		}
	}

	// 6. Person Names identified after honorifics/roles
	if pa.enabledClasses[config.AudioPIIName] {
		for _, match := range pa.nameRegex.FindAllStringSubmatchIndex(fullText, -1) {
			if len(match) >= 4 && match[2] != -1 && match[3] != -1 {
				nameStr := fullText[match[2]:match[3]]
				if start, end := mapTextRangeToWordTimestamps(words, fullText, match[2], match[3]); end > start {
					entities = append(entities, domain.AudioEntity{
						Class:      config.AudioPIIName,
						Text:       nameStr,
						StartMs:    start,
						EndMs:      end,
						Confidence: 0.90,
					})
				}
			}
		}
	}

	return deduplicateEntities(entities)
}

// MergeIntervalsForBleeper creates non-overlapping bleep time spans for audio synthesis.
func MergeIntervalsForBleeper(entities []domain.AudioEntity) []domain.AudioEntity {
	if len(entities) <= 1 {
		return entities
	}

	sorted := make([]domain.AudioEntity, len(entities))
	copy(sorted, entities)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].StartMs < sorted[i].StartMs {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	merged := make([]domain.AudioEntity, 0, len(sorted))
	current := sorted[0]

	for i := 1; i < len(sorted); i++ {
		next := sorted[i]
		if next.StartMs <= current.EndMs {
			if next.EndMs > current.EndMs {
				current.EndMs = next.EndMs
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}
	merged = append(merged, current)

	return merged
}

func deduplicateEntities(entities []domain.AudioEntity) []domain.AudioEntity {
	seen := make(map[string]bool)
	var result []domain.AudioEntity
	for _, e := range entities {
		key := string(e.Class) + ":" + e.Text
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

// mapTextRangeToWordTimestamps correlates character indices in fullText to word start/end milliseconds.
func mapTextRangeToWordTimestamps(words []TranscriptWord, fullText string, charStart, charEnd int) (int64, int64) {
	if len(words) == 0 {
		return 0, 0
	}

	var startMs int64 = -1
	var endMs int64 = -1

	currentOffset := 0
	for _, w := range words {
		wIndex := strings.Index(fullText[currentOffset:], w.Word)
		if wIndex == -1 {
			continue
		}
		wordStartChar := currentOffset + wIndex
		wordEndChar := wordStartChar + len(w.Word)
		currentOffset = wordEndChar

		// Overlap check with [charStart, charEnd]
		if wordEndChar >= charStart && wordStartChar <= charEnd {
			if startMs == -1 || w.StartMs < startMs {
				startMs = w.StartMs
			}
			if w.EndMs > endMs {
				endMs = w.EndMs
			}
		}
	}

	if startMs == -1 || endMs == -1 {
		return 0, 0
	}

	// Add 30ms acoustic safety margin
	startMs = max64(0, startMs-30)
	endMs += 30

	return startMs, endMs
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
