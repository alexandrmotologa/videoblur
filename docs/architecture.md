# videoblur Architecture Specification

Autonomous multimodal video and audio redaction engine designed for legal discovery, law enforcement records, and healthcare compliance.

## System design

The engine operates 100% offline with zero cloud transmission. Video and audio elementary streams are processed locally via native FFmpeg process piping and Go routines, ensuring compliance with CJIS and HIPAA data sovereignty mandates.

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
       |  1. Irreversibly Anonymized Video File (redacted.mp4)       |
       |  2. Cryptographic Compliance Certificate (audit.json)       |
       |  3. Embedded React 19 Review Studio (Local HTTP :8181)      |
       +-------------------------------------------------------------+
```

## Mathematical formulation

### 1. Kalman filter bounding box state

To eliminate privacy breaches caused by detection misses across rapid camera movement, each bounding box target is modeled as a 7-dimensional continuous state vector:

$$x = [u, v, s, r, \dot{u}, \dot{v}, \dot{s}]^T$$

where:
- $(u, v)$ is the horizontal and vertical center in pixel coordinates
- $s$ is the bounding box scale (area $w \cdot h$)
- $r$ is aspect ratio ($w / h$)
- $\dot{u}, \dot{v}, \dot{s}$ are respective first-order velocities

State projection (dt = 1 frame):

$$x_k = F x_{k-1} + w_k$$

Measurement update:

$$z_k = H x_k + v_k$$

When an object detector fails to recognize a target for up to $m \le 8$ consecutive frames, the Kalman prediction bridge keeps the blur mask active at the projected location. This guarantees zero-flicker continuity.

### 2. Phoneme-level audio modulation

Spoken personal information intervals $[t_{\text{start}}, t_{\text{end}}]$ are replaced with a 1 kHz sine wave:

$$A_{\text{redacted}}(t) = (1 - w(t)) \cdot A(t) + w(t) \cdot \sin(2\pi \cdot 1000 \cdot t)$$

Boundary cross-fading occurs over a 20ms cosine window:

$$w(\tau) = 0.5 \cdot (1 - \cos(\pi \cdot \tau / 0.020))$$

This removes phase clicks and pops at transition boundaries.

## Cryptographic non-repudiation

The audit manifest calculates a sequential cumulative SHA-256 digest across all redacted video frames:

$$H_k = \text{SHA-256}(H_{k-1} \,||\, \text{FrameHash}_k)$$

The final root hash $H_N$ is signed using an Ed25519 digital keypair. The resulting signature is embedded directly in `redaction_audit.json`. Any subsequent alteration to either the video frames or the audit record invalidates the signature during verification with `videoblur certify`.
