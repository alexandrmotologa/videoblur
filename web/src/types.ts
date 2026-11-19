export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Detection {
  class: 'face' | 'license_plate' | 'computer_screen' | 'id_badge';
  confidence: number;
  box: Rect;
  track_id?: number;
}

export interface AudioEntity {
  class: 'person_name' | 'phone_number' | 'ssn_cnp' | 'email_address' | 'credit_card' | 'physical_address';
  text: string;
  start_ms: number;
  end_ms: number;
  confidence: number;
}

export interface ManualOverride {
  id: string;
  action: 'add' | 'exempt' | 'delete';
  track_id?: number;
  class: 'face' | 'license_plate' | 'computer_screen' | 'id_badge';
  start_frame: number;
  end_frame: number;
  box: Rect;
}

export interface FrameRecord {
  frame_index: number;
  timestamp_sec: number;
  detections: Detection[];
  frame_sha256: string;
}

export interface EntityStats {
  faces_detected: number;
  license_plates_detected: number;
  screens_detected: number;
  id_badges_detected: number;
}

export interface AuditManifest {
  manifest_version: string;
  source_video: string;
  redacted_video: string;
  timestamp: string;
  compliance_standard: string;
  auditor_identity: string;
  total_frames: number;
  duration_sec: number;
  video_width: number;
  video_height: number;
  fps: number;
  visual_entities: EntityStats;
  audio_entities: AudioEntity[];
  total_bleeps: number;
  frame_hash_root: string;
  sample_frames?: FrameRecord[];
  signature_hex?: string;
  public_key_pem?: string;
}

export interface VerificationResult {
  is_valid: boolean;
  signature_valid: boolean;
  frame_hashes_valid: boolean;
  compliance_standard: string;
  auditor_identity: string;
  total_frames: number;
  total_redactions: number;
  errors?: string[];
}
