import { useState, useEffect } from 'react';
import { AuditManifest, VerificationResult } from '../types';
import { VideoTimelinePlayer } from '../components/VideoTimelinePlayer';
import { EntityFilterBar } from '../components/EntityFilterBar';
import { CertificateBadge } from '../components/CertificateBadge';
import { ExportActions } from '../components/ExportActions';

export const ReviewPage: React.FC = () => {
  const [manifest, setManifest] = useState<AuditManifest | null>(null);
  const [verification, setVerification] = useState<VerificationResult | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [activeFilters, setActiveFilters] = useState({
    faces: true,
    plates: true,
    screens: true,
    audio: true,
  });

  useEffect(() => {
    fetch('/api/audit')
      .then((res) => {
        if (res.ok) return res.json();
        throw new Error('No audit loaded');
      })
      .then((data: AuditManifest) => setManifest(data))
      .catch((err) => console.warn('Could not load audit manifest:', err));
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
          <VideoTimelinePlayer manifest={manifest} activeFilters={activeFilters} />
        </section>

        {/* Right: Controls & Certificate (4 cols) */}
        <aside className="col-span-4 bg-slate-900/60 p-6 flex flex-col gap-5 overflow-y-auto">
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
