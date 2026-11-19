package audio

import (
	"encoding/binary"
	"math"
	"math/rand"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// AudioBleeper applies acoustic censorship over sensitive intervals.
type AudioBleeper struct {
	mode          config.AudioCensorMode
	targetChannel config.AudioChannelTarget
	frequencyHz   float64
	volume        float64
	crossfadeMs   int
	pinkRows      [7]float64
	pinkRunning   float64
	pinkRand      *rand.Rand
}

// NewAudioBleeper initializes the audio bleeper with advanced acoustic modes.
func NewAudioBleeper(mode config.AudioCensorMode, targetChan config.AudioChannelTarget, freqHz, volume float64, crossfadeMs int) *AudioBleeper {
	if mode == "" {
		mode = config.AudioModeTone
	}
	if targetChan == "" {
		targetChan = config.ChannelAll
	}
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
		mode:          mode,
		targetChannel: targetChan,
		frequencyHz:   freqHz,
		volume:        volume,
		crossfadeMs:   crossfadeMs,
		pinkRand:      rand.New(rand.NewSource(42)),
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

		fadeStart := maxInt(0, startSample-crossfadeSamples)
		fadeEnd := minInt(totalSamples, endSample+crossfadeSamples)

		// Pre-compute garble buffer if garble mode is selected
		var garbleBuffer []int16
		if ab.mode == config.AudioModeGarble {
			garbleBuffer = make([]int16, (endSample-startSample)*channels)
			for s := startSample; s < endSample; s++ {
				// Reverse time within interval
				revSample := endSample - 1 - (s - startSample)
				for ch := 0; ch < channels; ch++ {
					offset := revSample*bytesPerSample + ch*2
					origVal := int16(binary.LittleEndian.Uint16(pcmBytes[offset : offset+2]))
					garbleBuffer[(s-startSample)*channels+ch] = origVal
				}
			}
		}

		for s := fadeStart; s < fadeEnd; s++ {
			t := float64(s) / float64(sampleRate)

			// Generate replacement acoustic signal based on configured mode
			var synthSample float64
			switch ab.mode {
			case config.AudioModeMute:
				synthSample = 0.0
			case config.AudioModePinkNoise:
				synthSample = ab.generatePinkNoise() * ab.volume * 32767.0
			case config.AudioModeGarble:
				// Garble sample extracted per channel below
			case config.AudioModeTone:
				fallthrough
			default:
				synthSample = math.Sin(2.0*math.Pi*ab.frequencyHz*t) * ab.volume * 32767.0
			}

			// Cosine cross-fade blend weight w in [0.0, 1.0]
			var w float64
			if s < startSample {
				progress := float64(s-fadeStart) / float64(startSample-fadeStart)
				w = 0.5 * (1.0 - math.Cos(math.Pi*progress))
			} else if s >= endSample {
				progress := float64(s-endSample) / float64(fadeEnd-endSample)
				w = 0.5 * (1.0 + math.Cos(math.Pi*progress))
			} else {
				w = 1.0
			}

			offset := s * bytesPerSample
			for ch := 0; ch < channels; ch++ {
				// Check channel targeting (all, left=0, right=1)
				if channels == 2 {
					if ab.targetChannel == config.ChannelLeft && ch != 0 {
						continue
					}
					if ab.targetChannel == config.ChannelRight && ch != 1 {
						continue
					}
				}

				chOffset := offset + ch*2
				if chOffset+2 > len(pcmBytes) {
					break
				}

				origVal := int16(binary.LittleEndian.Uint16(pcmBytes[chOffset : chOffset+2]))

				targetAcoustic := synthSample
				if ab.mode == config.AudioModeGarble && s >= startSample && s < endSample {
					targetAcoustic = float64(garbleBuffer[(s-startSample)*channels+ch])
				}

				blended := (1.0-w)*float64(origVal) + w*targetAcoustic
				clamped := int16(math.Max(-32768, math.Min(32767, blended)))
				binary.LittleEndian.PutUint16(pcmBytes[chOffset:chOffset+2], uint16(clamped))
			}
		}
	}
}

// generatePinkNoise implements the Voss-McCartney 1/f noise generation algorithm.
func (ab *AudioBleeper) generatePinkNoise() float64 {
	index := ab.pinkRand.Intn(128)
	numZeros := countTrailingZeros(index)
	if numZeros >= 7 {
		numZeros = 6
	}

	ab.pinkRunning -= ab.pinkRows[numZeros]
	newVal := (ab.pinkRand.Float64()*2.0 - 1.0) / 7.0
	ab.pinkRunning += newVal
	ab.pinkRows[numZeros] = newVal

	return math.Max(-1.0, math.Min(1.0, ab.pinkRunning))
}

func countTrailingZeros(n int) int {
	if n == 0 {
		return 7
	}
	zeros := 0
	for (n & 1) == 0 {
		zeros++
		n >>= 1
	}
	return zeros
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
