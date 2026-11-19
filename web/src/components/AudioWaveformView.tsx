import { AudioEntity } from '../types';

interface AudioWaveformViewProps {
  durationSec: number;
  currentTimeSec: number;
  audioEntities: AudioEntity[];
  onSeek: (seconds: number) => void;
}

export const AudioWaveformView: React.FC<AudioWaveformViewProps> = ({
  durationSec,
  currentTimeSec,
  audioEntities,
  onSeek,
}) => {
  const safeDuration = durationSec > 0 ? durationSec : 1;
  const playheadPercent = Math.min(100, Math.max(0, (currentTimeSec / safeDuration) * 100));

  return (
    <div className="mt-3 pt-3 border-t border-slate-800">
      <div className="flex items-center justify-between text-xs text-slate-400 mb-1.5">
        <span className="flex items-center gap-1.5 font-medium">
          <span className="w-2 h-2 rounded-full bg-amber-400"></span>
          Synchronized Audio Redaction Intervals (1 kHz Sine Bleeps)
        </span>
        <span className="font-mono text-amber-400">
          {audioEntities.length} bleeped intervals
        </span>
      </div>

      <div
        className="h-7 w-full bg-slate-950 rounded border border-slate-800 relative overflow-hidden cursor-pointer"
        onClick={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const clickX = e.clientX - rect.left;
          const ratio = Math.max(0, Math.min(1, clickX / rect.width));
          onSeek(ratio * safeDuration);
        }}
      >
        {/* Render highlight bands for each bleeped interval */}
        {audioEntities.map((ent, idx) => {
          const leftPct = (ent.start_ms / (safeDuration * 1000)) * 100;
          const widthPct = Math.max(0.5, ((ent.end_ms - ent.start_ms) / (safeDuration * 1000)) * 100);
          return (
            <div
              key={idx}
              title={`${ent.class}: ${ent.text} (${(ent.start_ms / 1000).toFixed(2)}s - ${(ent.end_ms / 1000).toFixed(2)}s)`}
              style={{ left: `${leftPct}%`, width: `${widthPct}%` }}
              className="absolute top-0 bottom-0 bg-amber-500/30 border-x border-amber-400/80 hover:bg-amber-500/50 transition z-10"
            />
          );
        })}

        {/* Playhead line */}
        <div
          style={{ left: `${playheadPercent}%` }}
          className="absolute top-0 bottom-0 w-0.5 bg-blue-500 z-20 pointer-events-none"
        />
      </div>
    </div>
  );
};
