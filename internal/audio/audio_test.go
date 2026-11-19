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

func TestAudioBleeperAcousticModes(t *testing.T) {
	sampleRate := 16000
	channels := 1
	numSamples := sampleRate // 1 second
	entities := []domain.AudioEntity{
		{
			Class:   config.AudioPIISSN,
			StartMs: 200,
			EndMs:   600,
		},
	}

	// 1. Tone mode test
	pcmTone := make([]byte, numSamples*2)
	bleeperTone := NewAudioBleeper(config.AudioModeTone, config.ChannelAll, 1000.0, 0.8, 20)
	bleeperTone.ProcessPCM16(pcmTone, sampleRate, channels, entities)

	peakSample := int(float64(sampleRate) * 0.40025)
	valTone := int16(binary.LittleEndian.Uint16(pcmTone[peakSample*2 : peakSample*2+2]))
	if math.Abs(float64(valTone)) < 5000 {
		t.Errorf("Expected peak bleep tone, got %d", valTone)
	}

	// 2. Mute mode test
	pcmMute := make([]byte, numSamples*2)
	for i := range pcmMute {
		pcmMute[i] = 120 // fill with audio noise
	}
	bleeperMute := NewAudioBleeper(config.AudioModeMute, config.ChannelAll, 1000.0, 0.8, 20)
	bleeperMute.ProcessPCM16(pcmMute, sampleRate, channels, entities)

	valMute := int16(binary.LittleEndian.Uint16(pcmMute[peakSample*2 : peakSample*2+2]))
	if valMute != 0 {
		t.Errorf("Expected 0 in mute mode, got %d", valMute)
	}

	// 3. Pink Noise mode test
	pcmPink := make([]byte, numSamples*2)
	bleeperPink := NewAudioBleeper(config.AudioModePinkNoise, config.ChannelAll, 1000.0, 0.8, 20)
	bleeperPink.ProcessPCM16(pcmPink, sampleRate, channels, entities)

	valPink := int16(binary.LittleEndian.Uint16(pcmPink[peakSample*2 : peakSample*2+2]))
	if valPink == 0 {
		t.Errorf("Expected stochastic non-zero pink noise, got 0")
	}

	// 4. Stereo channel isolation test
	stereoSamples := sampleRate * 2 * 2 // 1 sec, 2 channels, 2 bytes/sample
	pcmStereo := make([]byte, stereoSamples)
	bleeperLeftOnly := NewAudioBleeper(config.AudioModeTone, config.ChannelLeft, 1000.0, 0.8, 20)
	bleeperLeftOnly.ProcessPCM16(pcmStereo, sampleRate, 2, entities)

	// Left channel (ch 0) should have tone
	leftSampleOffset := peakSample*4 + 0
	valLeft := int16(binary.LittleEndian.Uint16(pcmStereo[leftSampleOffset : leftSampleOffset+2]))
	if math.Abs(float64(valLeft)) < 5000 {
		t.Errorf("Expected tone on Left channel, got %d", valLeft)
	}

	// Right channel (ch 1) should remain untouched (0)
	rightSampleOffset := peakSample*4 + 2
	valRight := int16(binary.LittleEndian.Uint16(pcmStereo[rightSampleOffset : rightSampleOffset+2]))
	if valRight != 0 {
		t.Errorf("Expected 0 on Right channel when targeting Left only, got %d", valRight)
	}
}
