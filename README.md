# videoblur

Autonomous multimodal video and audio redaction engine for legal discovery, law enforcement records, and healthcare compliance.

videoblur eliminates manual keyframing and audio bleeping in legal video processing. It runs entirely offline without transmitting media to cloud services, meeting CJIS, HIPAA, and FOIA statutory requirements.

## Key capabilities

- **Zero-cloud execution**: Operates 100% locally on CPU or GPU via Go and FFmpeg. No data leaves the local machine.
- **Visual multi-class detection**: Detects human faces, vehicle license plates, computer monitors, and ID badges.
- **Kalman temporal tracking**: 7-state Kalman filter bridges detection dropouts across up to 8 consecutive frames, preventing unblurred flicker during rapid motion.
- **Acoustic modulation modes**: Supports 1 kHz sine tones, attenuated muting, ambient pink noise, and voice garbling, with independent stereo channel isolation (left, right, or all).
- **Decoupled high-throughput pipeline**: Asynchronous 3-stage buffer ring achieves 90+ FPS on standard computing hardware.
- **Human-in-the-loop review studio**: Local React 19 web console for timeline scrubbing, interactive drag-to-draw bounding box overrides, track exemptions, and live re-rendering.
- **Automated folder watchdog daemon**: Monitors intake directories for bodycam dock uploads and processes files through a concurrent worker queue.
- **Cryptographic compliance manifest & court PDF**: Generates Ed25519-signed JSON audit manifests and official courtroom PDF certificates with cumulative SHA-256 frame hashes.

## Architecture

```
Raw Media Input -> Demuxer -> Visual Detector + Kalman Tracker -> Blur / Mosaic / Blackout
                           \-> Whisper ASR + PII Extractor     -> Tone / Mute / Pink Noise / Garble
                                                                         |
Muxer Encoder <- Synchronized Video & Audio Stream <---------------------+
      |
      v
redacted.mp4 + redaction_audit.json (Ed25519 Signed) + redaction_certificate.pdf
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
  --pdf redaction_certificate.pdf \
  --mode gaussian \
  --standard CJIS
```

Available visual modes:
- `gaussian`: Gaussian blur (configurable `--sigma`, default: 18.0)
- `mosaic`: Pixelation blocks (configurable `--block-size`, default: 24)
- `blackout`: Solid blackout rectangles for courtroom evidence

#### 2. Run folder watchdog daemon

Monitor an intake folder (such as a bodycam docking folder) and process files automatically:

```bash
videoblur watch \
  --inbox ./incoming \
  --outbox ./processed \
  --archive ./archive \
  --concurrency 4 \
  --pdf
```

#### 3. Scan footage without modifying

Perform a dry-run detection scan to identify all visual and audio entities with timestamps:

```bash
videoblur scan -i input_footage.mp4
```

#### 4. Verify cryptographic compliance certificate

Verify an existing audit manifest against its digital signature and export a PDF:

```bash
videoblur certify -m redaction_audit.json --pdf verified_certificate.pdf
```

Output:

```json
{
  "is_valid": true,
  "signature_valid": true,
  "compliance_standard": "CJIS",
  "auditor_identity": "videoblur-autonomous-agent",
  "total_frames": 120,
  "total_redactions": 3267
}
```

#### 5. Launch interactive review studio

Start the embedded local review server:

```bash
videoblur studio -p 8181 -v redacted.mp4 -a redaction_audit.json -s source.mp4
```

Navigate to `http://127.0.0.1:8181` to inspect frames, draw custom bounding boxes, exempt whitelisted tracks, scrub audio waveforms, and download certified exports.

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
  censor_mode: tone # options: tone, mute, pink_noise, garble
  target_channel: all # options: all, left, right
  bleep_frequency_hz: 1000.0
  bleep_volume: 0.75
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
