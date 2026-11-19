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
    <div className="flex flex-wrap items-center gap-2">
      <button
        type="button"
        onClick={downloadAuditJSON}
        disabled={!manifest}
        className="px-2.5 sm:px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition disabled:opacity-40"
      >
        Audit JSON
      </button>

      <a
        href="/api/pdf"
        download="redaction_certificate.pdf"
        className="px-2.5 sm:px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition flex items-center gap-1.5"
      >
        <svg className="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
        </svg>
        Court PDF
      </a>

      <a
        href="/api/video"
        download="redacted_video.mp4"
        className="px-3 sm:px-3.5 py-1.5 rounded text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white transition flex items-center gap-1.5"
      >
        Export Certified Video
      </a>
    </div>
  );
};
