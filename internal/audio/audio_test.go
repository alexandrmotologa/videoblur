package audio

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

func TestPIIAlignerExtraction(t *testing.T) {
	aligner := NewPIIAligner([]config.AudioPIIClass{
		config.AudioPIIName,
		config.AudioPIIPhone,
		config.AudioPIISSN,
		config.AudioPIIAddress,
		config.AudioPIICreditCard,
	})

	fullText := "Patient John Doe with SSN 123-45-6789 and phone 555-0199 living at 742 Evergreen Terrace with card 4111 2222 3333 4444."
	words := []TranscriptWord{
		{Word: "Patient", StartMs: 0, EndMs: 150, Conf: 0.99},
		{Word: "John", StartMs: 160, EndMs: 300, Conf: 0.99},
		{Word: "Doe", StartMs: 310, EndMs: 450, Conf: 0.99},
		{Word: "with", StartMs: 460, EndMs: 520, Conf: 0.99},
		{Word: "SSN", StartMs: 530, EndMs: 650, Conf: 0.99},
		{Word: "123-45-6789", StartMs: 660, EndMs: 1200, Conf: 0.99},
		{Word: "and", StartMs: 1220, EndMs: 1350, Conf: 0.99},
		{Word: "phone", StartMs: 1370, EndMs: 1550, Conf: 0.99},
		{Word: "555-0199", StartMs: 1570, EndMs: 2200, Conf: 0.99},
		{Word: "living", StartMs: 2220, EndMs: 2400, Conf: 0.99},
		{Word: "at", StartMs: 2420, EndMs: 2500, Conf: 0.99},
		{Word: "742", StartMs: 2520, EndMs: 2700, Conf: 0.99},
		{Word: "Evergreen", StartMs: 2720, EndMs: 3100, Conf: 0.99},
		{Word: "Terrace", StartMs: 3120, EndMs: 3400, Conf: 0.99},
		{Word: "with", StartMs: 3420, EndMs: 3550, Conf: 0.99},
		{Word: "card", StartMs: 3570, EndMs: 3700, Conf: 0.99},
		{Word: "4111", StartMs: 3720, EndMs: 4000, Conf: 0.99},
		{Word: "2222", StartMs: 4020, EndMs: 4300, Conf: 0.99},
		{Word: "3333", StartMs: 4320, EndMs: 4600, Conf: 0.99},
		{Word: "4444.", StartMs: 4620, EndMs: 5000, Conf: 0.99},
	}

	entities := aligner.FindPIIEntities(words, fullText)
	if len(entities) == 0 {
		t.Fatalf("Expected detected PII entities, got 0")
	}

	foundSSN := false
	foundPhone := false
	foundCard := false
	foundName := false
	foundAddress := false

	for _, ent := range entities {
		if ent.Class == config.AudioPIISSN {
			foundSSN = true
		}
		if ent.Class == config.AudioPIIPhone {
			foundPhone = true
		}
		if ent.Class == config.AudioPIICreditCard {
			foundCard = true
		}
		if ent.Class == config.AudioPIIName {
			foundName = true
		}
		if ent.Class == config.AudioPIIAddress {
			foundAddress = true
		}
	}

	if !foundSSN {
		t.Errorf("Expected SSN detection")
	}
	if !foundPhone {
		t.Errorf("Expected Phone detection")
	}
	if !foundCard {
		t.Errorf("Expected Credit Card detection")
	}
	if !foundName {
		t.Errorf("Expected Name detection")
	}
	if !foundAddress {
		t.Errorf("Expected Address detection")
	}
}

func TestAudioBleeperPCMModulation(t *testing.T) {
	sampleRate := 16000
	channels := 1
	durationSec := 1.0
	numSamples := int(float64(sampleRate) * durationSec)

	// Create 1 second of silence
	pcm := make([]byte, numSamples*2)

	bleeper := NewAudioBleeper(1000.0, 0.8, false, 20)
	entities := []domain.AudioEntity{
		{
			Class:   config.AudioPIISSN,
			StartMs: 200,
			EndMs:   600,
		},
	}

	bleeper.ProcessPCM16(pcm, sampleRate, channels, entities)

	// Sample at 400.25 ms (quarter wave of 1kHz tone = peak of sine)
	// 400.25ms * 16 samples/ms = 6404
	peakSample := int(float64(sampleRate) * 0.40025)
	val := int16(binary.LittleEndian.Uint16(pcm[peakSample*2 : peakSample*2+2]))
	if math.Abs(float64(val)) < 5000 {
		t.Errorf("Expected peak bleep tone at quarter-cycle, got %d", val)
	}

	// Verify sample before bleep remains untouched (0)
	beforeSample := int(float64(sampleRate) * 0.050)
	valBefore := int16(binary.LittleEndian.Uint16(pcm[beforeSample*2 : beforeSample*2+2]))
	if valBefore != 0 {
		t.Errorf("Sample before bleep was altered: %d", valBefore)
	}
}
