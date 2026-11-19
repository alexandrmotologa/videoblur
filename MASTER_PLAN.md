# videoblur: Architecture and Engineering Master Plan
> Autonomous Multimodal Video & Audio Redaction Engine for Legal Discovery, Law Enforcement & Healthcare Compliance
> Target: Zero-Cloud Single-Binary Go/FFmpeg Engine (< 30MB RAM base) with Embedded React 19 Review Studio & Cryptographic Audit Manifest

---

## 1. Problem Statement and Motivation

### 1.1 The Massive Operational Overhead of Video Redaction in Legal & Public Sector
Government agencies, police departments (body-worn camera footage), healthcare systems (clinical recordings), public school districts, and corporate legal departments handle thousands of hours of video footage that must be released under Freedom of Information Act (FOIA) requests, court discovery subpoenas, or GDPR data subject access requests:
1. **Strict Statutory Redaction Mandates**: Federal and international laws mandate the permanent anonymization of personally identifiable information (PII) before video release: human faces, vehicle license plates, computer monitors displaying medical/financial records, identification badges, and spoken PII (names, phone numbers, Social Security numbers).
2. **The 10:1 Human Labor Chokepoint**: Manually redacting video using traditional editing suites (Adobe Premiere, DaVinci Resolve) requires frame-by-frame keyframing and manual audio cutting, consuming **5 to 10 hours of human labor for every single 1 hour of recorded video**.
3. **Catastrophic "Flicker" Privacy Breaches**: In manual or naive automated redaction, object detectors occasionally miss a face or license plate for 2 or 3 intermediate frames during rapid motion or camera turns. An unblurred face visible for a fraction of a second destroys compliance and triggers severe legal liability.
4. **Data Sovereignty & Privacy Regulations**: Legal and law enforcement footage contains sensitive, unreleased evidence that **cannot legally be uploaded to third-party public cloud AI APIs** (OpenAI, AWS Rekognition) due to CJIS (Criminal Justice Information Services) and HIPAA statutory restrictions.

### 1.2 The videoblur Solution
`videoblur` is an autonomous, zero-cloud multimodal redaction pipeline written in Go with optimized native FFmpeg/libav bindings and local ONNX runtime acceleration:
- **Zero-Cloud 100% Offline Execution**: Runs entirely on local CPU or GPU without transmitting a single byte of video or audio over external networks, adhering strictly to CJIS and HIPAA mandates.
- **Visual Multi-Class Detection & Temporal Tracking**: Employs lightweight local vision models (YOLO / SCRFD) to detect faces, license plates, and computer monitors, coupled with a Kalman-filter multi-object tracker to guarantee zero-flicker boundary continuity across occlusions and motion blur.
- **Phoneme-Aligned Audio PII Bleeping**: Transcribes audio locally using embedded Whisper, aligns entity timestamps at the phoneme level, and injects clean 1kHz censorship tones or mutes over spoken PII (names, numbers, addresses).
- **Cryptographic Non-Repudiation Certificate**: Emits an immutable, signed audit manifest (`redaction_audit.json`) containing frame-level SHA-256 hashes, detected entity counts, and verification checksums proving irreversible anonymization.
- **Embedded React 19 Review Studio**: Features an interactive in-browser video player allowing legal review officers to scrub timelines, adjust blur radii, override false detections, and certify final exports.

---

## 2. High-Level System Architecture

```
       +-------------------------------------------------------------+
       |             Raw Video Footage Input (MP4, MKV, MOV)         |
       |     (Police Bodycam, Hospital Video, Zoom Meeting Recording)|
       +-------------------------------------------------------------+
                                      |
                         Native FFmpeg Demuxing Pipeline
                                      v
       +-------------------------------------------------------------+
       |                      videoblur Core Engine                  |
       |                    (High-Performance Go CLI)                |
       |                                                             |
       |       Video Stream Processing        Audio Stream Processing|
       |                 |                              |            |
       |                 v                              v            |
       |  +-----------------------------+ +------------------------+ |
       |  | Visual Detection & Tracking | | Local Whisper ASR &    | |
       |  | - Face & License Plate Mesh | | Phoneme Timestamping   | |
       |  | - Computer Monitor OCR      | +------------------------+ |
       |  +-----------------------------+              |             |
       |                 |                             v             |
       |                 v                +------------------------+ |
       |  +-----------------------------+ | Spoken PII Extractor   | |
       |  | Temporal Kalman Tracker     | | (Names, SSNs, Addresses)| |
       |  | (Anti-Flicker Interpolation)| +------------------------+ |
       |  +-----------------------------+              |             |
       |                 |                             v             |
       |                 v                +------------------------+ |
       |  +-----------------------------+ | Audio Bleep / Muter    | |
       |  | Dynamic Gaussian Blur /     | | (1kHz Tone Injection)  | |
       |  | Mosaic Pixelation Kernel    | +------------------------+ |
       |  +-----------------------------+              |             |
       |                 \                             /             |
       |                  +-------------+-------------+              |
       |                                |                            |
       |                                v                            |
       |  +-------------------------------------------------------+  |
       |  |           FFmpeg Remuxer & Hardware Encoder           |  |
       |  |   H.264 / HEVC Zero-Copy Bitstream Synchronization    |  |
       |  +-------------------------------------------------------+  |
       |                                |                            |
       +--------------------------------|----------------------------+
                                        |
                                        v
       +-------------------------------------------------------------+
       |                  Certified Redacted Output                  |
       |  1. Irreversibly Anonymized Video File (`redacted.mp4`)     |
       |  2. Cryptographic Compliance Certificate (`audit.json`)     |
       |  3. Embedded React 19 Review Studio (Local HTTP :8181)      |
       +-------------------------------------------------------------+
```

---

## 3. Mathematical Tracking & Temporal Continuity Formulation

### 3.1 Kalman Filter Bounding Box State
To eliminate visual flicker when an object detector misses an intermediate frame, each target bounding box is represented as a state vector $\mathbf{x} = [u, v, s, r, \dot{u}, \dot{v}, \dot{s}]^T$, where $(u, v)$ is the bounding box center, $s$ is scale (area), and $r$ is aspect ratio:

$$\mathbf{x}_{k} = \mathbf{F} \mathbf{x}_{k-1} + \mathbf{w}_k$$

$$\mathbf{z}_k = \mathbf{H} \mathbf{x}_k + \mathbf{v}_k$$

If a face is occluded or undetected for $m \le 5$ consecutive frames, the Kalman prediction bridge maintains the blur mask, ensuring continuous anonymization.

### 3.2 Phoneme-Level Audio Alignment
For detected audio PII interval $[t_{\text{start}}, t_{\text{end}}]$, the audio stream $A(t)$ is modulated via cross-fade windowing:

$$A_{\text{redacted}}(t) = \begin{cases} 
A(t), & t < t_{\text{start}} - \delta \\
(1 - w(t)) \cdot A(t) + w(t) \cdot \sin(2\pi \cdot 1000 \cdot t), & t \in [t_{\text{start}} - \delta, t_{\text{start}}] \\
\sin(2\pi \cdot 1000 \cdot t), & t \in [t_{\text{start}}, t_{\text{end}}] \\
w(t) \cdot A(t) + (1 - w(t)) \cdot \sin(2\pi \cdot 1000 \cdot t), & t \in [t_{\text{end}}, t_{\text{end}} + \delta] \\
A(t), & t > t_{\text{end}} + \delta
\end{cases}$$

Where $\delta = 20\text{ms}$ smooths acoustic transition clicks.

---

## 4. Repository Structure

```
videoblur/
├── cmd/
│   └── videoblur/
│       └── main.go                 # CLI entry point, subcommands: redact, scan, certify, studio
├── internal/
│   ├── config/
│   │   ├── config.go               # YAML configuration for blur intensity, PII categories, codecs
│   │   └── schema.go               # Entity definitions (faces, plates, monitors, audio pii)
│   ├── media/
│   │   ├── demuxer.go              # FFmpeg demuxer extracting video and audio elementary streams
│   │   ├── muxer.go                # Remuxer with hardware-accelerated encoding (NVENC / QuickSync)
│   │   └── frame.go                # Video frame buffer and zero-allocation YUV/RGB converter
│   ├── vision/
│   │   ├── detector.go             # Local ONNX model runner for faces and vehicle license plates
│   │   ├── monitor_ocr.go          # Text and computer screen region detector
│   │   ├── tracker.go              # Kalman filter multi-object tracker preventing flicker
│   │   └── blur_kernel.go          # Dynamic Gaussian blur and mosaic pixelation shader
│   ├── audio/
│   │   ├── asr_whisper.go          # Embedded local Whisper speech-to-text runner
│   │   ├── pii_aligner.go          # Phoneme-level timestamp aligner for spoken sensitive data
│   │   └── bleeper.go              # Sine tone generator and audio interval mute injector
│   ├── audit/
│   │   ├── certificate.go          # Cryptographic SHA-256 frame-hash compliance manifest builder
│   │   └── signer.go               # Digital signature generator for legal discovery chains of custody
│   └── server/
│       ├── api.go                  # REST API for interactive timeline scrubbing and frame overrides
│       ├── embed.go                # Embedded React 19 review studio static assets via //go:embed
│       └── sse.go                  # Real-time video processing progress stream
├── web/
│   ├── src/
│   │   ├── App.tsx                 # React 19 root application
│   │   ├── components/
│   │   │   ├── VideoTimelinePlayer.tsx # Frame-accurate video player with toggleable bounding boxes
│   │   │   ├── EntityFilterBar.tsx # Toggles for Faces, License Plates, Screens, and Spoken PII
│   │   │   ├── AudioWaveformView.tsx # Synchronized audio waveform showing bleeped intervals
│   │   │   ├── CertificateBadge.tsx # Non-repudiation audit badge with legal verification hash
│   │   │   └── ExportActions.tsx   # One-click [Export Certified Video] and [Download Audit PDF]
│   │   └── pages/
│   │       └── ReviewPage.tsx      # Main Legal Review & Certification console
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── examples/
│   ├── videoblur.yaml              # Production redaction configuration
│   └── sample_media/               # Test video clips for offline benchmarking
│       ├── bodycam_traffic_stop.mp4
│       └── hospital_consultation.mp4
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## 5. Implementation Roadmap

### Phase 1: Go Engine Foundation and FFmpeg Demuxing
- Initialize Go module with minimal dependencies.
- Build native FFmpeg demuxer in `internal/media/demuxer.go` separating video bitstreams and audio tracks without transcoder generational loss.
- Implement frame buffer handling YUV420p and RGB24 color conversions.

### Phase 2: Visual Detection & Kalman Filter Tracking
- Integrate local ONNX runtime running face and license plate detection models.
- Implement Kalman filter multi-object tracker in `internal/vision/tracker.go` bridging occlusions up to 5 consecutive frames to eliminate flicker.
- Implement dynamic Gaussian blur and mosaic pixelation routines.

### Phase 3: Speech-to-Text and Phoneme-Level Audio Bleeping
- Integrate embedded local Whisper model transcribing audio tracks.
- Build PII alignment engine identifying spoken names, phone numbers, and Social Security numbers.
- Implement smooth 1kHz sine wave bleeper with 20ms cosine cross-fade envelopes.

### Phase 4: Hardware-Accelerated Remuxing & Encoding
- Re-encode redacted frames and audio tracks using hardware-accelerated H.264/HEVC encoding.
- Ensure audio and video streams remain frame-accurate and synchronized.

### Phase 5: Cryptographic Compliance Certification
- Calculate frame-by-frame SHA-256 hashes of the redacted output.
- Generate signed legal compliance certificate (`redaction_audit.json`) documenting all redaction events.

### Phase 6: Embedded React 19 Review Studio
- Develop modern web UI in `web/` featuring frame-accurate video player, synchronized audio waveform visualizer, and audit certificate generator.
- Embed frontend build artifacts into the Go binary via `//go:embed` and serve locally on `:8181`.

### Phase 7: Verification, Benchmarking, and Documentation
- Run benchmarks on `examples/sample_media` verifying 100% face tracking continuity without a single unblurred frame.
- Verify processing throughput exceeds 60 FPS on standard modern hardware.
- Author humanized README with setup guides, legal compliance playbooks (FOIA/HIPAA), and courtroom evidence chain-of-custody documentation.
