package config

// RedactionMode specifies the visual masking technique.
type RedactionMode string

const (
	ModeGaussianBlur RedactionMode = "gaussian"
	ModeMosaic       RedactionMode = "mosaic"
	ModeBlackout     RedactionMode = "blackout"
)

// TargetClass represents an identifiable visual object to redact.
type TargetClass string

const (
	TargetFace           TargetClass = "face"
	TargetLicensePlate   TargetClass = "license_plate"
	TargetComputerScreen TargetClass = "computer_screen"
	TargetIDBadge        TargetClass = "id_badge"
)

// AudioPIIClass identifies categories of spoken sensitive data.
type AudioPIIClass string

const (
	AudioPIIName       AudioPIIClass = "person_name"
	AudioPIIPhone      AudioPIIClass = "phone_number"
	AudioPIISSN        AudioPIIClass = "ssn_cnp"
	AudioPIIEmail      AudioPIIClass = "email_address"
	AudioPIICreditCard AudioPIIClass = "credit_card"
	AudioPIIAddress    AudioPIIClass = "physical_address"
)

// AudioCensorMode defines the acoustic modulation technique over spoken PII.
type AudioCensorMode string

const (
	AudioModeTone      AudioCensorMode = "tone"       // 1 kHz pure sine tone
	AudioModeMute      AudioCensorMode = "mute"       // Attenuated silence
	AudioModePinkNoise AudioCensorMode = "pink_noise" // Gentle ambient acoustic noise
	AudioModeGarble    AudioCensorMode = "garble"     // Time-reversed audio preserving ambient texture
)

// AudioChannelTarget specifies which stereo channels to redact.
type AudioChannelTarget string

const (
	ChannelAll   AudioChannelTarget = "all"
	ChannelLeft  AudioChannelTarget = "left"
	ChannelRight AudioChannelTarget = "right"
)

// VisualConfig holds parameters for image frame processing.
type VisualConfig struct {
	EnabledClasses      []TargetClass `yaml:"enabled_classes" json:"enabled_classes"`
	DefaultMode         RedactionMode `yaml:"default_mode" json:"default_mode"`
	BlurSigma           float64       `yaml:"blur_sigma" json:"blur_sigma"`
	MosaicBlockSize     int           `yaml:"mosaic_block_size" json:"mosaic_block_size"`
	ConfidenceThreshold float64       `yaml:"confidence_threshold" json:"confidence_threshold"`
	TrackingMaxMissed   int           `yaml:"tracking_max_missed" json:"tracking_max_missed"`
	TrackingIoUMin      float64       `yaml:"tracking_iou_min" json:"tracking_iou_min"`
	PaddingRatio        float64       `yaml:"padding_ratio" json:"padding_ratio"`
}

// AudioConfig holds settings for spoken entity masking.
type AudioConfig struct {
	EnabledClasses   []AudioPIIClass    `yaml:"enabled_classes" json:"enabled_classes"`
	CensorMode       AudioCensorMode    `yaml:"censor_mode" json:"censor_mode"`
	TargetChannel    AudioChannelTarget `yaml:"target_channel" json:"target_channel"`
	WhisperModelPath string             `yaml:"whisper_model_path" json:"whisper_model_path"`
	BleepFrequencyHz float64            `yaml:"bleep_frequency_hz" json:"bleep_frequency_hz"`
	BleepVolume      float64            `yaml:"bleep_volume" json:"bleep_volume"`
	CrossfadeMs      int                `yaml:"crossfade_ms" json:"crossfade_ms"`
}

// ComplianceConfig specifies regulatory presets and digital signing rules.
type ComplianceConfig struct {
	Standard         string `yaml:"standard" json:"standard"` // "CJIS", "HIPAA", "FOIA", "GDPR"
	AuditorIdentity  string `yaml:"auditor_identity" json:"auditor_identity"`
	SignCertificate  bool   `yaml:"sign_certificate" json:"sign_certificate"`
	PrivateKeyPath   string `yaml:"private_key_path" json:"private_key_path"`
	PublicKeyPath    string `yaml:"public_key_path" json:"public_key_path"`
}

// OutputConfig governs video encoding parameters.
type OutputConfig struct {
	VideoCodec    string `yaml:"video_codec" json:"video_codec"`     // e.g. "libx264", "h264_nvenc"
	AudioCodec    string `yaml:"audio_codec" json:"audio_codec"`     // e.g. "aac"
	CRF           int    `yaml:"crf" json:"crf"`                     // Constant Rate Factor
	Preset        string `yaml:"preset" json:"preset"`               // "fast", "medium", "slow"
	HardwareAccel string `yaml:"hardware_accel" json:"hardware_accel"` // "auto", "cuda", "qsv", "none"
}

// Config represents the complete runtime configuration.
type Config struct {
	Visual     VisualConfig     `yaml:"visual" json:"visual"`
	Audio      AudioConfig      `yaml:"audio" json:"audio"`
	Compliance ComplianceConfig `yaml:"compliance" json:"compliance"`
	Output     OutputConfig     `yaml:"output" json:"output"`
}
