import { AuditManifest, VerificationResult } from '../types';

interface CertificateBadgeProps {
  manifest: AuditManifest | null;
  verification: VerificationResult | null;
  onVerify: () => void;
  isVerifying: boolean;
}

export const CertificateBadge: React.FC<CertificateBadgeProps> = ({
  manifest,
  verification,
  onVerify,
  isVerifying,
}) => {
  return (
    <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Cryptographic Certificate
        </h2>
        <button
          type="button"
          onClick={onVerify}
          disabled={isVerifying}
          className="text-xs font-medium px-2.5 py-1 rounded bg-blue-600 hover:bg-blue-500 text-white disabled:opacity-50 transition"
        >
          {isVerifying ? 'Verifying...' : 'Verify Signature'}
        </button>
      </div>

      <div className="space-y-2 text-xs">
        <div className="flex justify-between py-1 border-b border-slate-800/80">
          <span className="text-slate-400">Standard:</span>
          <span className="font-mono text-emerald-400 font-medium">
            {manifest?.compliance_standard || 'CJIS Evidence Grade'}
          </span>
        </div>
        <div className="flex justify-between py-1 border-b border-slate-800/80">
          <span className="text-slate-400">Auditor:</span>
          <span className="font-mono text-slate-200 truncate max-w-[180px]">
            {manifest?.auditor_identity || 'videoblur-autonomous-agent'}
          </span>
        </div>
        <div className="flex justify-between py-1 border-b border-slate-800/80">
          <span className="text-slate-400">Total Frames:</span>
          <span className="font-mono text-slate-200">
            {manifest?.total_frames ?? '--'}
          </span>
        </div>
        <div className="flex flex-col py-1 border-b border-slate-800/80">
          <span className="text-slate-400 mb-1">Cumulative Frame Hash Root:</span>
          <span className="font-mono text-[10px] text-slate-300 truncate bg-slate-950 p-1.5 rounded border border-slate-800">
            {manifest?.frame_hash_root || 'pending'}
          </span>
        </div>
        <div className="flex flex-col py-1">
          <span className="text-slate-400 mb-1">Ed25519 Signature:</span>
          <span className="font-mono text-[10px] text-emerald-400 truncate bg-slate-950 p-1.5 rounded border border-slate-800">
            {manifest?.signature_hex || 'pending'}
          </span>
        </div>
      </div>

      {verification && (
        <div
          className={`mt-3 p-2.5 rounded text-xs font-mono border ${
            verification.is_valid && verification.signature_valid
              ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
              : 'bg-rose-500/10 border-rose-500/20 text-rose-400'
          }`}
        >
          {verification.is_valid && verification.signature_valid
            ? '✓ Signature Verified: Irreversible Redaction & 0 Tampering'
            : '✗ Verification Failed: Signature or Data Mismatch'}
        </div>
      )}
    </div>
  );
};
