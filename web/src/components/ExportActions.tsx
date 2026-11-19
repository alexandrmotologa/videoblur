import { AuditManifest } from '../types';

interface ExportActionsProps {
  manifest: AuditManifest | null;
}

export const ExportActions: React.FC<ExportActionsProps> = ({ manifest }) => {
  const downloadAuditJSON = () => {
    if (!manifest) return;
    const blob = new Blob([JSON.stringify(manifest, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'redaction_audit.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex items-center gap-3">
      <button
        type="button"
        onClick={downloadAuditJSON}
        disabled={!manifest}
        className="px-3.5 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition disabled:opacity-40"
      >
        Download Audit JSON
      </button>

      <a
        href="/api/video"
        download="redacted_video.mp4"
        className="px-3.5 py-1.5 rounded text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white transition flex items-center gap-1.5"
      >
        Export Certified Video
      </a>
    </div>
  );
};
