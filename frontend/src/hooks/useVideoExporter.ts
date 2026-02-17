"use client";

import { useCallback, useRef, useState } from "react";
import { domToCanvas } from "modern-screenshot";
import { Muxer, ArrayBufferTarget } from "mp4-muxer";

export interface ExportProgress {
  phase: "idle" | "capturing" | "done" | "error";
  percent: number;
}

export interface ExportOptions {
  width?: number;
  height?: number;
  watermarkText?: string;
  bitrate?: number;
}

const VIDEO_FPS = 30;
const FRAME_US = Math.round(1_000_000 / VIDEO_FPS);

const DEFAULT_OPTIONS: Required<ExportOptions> = {
  width: 1280,
  height: 720,
  watermarkText: "moltgame.com",
  bitrate: 4_000_000,
};

export function useVideoExporter() {
  const [progress, setProgress] = useState<ExportProgress>({
    phase: "idle",
    percent: 0,
  });
  const abortRef = useRef(false);

  const abort = useCallback(() => {
    abortRef.current = true;
  }, []);

  /**
   * Record the container in real-time for `durationMs` milliseconds.
   * Caller is responsible for starting playback before calling this.
   */
  const startRecording = useCallback(
    async (
      containerRef: React.RefObject<HTMLDivElement | null>,
      durationMs: number,
      options?: ExportOptions,
    ) => {
      const opts = { ...DEFAULT_OPTIONS, ...options };
      const container = containerRef.current;
      if (!container || durationMs <= 0) return;

      if (typeof VideoEncoder === "undefined") {
        console.error("[VideoExporter] WebCodecs API not supported.");
        setProgress({ phase: "error", percent: 0 });
        return;
      }

      abortRef.current = false;
      setProgress({ phase: "capturing", percent: 0 });

      const rect = container.getBoundingClientRect();
      const scale = opts.width / rect.width;

      // Offscreen canvas for compositing
      const compositeCanvas = document.createElement("canvas");
      compositeCanvas.width = opts.width;
      compositeCanvas.height = opts.height;
      const ctx = compositeCanvas.getContext("2d")!;

      // Set up encoder + muxer
      const muxer = new Muxer({
        target: new ArrayBufferTarget(),
        video: { codec: "avc", width: opts.width, height: opts.height },
        fastStart: "in-memory",
      });

      const encoder = new VideoEncoder({
        output: (chunk, meta) => muxer.addVideoChunk(chunk, meta ?? undefined),
        error: (e) => console.error("[VideoExporter] Encoder error:", e),
      });

      encoder.configure({
        codec: "avc1.640028",
        width: opts.width,
        height: opts.height,
        bitrate: opts.bitrate,
        framerate: VIDEO_FPS,
      });

      const startTime = performance.now();
      let frameCount = 0;

      try {
        // Real-time capture loop
        while (!abortRef.current) {
          const elapsed = performance.now() - startTime;
          if (elapsed >= durationMs) break;

          const capture = await domToCanvas(container, { scale });

          ctx.fillStyle = "#0a0a0a";
          ctx.fillRect(0, 0, opts.width, opts.height);

          const dx = Math.round((opts.width - capture.width) / 2);
          const dy = Math.round((opts.height - capture.height) / 2);
          ctx.drawImage(capture, dx, dy);

          if (opts.watermarkText) {
            ctx.save();
            ctx.font = "14px sans-serif";
            ctx.fillStyle = "rgba(255, 255, 255, 0.4)";
            ctx.textAlign = "right";
            ctx.textBaseline = "bottom";
            ctx.fillText(opts.watermarkText, opts.width - 16, opts.height - 12);
            ctx.restore();
          }

          const bmp = await createImageBitmap(compositeCanvas);
          const timestampUs = Math.round(elapsed * 1000);

          const vf = new VideoFrame(bmp, {
            timestamp: timestampUs,
            duration: FRAME_US,
          });
          encoder.encode(vf, { keyFrame: frameCount % (VIDEO_FPS * 2) === 0 });
          vf.close();
          bmp.close();
          frameCount++;

          setProgress({
            phase: "capturing",
            percent: Math.round((elapsed / durationMs) * 95),
          });

          // Yield to browser to keep UI responsive + let animations advance
          await waitForPaint();
        }

        if (frameCount === 0 || abortRef.current) {
          encoder.close();
          setProgress(abortRef.current
            ? { phase: "idle", percent: 0 }
            : { phase: "error", percent: 0 });
          return;
        }

        await encoder.flush();
        encoder.close();
        muxer.finalize();

        const buf = (muxer.target as ArrayBufferTarget).buffer;
        const blob = new Blob([buf], { type: "video/mp4" });

        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `moltgame-replay-${Date.now()}.mp4`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);

        setProgress({ phase: "done", percent: 100 });
      } catch (err) {
        console.error("[VideoExporter] Export failed:", err);
        setProgress({ phase: "error", percent: 0 });
      }
    },
    [],
  );

  const reset = useCallback(() => {
    setProgress({ phase: "idle", percent: 0 });
  }, []);

  return { startRecording, progress, abort, reset };
}

function waitForPaint() {
  return new Promise<void>((resolve) => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => resolve());
    });
  });
}
