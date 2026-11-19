import { useState, useEffect } from 'react';
import { AuditManifest, VerificationResult, ManualOverride, Rect } from '../types';
import { VideoTimelinePlayer } from '../components/VideoTimelinePlayer';
import { EntityFilterBar } from '../components/EntityFilterBar';
import { CertificateBadge } from '../components/CertificateBadge';
import { ExportActions } from '../components/ExportActions';
import { OverridesManager } from '../components/OverridesManager';

export const ReviewPage: React.FC = () => {
  const [manifest, setManifest] = useState<AuditManifest | null>(null);
  const [verification, setVerification] = useState<VerificationResult | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [overrides, setOverrides] = useState<ManualOverride[]>([]);
  const [isReRendering, setIsReRendering] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);

  const [activeFilters, setActiveFilters] = useState({
    faces: true,
    plates: true,
    screens: true,
    audio: true,
  });

  const loadAudit = () => {
    fetch('/api/audit')
      .then((res) => {
        if (res.ok) return res.json();
        throw new Error('No audit loaded');
      })
      .then((data: AuditManifest) => setManifest(data))
      .catch((err) => console.warn('Could not load audit manifest:', err));
  };

  const loadOverrides = () => {
    fetch('/api/overrides')
      .then((res) => res.json())
      .then((data: ManualOverride[]) => {
        if (Array.isArray(data)) setOverrides(data);
      })
      .catch((err) => console.warn('Failed loading overrides:', err));
  };

  useEffect(() => {
    loadAudit();
    loadOverrides();
  }, []);

  const handleVerify = async () => {
    setIsVerifying(true);
    try {
      const res = await fetch('/api/certify', { method: 'POST' });
      const data: VerificationResult = await res.json();
      setVerification(data);
    } catch (err) {
      console.error('Certification check failed:', err);
    } finally {
      setIsVerifying(false);
    }
  };

  const handleToggleFilter = (key: 'faces' | 'plates' | 'screens' | 'audio') => {
    setActiveFilters((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleBoxDrawn = (box: Rect, frame: number) => {
    const newOv: ManualOverride = {
      id: 'ov_' + Date.now(),
      action: 'add',
      class: 'face',
      start_frame: Math.max(0, frame - 15),
      end_frame: frame + 45, // cover a 2-second interval around the frame
      box,
    };
    const updated = [...overrides, newOv];
    setOverrides(updated);
    setIsDrawing(false);

    // Save to server
    fetch('/api/overrides', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    }).catch(console.error);
  };

  const handleAddExemptTrack = (trackId: number) => {
    const newOv: ManualOverride = {
      id: 'ov_exempt_' + Date.now(),
      action: 'exempt',
      track_id: trackId,
      class: 'face',
      start_frame: 0,
      end_frame: 0, // 0 means entire video duration
      box: { x: 0, y: 0, w: 0, h: 0 },
    };
    const updated = [...overrides, newOv];
    setOverrides(updated);
    fetch('/api/overrides', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    }).catch(console.error);
  };

  const handleRemoveOverride = (id: string) => {
    const updated = overrides.filter((o) => o.id !== id);
    setOverrides(updated);
    fetch('/api/overrides', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    }).catch(console.error);
  };

  const handleApplyReRender = async () => {
    setIsReRendering(true);
    try {
      // 1. Save overrides
      await fetch('/api/overrides', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(overrides),
      });

      // 2. Trigger pipeline re-render
      const res = await fetch('/api/re-render', { method: 'POST' });
      if (res.ok) {
        loadAudit();
        setVerification(null);
      }
    } catch (err) {
      console.error('Re-render error:', err);
    } finally {
      setIsReRendering(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col bg-slate-950 text-slate-100">
      {/* Header */}
      <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur px-6 py-3 flex items-center justify-between sticky top-0 z-50">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded bg-blue-600 flex items-center justify-center font-mono font-bold text-white text-sm">
            VB
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-sm font-semibold tracking-tight text-white">videoblur Studio</h1>
              <span className="text-[11px] px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 font-mono border border-blue-500/20">
                React 19
              </span>
              <span className="text-[11px] px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-mono border border-emerald-500/20">
                Zero Cloud
              </span>
            </div>
            <p className="text-xs text-slate-400">Autonomous Legal Discovery & Compliance Redaction Console</p>
          </div>
        </div>

        <ExportActions manifest={manifest} />
      </header>

      {/* Main Grid Workspace */}
      <main className="flex-1 grid grid-cols-12 gap-0 overflow-hidden">
        {/* Left: Video Player (8 cols) */}
        <section className="col-span-8 flex flex-col border-r border-slate-800 bg-slate-950 p-6 overflow-y-auto">
          <VideoTimelinePlayer
            manifest={manifest}
            activeFilters={activeFilters}
            isDrawing={isDrawing}
            onBoxDrawn={handleBoxDrawn}
          />
        </section>

        {/* Right: Controls & Certificate (4 cols) */}
        <aside className="col-span-4 bg-slate-900/60 p-6 flex flex-col gap-5 overflow-y-auto">
          <OverridesManager
            overrides={overrides}
            onAddBoxPrompt={() => setIsDrawing(true)}
            onAddExemptTrack={handleAddExemptTrack}
            onRemoveOverride={handleRemoveOverride}
            onApplyReRender={handleApplyReRender}
            isReRendering={isReRendering}
            isDrawing={isDrawing}
            onToggleDrawing={() => setIsDrawing(!isDrawing)}
          />

          <EntityFilterBar
            visualStats={manifest?.visual_entities || { faces_detected: 0, license_plates_detected: 0, screens_detected: 0, id_badges_detected: 0 }}
            audioEntities={manifest?.audio_entities || []}
            activeFilters={activeFilters}
            onToggleFilter={handleToggleFilter}
          />

          <CertificateBadge
            manifest={manifest}
            verification={verification}
            onVerify={handleVerify}
            isVerifying={isVerifying}
          />

          {/* Spoken PII Transcript Log */}
          <div className="p-4 rounded-lg bg-slate-900 border border-slate-800 flex-1">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-3">
              Spoken PII Log
            </h2>
            <div className="space-y-2 text-xs overflow-y-auto max-h-48">
              {manifest?.audio_entities && manifest.audio_entities.length > 0 ? (
                manifest.audio_entities.map((ent, i) => (
                  <div
                    key={i}
                    className="p-2.5 rounded bg-slate-950 border border-slate-800 flex justify-between items-center"
                  >
                    <div>
                      <strong className="text-rose-400 capitalize">
                        {ent.class.replace('_', ' ')}
                      </strong>
                      : <span className="text-slate-300">"{ent.text}"</span>
                    </div>
                    <span className="font-mono text-slate-500">
                      {(ent.start_ms / 1000).toFixed(2)}s
                    </span>
                  </div>
                ))
              ) : (
                <p className="text-slate-500 italic">No spoken PII detected in audio stream.</p>
              )}
            </div>
          </div>
        </aside>
      </main>
    </div>
  );
};
