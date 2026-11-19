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

    // Look for matching sample frame detections
    const sampleRecord = manifest?.sample_frames?.find(
      (f) => Math.abs(f.timestamp_sec - currentTime) < 1.0 / fps
    );

    const scaleX = canvas.width / (manifest?.video_width || canvas.width);
    const scaleY = canvas.height / (manifest?.video_height || canvas.height);

    if (sampleRecord && sampleRecord.detections) {
      sampleRecord.detections.forEach((det: Detection) => {
        if (det.class === 'face' && !activeFilters.faces) return;
        if (det.class === 'license_plate' && !activeFilters.plates) return;
        if (det.class === 'computer_screen' && !activeFilters.screens) return;

        let strokeColor = '#3b82f6';
        if (det.class === 'license_plate') strokeColor = '#f59e0b';
        if (det.class === 'computer_screen') strokeColor = '#a855f7';

        const rx = det.box.x * scaleX;
        const ry = det.box.y * scaleY;
        const rw = det.box.w * scaleX;
        const rh = det.box.h * scaleY;

        ctx.strokeStyle = strokeColor;
        ctx.lineWidth = 2;
        ctx.strokeRect(rx, ry, rw, rh);

        ctx.fillStyle = strokeColor;
        ctx.font = '10px monospace';
        const label = `${det.class} ${Math.round(det.confidence * 100)}%`;
        ctx.fillText(label, rx, Math.max(12, ry - 4));
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
