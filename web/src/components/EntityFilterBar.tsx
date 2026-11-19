import { EntityStats, AudioEntity } from '../types';

interface EntityFilterBarProps {
  visualStats: EntityStats;
  audioEntities: AudioEntity[];
  activeFilters: {
    faces: boolean;
    plates: boolean;
    screens: boolean;
    audio: boolean;
  };
  onToggleFilter: (key: 'faces' | 'plates' | 'screens' | 'audio') => void;
}

export const EntityFilterBar: React.FC<EntityFilterBarProps> = ({
  visualStats,
  audioEntities,
  activeFilters,
  onToggleFilter,
}) => {
  return (
    <div className="p-4 rounded-lg bg-slate-900 border border-slate-800">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-3">
        Redaction Categories
      </h2>
      <div className="space-y-2">
        <button
          type="button"
          onClick={() => onToggleFilter('faces')}
          className={`w-full flex items-center justify-between p-2.5 rounded border text-left transition ${
            activeFilters.faces
              ? 'bg-blue-950/40 border-blue-500/40 text-blue-100'
              : 'bg-slate-950/40 border-slate-800 text-slate-400 opacity-60'
          }`}
        >
          <span className="text-sm font-medium flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded bg-blue-500"></span> Human Faces
          </span>
          <span className="text-xs font-mono px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">
            {visualStats?.faces_detected ?? 0}
          </span>
        </button>

        <button
          type="button"
          onClick={() => onToggleFilter('plates')}
          className={`w-full flex items-center justify-between p-2.5 rounded border text-left transition ${
            activeFilters.plates
              ? 'bg-amber-950/40 border-amber-500/40 text-amber-100'
              : 'bg-slate-950/40 border-slate-800 text-slate-400 opacity-60'
          }`}
        >
          <span className="text-sm font-medium flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded bg-amber-500"></span> License Plates
          </span>
          <span className="text-xs font-mono px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
            {visualStats?.license_plates_detected ?? 0}
          </span>
        </button>

        <button
          type="button"
          onClick={() => onToggleFilter('screens')}
          className={`w-full flex items-center justify-between p-2.5 rounded border text-left transition ${
            activeFilters.screens
              ? 'bg-purple-950/40 border-purple-500/40 text-purple-100'
              : 'bg-slate-950/40 border-slate-800 text-slate-400 opacity-60'
          }`}
        >
          <span className="text-sm font-medium flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded bg-purple-500"></span> Computer Monitors
          </span>
          <span className="text-xs font-mono px-2 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">
            {visualStats?.screens_detected ?? 0}
          </span>
        </button>

        <button
          type="button"
          onClick={() => onToggleFilter('audio')}
          className={`w-full flex items-center justify-between p-2.5 rounded border text-left transition ${
            activeFilters.audio
              ? 'bg-rose-950/40 border-rose-500/40 text-rose-100'
              : 'bg-slate-950/40 border-slate-800 text-slate-400 opacity-60'
          }`}
        >
          <span className="text-sm font-medium flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded bg-rose-500"></span> Spoken PII (Audio Bleeps)
          </span>
          <span className="text-xs font-mono px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/20">
            {audioEntities?.length ?? 0}
          </span>
        </button>
      </div>
    </div>
  );
};
