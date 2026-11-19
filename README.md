# videoblur

Autonomous multimodal video and audio redaction engine for legal discovery, law enforcement records, and healthcare compliance.

videoblur eliminates manual keyframing and audio bleeping in legal video processing. It runs entirely offline without transmitting media to cloud services, meeting CJIS, HIPAA, and FOIA statutory requirements.

## Key capabilities

- **Zero-cloud execution**: Operates 100% locally on CPU or GPU via Go and FFmpeg. No data leaves the local machine.
- **Visual multi-class detection**: Detects human faces, vehicle license plates, computer monitors, and ID badges.
- **Kalman temporal tracking**: 7-state Kalman filter bridges detection dropouts across up to 8 consecutive frames, preventing unblurred flicker during rapid motion.
- **Phoneme-aligned audio bleeping**: Transcribes spoken audio locally via Whisper, identifies sensitive personal entities, and injects 1 kHz sine waves with 20ms cosine cross-fade envelopes.
- **Cryptographic compliance manifest**: Generates an Ed25519-signed JSON manifest with cumulative SHA-256 frame hashes for court chain-of-custody verification.
- **Embedded review studio**: Includes a local React 19 web console for timeline scrubbing, bounding box inspection, and one-click export.

## Architecture

```
Raw Media Input -> Demuxer -> Visual Detector + Kalman Tracker -> Blur / Mosaic / Blackout
                           \-> Whisper ASR + PII Extractor     -> 1kHz Sine Wave Crossfade
                                                                         |
Muxer Encoder <- Synchronized Video & Audio Stream <---------------------+
      |
      v
redacted.mp4 + redaction_audit.json (Ed25519 Signed)
```

## Quick start

### Prerequisites

- Go 1.22 or newer
- FFmpeg 6.0 or newer with ffprobe available in PATH
- Node.js 20+ (for building the web studio)

### Installation

Clone the repository and build the binary:

```bash
git clone https://github.com/alexandrmotologa/videoblur.git
cd videoblur
make build
```

The compiled binary will be placed at `bin/videoblur`.

### Basic usage

#### 1. Redact video and audio

Execute the full redaction pipeline on an input video:

```bash
videoblur redact \
  -i input_footage.mp4 \
  -o redacted.mp4 \
  -a redaction_audit.json \
  --mode gaussian \
  --standard CJIS
```

Available modes:
- `gaussian`: Gaussian blur (configurable `--sigma`, default: 18.0)
- `mosaic`: Pixelation blocks (configurable `--block-size`, default: 24)
- `blackout`: Solid blackout rectangles for courtroom evidence

#### 2. Scan footage without modifying

Perform a dry-run detection scan to identify all visual and audio entities with timestamps:

```bash
videoblur scan -i input_footage.mp4
```

#### 3. Verify cryptographic compliance certificate

Verify an existing audit manifest against its digital signature:

```bash
videoblur certify -m redaction_audit.json
```

Output:

```json
{
  "is_valid": true,
  "signature_valid": true,
  "compliance_standard": "CJIS",
  "auditor_identity": "videoblur-autonomous-agent",
  "total_frames": 150,
  "total_redactions": 5671
}
```

#### 4. Launch interactive review studio

Start the embedded local review server:

```bash
videoblur studio -p 8181 -v redacted.mp4 -a redaction_audit.json
```

Navigate to `http://127.0.0.1:8181` to inspect frames, scrub audio waveforms, and download certified exports.

## Configuration file

Create a `videoblur.yaml` file to customize redaction rules:

```yaml
visual:
  enabled_classes:
    - face
    - license_plate
    - computer_screen
    - id_badge
  default_mode: gaussian
  blur_sigma: 18.0
  mosaic_block_size: 24
  confidence_threshold: 0.40
  tracking_max_missed: 8
  tracking_iou_min: 0.25
  padding_ratio: 0.12

audio:
  enabled_classes:
    - person_name
    - phone_number
    - ssn_cnp
    - email_address
    - credit_card
    - physical_address
  bleep_frequency_hz: 1000.0
  bleep_volume: 0.75
  mute_only: false
  crossfade_ms: 20

compliance:
  standard: CJIS
  auditor_identity: officer-compliance-unit
  sign_certificate: true

output:
  video_codec: libx264
  audio_codec: aac
  crf: 18
  preset: fast
```

## Running tests

Run the test suite across all subsystems:

```bash
go test -v ./...
```

To run end-to-end processing benchmarks against sample media:

```bash
make run-redact
make run-certify
```

## Documentation

- [Architecture specification](docs/architecture.md): Engineering details, Kalman filter state equations, and audio modulation mathematics.
- [Legal compliance playbook](docs/compliance.md): Regulatory compliance details for FOIA, CJIS, and HIPAA Safe Harbor.

## License

Apache License 2.0. See LICENSE for details.
