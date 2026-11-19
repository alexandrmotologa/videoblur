package media

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MediaInfo contains container and stream metadata.
type MediaInfo struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	DurationSec float64 `json:"duration_sec"`
	FPS         float64 `json:"fps"`
	TotalFrames int     `json:"total_frames"`
	VideoCodec  string  `json:"video_codec"`
	AudioCodec  string  `json:"audio_codec"`
	SampleRate  int     `json:"sample_rate"`
	Channels    int     `json:"channels"`
	HasAudio    bool    `json:"has_audio"`
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
		Duration  string `json:"duration"`
		NbFrames  string `json:"nb_frames"`
		SampleRate string `json:"sample_rate"`
		Channels  int    `json:"channels"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ProbeMedia inspects a video or audio file using ffprobe.
func ProbeMedia(filePath string) (*MediaInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running ffprobe on %s: %w", filePath, err)
	}

	var data ffprobeOutput
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("parsing ffprobe json: %w", err)
	}

	info := &MediaInfo{
		FPS: 30.0, // default fallback
	}

	// Parse format duration
	if data.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
			info.DurationSec = dur
		}
	}

	for _, s := range data.Streams {
		if s.CodecType == "video" && info.Width == 0 {
			info.VideoCodec = s.CodecName
			info.Width = s.Width
			info.Height = s.Height

			// Parse frame rate e.g. "30/1" or "30000/1001"
			fpsStr := s.RFrameRate
			if fpsStr == "" || fpsStr == "0/0" {
				fpsStr = s.AvgFrameRate
			}
			if fpsStr != "" && strings.Contains(fpsStr, "/") {
				parts := strings.Split(fpsStr, "/")
				if len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den > 0 {
						info.FPS = num / den
					}
				}
			}

			if s.NbFrames != "" {
				if frames, err := strconv.Atoi(s.NbFrames); err == nil {
					info.TotalFrames = frames
				}
			}
			if s.Duration != "" && info.DurationSec == 0 {
				if dur, err := strconv.ParseFloat(s.Duration, 64); err == nil {
					info.DurationSec = dur
				}
			}
		} else if s.CodecType == "audio" && !info.HasAudio {
			info.HasAudio = true
			info.AudioCodec = s.CodecName
			info.Channels = s.Channels
			if s.SampleRate != "" {
				info.SampleRate, _ = strconv.Atoi(s.SampleRate)
			}
		}
	}

	if info.TotalFrames == 0 && info.DurationSec > 0 && info.FPS > 0 {
		info.TotalFrames = int(info.DurationSec * info.FPS)
	}

	return info, nil
}
