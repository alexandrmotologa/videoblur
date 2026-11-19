package audio

import (
	"encoding/binary"
	"math"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// AudioBleeper applies acoustic censorship (1 kHz sine tone or mute) over sensitive intervals.
type AudioBleeper struct {
	frequencyHz float64
	volume      float64
	muteOnly    bool
	crossfadeMs int
}

// NewAudioBleeper initializes the audio bleeper.
func NewAudioBleeper(freqHz, volume float64, muteOnly bool, crossfadeMs int) *AudioBleeper {
	if freqHz <= 0 {
		freqHz = 1000.0
	}
	if volume <= 0 || volume > 1.0 {
		volume = 0.75
	}
	if crossfadeMs <= 0 {
		crossfadeMs = 20
	}
	return &AudioBleeper{
		frequencyHz: freqHz,
		volume:      volume,
		muteOnly:    muteOnly,
		crossfadeMs: crossfadeMs,
	}
}

// ProcessPCM16 modulates an interleaved 16-bit PCM byte slice in place.
// sampleRate: e.g. 44100, 48000, 16000. channels: 1 (mono) or 2 (stereo).
func (ab *AudioBleeper) ProcessPCM16(pcmBytes []byte, sampleRate int, channels int, entities []domain.AudioEntity) {
	if len(entities) == 0 || len(pcmBytes) < 2 {
		return
	}

	bytesPerSample := 2 * channels
	totalSamples := len(pcmBytes) / bytesPerSample
	crossfadeSamples := int(float64(sampleRate*ab.crossfadeMs) / 1000.0)
	if crossfadeSamples < 1 {
		crossfadeSamples = 1
	}

	for _, ent := range entities {
		startSample := int(float64(ent.StartMs*int64(sampleRate)) / 1000.0)
		endSample := int(float64(ent.EndMs*int64(sampleRate)) / 1000.0)

		if startSample < 0 {
			startSample = 0
		}
		if endSample > totalSamples {
			endSample = totalSamples
		}
		if startSample >= endSample {
			continue
		}

		// Fade-in window start: startSample - crossfadeSamples
		fadeStart := maxInt(0, startSample-crossfadeSamples)
		// Fade-out window end: endSample + crossfadeSamples
		fadeEnd := minInt(totalSamples, endSample+crossfadeSamples)

		for s := fadeStart; s < fadeEnd; s++ {
			t := float64(s) / float64(sampleRate)

			// Generate 1 kHz sine tone
			var toneSample float64
			if !ab.muteOnly {
				toneSample = math.Sin(2.0*math.Pi*ab.frequencyHz*t) * ab.volume * 32767.0
			}

			// Determine cross-fade blend weight w in [0.0, 1.0]
			// w = 0.0 means original audio, w = 1.0 means pure bleep tone/mute
			var w float64
			if s < startSample {
				// Fade-in from original to bleep
				progress := float64(s-fadeStart) / float64(startSample-fadeStart)
				w = 0.5 * (1.0 - math.Cos(math.Pi*progress))
			} else if s >= endSample {
				// Fade-out from bleep back to original
				progress := float64(s-endSample) / float64(fadeEnd-endSample)
				w = 0.5 * (1.0 + math.Cos(math.Pi*progress))
			} else {
				// Full redaction plateau
				w = 1.0
			}

			offset := s * bytesPerSample
			for ch := 0; ch < channels; ch++ {
				chOffset := offset + ch*2
				if chOffset+2 > len(pcmBytes) {
					break
				}

				origVal := int16(binary.LittleEndian.Uint16(pcmBytes[chOffset : chOffset+2]))
				blended := (1.0-w)*float64(origVal) + w*toneSample

				// Clamp to int16 boundaries
				clamped := int16(math.Max(-32768, math.Min(32767, blended)))
				binary.LittleEndian.PutUint16(pcmBytes[chOffset:chOffset+2], uint16(clamped))
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
