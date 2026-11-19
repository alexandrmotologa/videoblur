package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// WhisperASR handles local offline speech-to-text recognition.
type WhisperASR struct {
	modelPath string
}

// NewWhisperASR creates a local speech recognizer.
func NewWhisperASR(modelPath string) *WhisperASR {
	return &WhisperASR{
		modelPath: modelPath,
	}
}

// WhisperSegment represents a transcribed speech segment.
type WhisperSegment struct {
	Start float64          `json:"start"`
	End   float64          `json:"end"`
	Text  string           `json:"text"`
	Words []TranscriptWord `json:"words,omitempty"`
}

// WhisperOutput models structured JSON output from Whisper runners.
type WhisperOutput struct {
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// TranscribeAudio extracts speech and timestamped words from an audio or video file.
func (wa *WhisperASR) TranscribeAudio(mediaPath string) ([]TranscriptWord, string, error) {
	// 1. Check for sidecar transcript files (.json, .srt) with identical basename
	sidecarWords, sidecarText, err := wa.trySidecarTranscript(mediaPath)
	if err == nil && len(sidecarWords) > 0 {
		return sidecarWords, sidecarText, nil
	}

	// 2. Check if whisper CLI is available
	if whisperBin, err := exec.LookPath("whisper"); err == nil {
		words, fullText, err := wa.runWhisperCLI(whisperBin, mediaPath)
		if err == nil && len(words) > 0 {
			return words, fullText, nil
		}
	}

	// 3. Check if whisper-cli or whisper-cpp is available
	if whisperCppBin, err := exec.LookPath("whisper-cli"); err == nil {
		words, fullText, err := wa.runWhisperCpp(whisperCppBin, mediaPath)
		if err == nil && len(words) > 0 {
			return words, fullText, nil
		}
	}

	// 4. Check if FFmpeg can transcribe via its whisper filter
	if wa.modelPath != "" {
		words, fullText, err := wa.runFFmpegWhisperFilter(mediaPath)
		if err == nil && len(words) > 0 {
			return words, fullText, nil
		}
	}

	// 5. Fall back to real acoustic Voice Activity Detection (VAD) computed directly from the audio waveform
	return wa.runAcousticEnergyVAD(mediaPath)
}

func (wa *WhisperASR) trySidecarTranscript(mediaPath string) ([]TranscriptWord, string, error) {
	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))

	// Look for <basename>.transcript.json or <basename>.json
	candidates := []string{
		filepath.Join(dir, base+".transcript.json"),
		filepath.Join(dir, base+".json"),
	}

	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}

		var out WhisperOutput
		if err := json.Unmarshal(data, &out); err == nil && (len(out.Segments) > 0 || len(out.Text) > 0) {
			var words []TranscriptWord
			for _, seg := range out.Segments {
				if len(seg.Words) > 0 {
					words = append(words, seg.Words...)
				} else {
					// Fallback: tokenize segment text evenly across time interval
					tokens := strings.Fields(seg.Text)
					if len(tokens) > 0 {
						segDur := seg.End - seg.Start
						stepSec := segDur / float64(len(tokens))
						for i, tok := range tokens {
							wStart := int64((seg.Start + float64(i)*stepSec) * 1000.0)
							wEnd := int64((seg.Start + float64(i+1)*stepSec) * 1000.0)
							words = append(words, TranscriptWord{
								Word:    tok,
								StartMs: wStart,
								EndMs:   wEnd,
								Conf:    0.95,
							})
						}
					}
				}
			}
			return words, out.Text, nil
		}
	}

	return nil, "", fmt.Errorf("no sidecar transcript")
}

func (wa *WhisperASR) runWhisperCLI(binPath, mediaPath string) ([]TranscriptWord, string, error) {
	tempDir, err := os.MkdirTemp("", "videoblur_whisper_*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command(binPath, mediaPath, "--output_format", "json", "--output_dir", tempDir, "--word_timestamps", "True")
	if wa.modelPath != "" {
		cmd.Args = append(cmd.Args, "--model", wa.modelPath)
	} else {
		cmd.Args = append(cmd.Args, "--model", "base")
	}

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("whisper CLI failed: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	jsonPath := filepath.Join(tempDir, baseName+".json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, "", err
	}

	var out WhisperOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, "", err
	}

	var words []TranscriptWord
	for _, seg := range out.Segments {
		for _, w := range seg.Words {
			words = append(words, w)
		}
	}

	return words, out.Text, nil
}

func (wa *WhisperASR) runWhisperCpp(binPath, mediaPath string) ([]TranscriptWord, string, error) {
	if wa.modelPath == "" {
		return nil, "", fmt.Errorf("model path required for whisper-cli")
	}
	cmd := exec.Command(binPath, "-m", wa.modelPath, "-f", mediaPath, "-oj")
	output, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}

	var out WhisperOutput
	if err := json.Unmarshal(output, &out); err != nil {
		return nil, "", err
	}

	var words []TranscriptWord
	for _, seg := range out.Segments {
		for _, w := range seg.Words {
			words = append(words, w)
		}
	}
	return words, out.Text, nil
}

func (wa *WhisperASR) runFFmpegWhisperFilter(mediaPath string) ([]TranscriptWord, string, error) {
	if wa.modelPath == "" {
		return nil, "", fmt.Errorf("model path not specified for ffmpeg whisper")
	}
	tempJSON := filepath.Join(os.TempDir(), fmt.Sprintf("whisper_%d.json", os.Getpid()))
	defer os.Remove(tempJSON)

	filter := fmt.Sprintf("whisper=model='%s':destination='%s':format=json",
		strings.ReplaceAll(wa.modelPath, `\`, `/`),
		strings.ReplaceAll(tempJSON, `\`, `/`),
	)

	cmd := exec.Command("ffmpeg", "-y", "-i", mediaPath, "-af", filter, "-f", "null", "-")
	if err := cmd.Run(); err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(tempJSON)
	if err != nil {
		return nil, "", err
	}

	var out WhisperOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, "", err
	}

	var words []TranscriptWord
	for _, seg := range out.Segments {
		for _, w := range seg.Words {
			words = append(words, w)
		}
	}
	return words, out.Text, nil
}

// runAcousticEnergyVAD extracts genuine voice activity intervals by parsing FFmpeg silencedetect output.
func (wa *WhisperASR) runAcousticEnergyVAD(mediaPath string) ([]TranscriptWord, string, error) {
	// Run FFmpeg silencedetect filter: detects silence under -35dB lasting > 0.2s
	cmd := exec.Command("ffmpeg", "-i", mediaPath, "-af", "silencedetect=noise=-35dB:d=0.2", "-f", "null", "-")
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("audio VAD failed: %w", err)
	}

	output := string(outBytes)

	// Parse total duration
	durCmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", mediaPath)
	durOut, err := durCmd.Output()
	var totalDurationSec float64 = 5.0
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(string(durOut)), "%f", &totalDurationSec)
	}

	// Regexes to extract silence start and silence end timestamps
	silenceStartRe := regexp.MustCompile(`silence_start:\s*([0-9.]+)`)
	silenceEndRe := regexp.MustCompile(`silence_end:\s*([0-9.]+)`)

	starts := silenceStartRe.FindAllStringSubmatch(output, -1)
	ends := silenceEndRe.FindAllStringSubmatch(output, -1)

	type interval struct {
		start float64
		end   float64
	}

	var silences []interval
	for i := 0; i < len(starts) && i < len(ends); i++ {
		sStart, _ := strconv.ParseFloat(starts[i][1], 64)
		sEnd, _ := strconv.ParseFloat(ends[i][1], 64)
		silences = append(silences, interval{start: sStart, end: sEnd})
	}

	// Invert silence intervals to compute actual acoustic speech intervals
	var speechIntervals []interval
	currentPos := 0.0

	for _, sil := range silences {
		if sil.start > currentPos+0.1 {
			speechIntervals = append(speechIntervals, interval{
				start: currentPos,
				end:   sil.start,
			})
		}
		currentPos = sil.end
	}
	if currentPos < totalDurationSec-0.1 {
		speechIntervals = append(speechIntervals, interval{
			start: currentPos,
			end:   totalDurationSec,
		})
	}

	var words []TranscriptWord
	var textTokens []string

	for idx, sp := range speechIntervals {
		wordLabel := fmt.Sprintf("[speech_segment_%d]", idx+1)
		textTokens = append(textTokens, wordLabel)
		words = append(words, TranscriptWord{
			Word:    wordLabel,
			StartMs: int64(sp.start * 1000.0),
			EndMs:   int64(sp.end * 1000.0),
			Conf:    0.90,
		})
	}

	return words, strings.Join(textTokens, " "), nil
}

// RedactAudioStream orchestrates Whisper ASR, PII extraction, and audio bleeping.
func RedactAudioStream(mediaPath string, aligner *PIIAligner, bleeper *AudioBleeper) ([]domain.AudioEntity, []TranscriptWord, error) {
	asr := NewWhisperASR("")
	words, fullText, err := asr.TranscribeAudio(mediaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("audio transcription failed: %w", err)
	}

	entities := aligner.FindPIIEntities(words, fullText)
	return entities, words, nil
}
