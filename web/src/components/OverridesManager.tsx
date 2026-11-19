import { useState } from 'react';
import { ManualOverride } from '../types';

interface OverridesManagerProps {
  overrides: ManualOverride[];
  onAddBoxPrompt: () => void;
  onAddExemptTrack: (trackId: number) => void;
  onRemoveOverride: (id: string) => void;
  onApplyReRender: () => void;
  isReRendering: boolean;
  isDrawing: boolean;
  onToggleDrawing: () => void;
}

export const OverridesManager: React.FC<OverridesManagerProps> = ({
  overrides,
  onAddExemptTrack,
  onRemoveOverride,
  onApplyReRender,
  isReRendering,
  isDrawing,
  onToggleDrawing,
}) => {
  const [exemptInput, setExemptInput] = useState('');

  const handleExemptSubmit = () => {
    const trackId = parseInt(exemptInput, 10);
    if (!isNaN(trackId)) {
      onAddExemptTrack(trackId);
      setExemptInput('');
    }
  };

  return (
    <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Human Officer Overrides
        </h2>
        <button
          type="button"
          onClick={onToggleDrawing}
          className={`text-xs px-2.5 py-1 rounded font-medium border transition ${
            isDrawing
              ? 'bg-amber-500/20 border-amber-500/40 text-amber-300'
              : 'bg-slate-800 border-slate-700 text-slate-300 hover:bg-slate-700'
          }`}
        >
          {isDrawing ? 'Cancel Drawing' : '+ Draw Box On Frame'}
        </button>
      </div>

      {/* Exempt Track Input */}
      <div className="flex gap-2 mb-3">
        <input
          type="number"
          placeholder="Exempt Track ID (e.g. 1)"
          value={exemptInput}
          onChange={(e) => setExemptInput(e.target.value)}
          className="flex-1 px-2.5 py-1 text-xs rounded bg-slate-950 border border-slate-800 text-slate-200"
        />
        <button
          type="button"
          onClick={handleExemptSubmit}
          disabled={!exemptInput}
          className="px-2.5 py-1 text-xs rounded bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 disabled:opacity-40"
        >
          Exempt
        </button>
      </div>

      <div className="space-y-2 mb-3 max-h-32 overflow-y-auto">
        {overrides.length === 0 ? (
          <p className="text-xs text-slate-500 italic">No manual overrides or exemptions active.</p>
        ) : (
          overrides.map((ov) => (
            <div
              key={ov.id}
              className="p-2 rounded bg-slate-950 border border-slate-800 flex items-center justify-between text-xs"
            >
              <div>
                <span
                  className={`font-mono uppercase font-bold text-[10px] px-1.5 py-0.5 rounded mr-1.5 ${
                    ov.action === 'add'
                      ? 'bg-blue-500/20 text-blue-400'
                      : ov.action === 'exempt'
                      ? 'bg-emerald-500/20 text-emerald-400'
                      : 'bg-rose-500/20 text-rose-400'
                  }`}
                >
                  {ov.action}
                </span>
                <span className="text-slate-200">
                  {ov.class || 'target'}
                  {ov.track_id ? ` (Track #${ov.track_id})` : ''}
                </span>
              </div>
              <button
                type="button"
                onClick={() => onRemoveOverride(ov.id)}
                className="text-slate-500 hover:text-rose-400 font-mono text-xs px-1"
                title="Remove override"
              >
                ✕
              </button>
            </div>
          ))
        )}
      </div>

      <button
        type="button"
        onClick={onApplyReRender}
        disabled={isReRendering || overrides.length === 0}
        className="w-full py-1.5 rounded text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white transition disabled:opacity-40"
      >
        {isReRendering ? 'Re-rendering Video Pipeline...' : 'Apply Overrides & Re-Render'}
      </button>
    </div>
  );
};
