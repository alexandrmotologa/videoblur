import { useRef, useState, useEffect } from 'react';
import { AuditManifest, Detection, Rect } from '../types';
import { AudioWaveformView } from './AudioWaveformView';

interface VideoTimelinePlayerProps {
  manifest: AuditManifest | null;
  activeFilters: {
    faces: boolean;
    plates: boolean;
    screens: boolean;
    audio: boolean;
  };
  isDrawing?: boolean;
  onBoxDrawn?: (box: Rect, frame: number) => void;
}

export const VideoTimelinePlayer: React.FC<VideoTimelinePlayerProps> = ({
  manifest,
  activeFilters,
  isDrawing = false,
  onBoxDrawn,
}) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  // Drawing state
  const [isMouseDown, setIsMouseDown] = useState(false);
  const [dragStart, setDragStart] = useState<{ x: number; y: number } | null>(null);
  const [currentDrag, setCurrentDrag] = useState<{ x: number; y: number } | null>(null);

  const fps = manifest?.fps || 30.0;
  const currentFrame = Math.floor(currentTime * fps);

  const togglePlay = () => {
    if (!videoRef.current) return;
    if (videoRef.current.paused) {
      videoRef.current.play();
      setIsPlaying(true);
    } else {
      videoRef.current.pause();
      setIsPlaying(false);
    }
  };

  const stepFrame = (delta: number) => {
    if (!videoRef.current) return;
    videoRef.current.pause();
    setIsPlaying(false);
    const newTime = Math.max(0, Math.min(duration, currentTime + delta / fps));
    videoRef.current.currentTime = newTime;
    setCurrentTime(newTime);
  };

  const handleSeek = (newTime: number) => {
    if (!videoRef.current) return;
    videoRef.current.currentTime = newTime;
    setCurrentTime(newTime);
  };

  // Canvas drawing: render detection overlays and manual drawing rect
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Look for matching sample frame detections or closest frame within 0.35s
    let sampleRecord = manifest?.sample_frames?.find(
      (f) => Math.abs(f.timestamp_sec - currentTime) < 1.0 / fps
    );

    if (!sampleRecord && manifest?.sample_frames && manifest.sample_frames.length > 0) {
      let minDiff = 0.35;
      for (const f of manifest.sample_frames) {
        const diff = Math.abs(f.timestamp_sec - currentTime);
        if (diff < minDiff) {
          minDiff = diff;
          sampleRecord = f;
        }
      }
    }

    const scaleX = canvas.width / (manifest?.video_width || canvas.width);
    const scaleY = canvas.height / (manifest?.video_height || canvas.height);

    if (sampleRecord && sampleRecord.detections) {
      // Deduplicate visual detections for crisp, non-overlapping HUD overlay
      const validDets = sampleRecord.detections
        .filter((det) => {
          if (det.class === 'face' && !activeFilters.faces) return false;
          if (det.class === 'license_plate' && !activeFilters.plates) return false;
          if (det.class === 'computer_screen' && !activeFilters.screens) return false;
          return true;
        })
        .sort((a, b) => b.confidence - a.confidence);

      const renderDets: Detection[] = [];
      for (const d of validDets) {
        const hasOverlap = renderDets.some((existing) => {
          const dx = Math.abs((existing.box.x + existing.box.w / 2) - (d.box.x + d.box.w / 2));
          const dy = Math.abs((existing.box.y + existing.box.h / 2) - (d.box.y + d.box.h / 2));
          const maxDim = Math.max(existing.box.w, existing.box.h, d.box.w, d.box.h);
          return dx < maxDim * 1.1 && dy < maxDim * 1.1;
        });
        if (!hasOverlap) {
          renderDets.push(d);
        }
      }

      renderDets.forEach((det: Detection) => {
        let strokeColor = '#38bdf8'; // sky-400
        let bgTag = 'rgba(15, 23, 42, 0.90)';
        let classTitle = 'FACE';
        if (det.class === 'license_plate') {
          strokeColor = '#fbbf24'; // amber-400
          classTitle = 'PLATE';
        } else if (det.class === 'computer_screen') {
          strokeColor = '#c084fc'; // purple-400
          classTitle = 'SCREEN';
        } else if (det.class === 'id_badge') {
          strokeColor = '#34d399'; // emerald-400
          classTitle = 'BADGE';
        }

        const rx = det.box.x * scaleX;
        const ry = det.box.y * scaleY;
        const rw = det.box.w * scaleX;
        const rh = det.box.h * scaleY;

        // 1. Translucent border
        ctx.strokeStyle = strokeColor;
        ctx.lineWidth = 1.5;
        ctx.strokeRect(rx, ry, rw, rh);

        // 2. Tactical HUD Corner Brackets
        const cornerLen = Math.min(10, Math.min(rw, rh) / 3);
        ctx.lineWidth = 2.5;
        ctx.beginPath();
        // Top-left
        ctx.moveTo(rx, ry + cornerLen);
        ctx.lineTo(rx, ry);
        ctx.lineTo(rx + cornerLen, ry);
        // Top-right
        ctx.moveTo(rx + rw - cornerLen, ry);
        ctx.lineTo(rx + rw, ry);
        ctx.lineTo(rx + rw, ry + cornerLen);
        // Bottom-left
        ctx.moveTo(rx, ry + rh - cornerLen);
        ctx.lineTo(rx, ry + rh);
        ctx.lineTo(rx + cornerLen, ry + rh);
        // Bottom-right
        ctx.moveTo(rx + rw - cornerLen, ry + rh);
        ctx.lineTo(rx + rw, ry + rh);
        ctx.lineTo(rx + rw, ry + rh - cornerLen);
        ctx.stroke();

        // 3. Crisp Pill Badge for Label
        const trackStr = det.track_id && det.track_id > 0 ? `#${det.track_id} ` : '';
        const confStr = `${Math.round(det.confidence * 100)}%`;
        const labelText = `${trackStr}${classTitle} • ${confStr}`;

        ctx.font = 'bold 10px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace';
        const textMetrics = ctx.measureText(labelText);
        const tagW = textMetrics.width + 16;
        const tagH = 17;
        const tagX = rx;
        const tagY = Math.max(0, ry - tagH - 3);

        // Draw pill rounded background
        ctx.fillStyle = bgTag;
        ctx.strokeStyle = strokeColor;
        ctx.lineWidth = 1;
        ctx.beginPath();
        if (ctx.roundRect) {
          ctx.roundRect(tagX, tagY, tagW, tagH, 3);
        } else {
          ctx.rect(tagX, tagY, tagW, tagH);
        }
        ctx.fill();
        ctx.stroke();

        // Status indicator dot inside pill
        ctx.fillStyle = strokeColor;
        ctx.beginPath();
        ctx.arc(tagX + 7, tagY + tagH / 2, 2.5, 0, Math.PI * 2);
        ctx.fill();

        // Text label
        ctx.fillStyle = '#f8fafc';
        ctx.fillText(labelText, tagX + 14, tagY + 12);
      });
    }

    // Render interactive user drag box
    if (isDrawing && isMouseDown && dragStart && currentDrag) {
      const x = Math.min(dragStart.x, currentDrag.x);
      const y = Math.min(dragStart.y, currentDrag.y);
      const w = Math.abs(currentDrag.x - dragStart.x);
      const h = Math.abs(currentDrag.y - dragStart.y);

      ctx.strokeStyle = '#eab308';
      ctx.lineWidth = 2;
      ctx.setLineDash([4, 4]);
      ctx.strokeRect(x, y, w, h);
      ctx.fillStyle = 'rgba(234, 179, 8, 0.2)';
      ctx.fillRect(x, y, w, h);
      ctx.setLineDash([]);
    }
  }, [currentTime, manifest, activeFilters, fps, isDrawing, isMouseDown, dragStart, currentDrag]);

  // Mouse event handlers for interactive drawing
  const handleCanvasMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!isDrawing) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    setIsMouseDown(true);
    setDragStart({ x, y });
    setCurrentDrag({ x, y });
  };

  const handleCanvasMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!isDrawing || !isMouseDown) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    setCurrentDrag({ x, y });
  };

  const handleCanvasMouseUp = () => {
    if (!isDrawing || !isMouseDown || !dragStart || !currentDrag || !canvasRef.current) return;
    setIsMouseDown(false);

    const canvas = canvasRef.current;
    const scaleX = (manifest?.video_width || canvas.width) / canvas.width;
    const scaleY = (manifest?.video_height || canvas.height) / canvas.height;

    const x = Math.round(Math.min(dragStart.x, currentDrag.x) * scaleX);
    const y = Math.round(Math.min(dragStart.y, currentDrag.y) * scaleY);
    const w = Math.round(Math.abs(currentDrag.x - dragStart.x) * scaleX);
    const h = Math.round(Math.abs(currentDrag.y - dragStart.y) * scaleY);

    if (w > 5 && h > 5 && onBoxDrawn) {
      onBoxDrawn({ x, y, w, h }, currentFrame);
    }

    setDragStart(null);
    setCurrentDrag(null);
  };

  const formatTimecode = (sec: number) => {
    const mins = Math.floor(sec / 60);
    const secs = (sec % 60).toFixed(2);
    return `${String(mins).padStart(2, '0')}:${String(secs).padStart(5, '0')}`;
  };

  return (
    <div className="flex flex-col">
      {/* Video Container */}
      <div className="relative rounded-lg overflow-hidden border border-slate-800 bg-black aspect-video flex items-center justify-center shadow-2xl">
        <video
          ref={videoRef}
          className="w-full h-full object-contain"
          src="/api/video"
          preload="metadata"
          onLoadedMetadata={(e) => {
            const v = e.currentTarget;
            setDuration(v.duration);
            if (canvasRef.current) {
              canvasRef.current.width = v.videoWidth || 1280;
              canvasRef.current.height = v.videoHeight || 720;
            }
          }}
          onTimeUpdate={(e) => {
            setCurrentTime(e.currentTarget.currentTime);
          }}
          onEnded={() => setIsPlaying(false)}
        />
        <canvas
          ref={canvasRef}
          onMouseDown={handleCanvasMouseDown}
          onMouseMove={handleCanvasMouseMove}
          onMouseUp={handleCanvasMouseUp}
          className={`absolute inset-0 w-full h-full ${
            isDrawing ? 'cursor-crosshair pointer-events-auto' : 'pointer-events-none'
          }`}
        />

        {/* HUD OSD Badges */}
        <div className="absolute top-3 left-3 flex items-center gap-2 pointer-events-none">
          <div className="px-2.5 py-1 rounded bg-slate-950/85 backdrop-blur border border-slate-700/60 flex items-center gap-2 text-[11px] font-mono text-slate-300 shadow-lg">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <span>AXON BODY 3</span>
            <span className="text-slate-600">|</span>
            <span className="text-emerald-400 font-semibold">CJIS COMPLIANT</span>
          </div>
        </div>

        <div className="absolute top-3 right-3 hidden sm:flex items-center gap-2 pointer-events-none">
          <div className="px-2.5 py-1 rounded bg-slate-950/85 backdrop-blur border border-slate-700/60 text-[11px] font-mono text-cyan-400 shadow-lg">
            KALMAN 7-STATE ACTIVE
          </div>
        </div>
      </div>

      {/* Control Panel */}
      <div className="mt-4 p-4 rounded-lg bg-slate-900 border border-slate-800">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={togglePlay}
              className="w-8 h-8 rounded bg-slate-800 hover:bg-slate-700 flex items-center justify-center text-slate-200 transition"
              title={isPlaying ? 'Pause (Space)' : 'Play (Space)'}
            >
              {isPlaying ? (
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                  <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
                </svg>
              ) : (
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
              )}
            </button>

            <button
              type="button"
              onClick={() => stepFrame(-1)}
              className="px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-mono transition"
              title="Previous Frame"
            >
              -1 Fr
            </button>
            <button
              type="button"
              onClick={() => stepFrame(1)}
              className="px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-mono transition"
              title="Next Frame"
            >
              +1 Fr
            </button>

            <span className="font-mono text-xs text-slate-300 ml-2">
              {formatTimecode(currentTime)} / {formatTimecode(duration)}
            </span>
          </div>

          <div className="flex items-center gap-3 text-xs">
            <span className="text-slate-400">
              FPS: <strong className="text-slate-200">{fps.toFixed(1)}</strong>
            </span>
            <span className="text-slate-700">|</span>
            <span className="text-slate-400">
              Frame: <strong className="text-blue-400 font-mono">{currentFrame}</strong>
            </span>
          </div>
        </div>

        {/* Scrubber slider */}
        <input
          type="range"
          min={0}
          max={duration || 100}
          step={1 / fps}
          value={currentTime}
          onChange={(e) => handleSeek(parseFloat(e.target.value))}
          className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer"
        />

        {/* Audio Waveform & Bleep timeline */}
        <AudioWaveformView
          durationSec={duration}
          currentTimeSec={currentTime}
          audioEntities={manifest?.audio_entities || []}
          onSeek={handleSeek}
        />
      </div>
    </div>
  );
};
