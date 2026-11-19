package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultConfig returns safe production defaults for video and audio redaction.
func DefaultConfig() *Config {
	return &Config{
		Visual: VisualConfig{
			EnabledClasses: []TargetClass{
				TargetFace,
				TargetLicensePlate,
				TargetComputerScreen,
				TargetIDBadge,
			},
			DefaultMode:         ModeGaussianBlur,
			BlurSigma:           18.0,
			MosaicBlockSize:     24,
			ConfidenceThreshold: 0.40,
			TrackingMaxMissed:   8,
			TrackingIoUMin:      0.25,
			PaddingRatio:        0.12, // 12% bbox expansion to avoid edge disclosure
		},
		Audio: AudioConfig{
			EnabledClasses: []AudioPIIClass{
				AudioPIIName,
				AudioPIIPhone,
				AudioPIISSN,
				AudioPIIEmail,
				AudioPIICreditCard,
				AudioPIIAddress,
			},
			WhisperModelPath: "",
			BleepFrequencyHz: 1000.0,
			BleepVolume:      0.75,
			MuteOnly:         false,
			CrossfadeMs:      20,
		},
		Compliance: ComplianceConfig{
			Standard:        "CJIS",
			AuditorIdentity: "videoblur-autonomous-agent",
			SignCertificate: true,
			PrivateKeyPath:  "",
			PublicKeyPath:   "",
		},
		Output: OutputConfig{
			VideoCodec:    "libx264",
			AudioCodec:    "aac",
			CRF:           18,
			Preset:        "fast",
			HardwareAccel: "auto",
		},
	}
}

// LoadFromFile reads and parses a YAML configuration file.
func LoadFromFile(filePath string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", filePath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks configuration sanity.
func (c *Config) Validate() error {
	if c.Visual.BlurSigma <= 0 {
		c.Visual.BlurSigma = 15.0
	}
	if c.Visual.MosaicBlockSize <= 0 {
		c.Visual.MosaicBlockSize = 20
	}
	if c.Visual.ConfidenceThreshold <= 0 || c.Visual.ConfidenceThreshold > 1.0 {
		c.Visual.ConfidenceThreshold = 0.40
	}
	if c.Visual.TrackingMaxMissed < 0 {
		c.Visual.TrackingMaxMissed = 6
	}
	if c.Audio.BleepFrequencyHz <= 0 {
		c.Audio.BleepFrequencyHz = 1000.0
	}
	if c.Audio.CrossfadeMs <= 0 {
		c.Audio.CrossfadeMs = 20
	}
	return nil
}
