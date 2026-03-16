"use client";

import { useRef, useCallback, useState, useEffect } from "react";

export type SoundName =
  | "fold"
  | "check"
  | "call"
  | "raise"
  | "allin"
  | "deal"
  | "showdown"
  | "pot-win"
  | "eliminated"
  | "game-over"
  | "timeout-tick";

const SOUND_PATHS: Record<SoundName, string> = {
  fold: "/sounds/fold.mp3",
  check: "/sounds/check.mp3",
  call: "/sounds/call.mp3",
  raise: "/sounds/raise.mp3",
  allin: "/sounds/allin.mp3",
  deal: "/sounds/deal.mp3",
  showdown: "/sounds/showdown.mp3",
  "pot-win": "/sounds/pot-win.mp3",
  eliminated: "/sounds/eliminated.mp3",
  "game-over": "/sounds/game-over.mp3",
  "timeout-tick": "/sounds/timeout-tick.mp3",
};

const ALL_SOUNDS = Object.keys(SOUND_PATHS) as SoundName[];

// Module-level singleton — shared across all hook instances / re-renders
let audioCtx: AudioContext | null = null;
let gainNode: GainNode | null = null;
const bufferCache = new Map<SoundName, AudioBuffer>();
let preloadStarted = false;

function getAudioContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext();
    gainNode = audioCtx.createGain();
    gainNode.connect(audioCtx.destination);
  }
  return audioCtx;
}

function getGainNode(): GainNode {
  getAudioContext();
  return gainNode!;
}

async function preloadAll() {
  if (preloadStarted) return;
  preloadStarted = true;

  const ctx = getAudioContext();

  await Promise.all(
    ALL_SOUNDS.map(async (name) => {
      if (bufferCache.has(name)) return;
      try {
        const res = await fetch(SOUND_PATHS[name]);
        const arrayBuf = await res.arrayBuffer();
        const audioBuf = await ctx.decodeAudioData(arrayBuf);
        bufferCache.set(name, audioBuf);
      } catch {
        // Silently ignore — sound just won't play
      }
    }),
  );
}

export function useSoundEffects() {
  const [muted, setMuted] = useState(false);
  const mutedRef = useRef(false);

  // Keep ref in sync
  useEffect(() => {
    mutedRef.current = muted;
    getGainNode().gain.value = muted ? 0 : 1;
  }, [muted]);

  // Preload all sounds on mount
  useEffect(() => {
    preloadAll();
  }, []);

  const toggleMute = useCallback(() => setMuted((m) => !m), []);

  const play = useCallback((name: SoundName, volume = 1.0) => {
    if (typeof window === "undefined") return;
    if (mutedRef.current) return;

    const buf = bufferCache.get(name);
    if (!buf) return;

    try {
      const ctx = getAudioContext();
      // Resume if suspended (browser autoplay policy)
      if (ctx.state === "suspended") {
        ctx.resume();
      }

      const source = ctx.createBufferSource();
      source.buffer = buf;

      // Per-sound volume control
      if (volume < 1.0) {
        const vol = ctx.createGain();
        vol.gain.value = Math.max(0, Math.min(1, volume));
        source.connect(vol);
        vol.connect(getGainNode());
      } else {
        source.connect(getGainNode());
      }

      source.start();
    } catch {
      // Ignore
    }
  }, []);

  const playAction = useCallback(
    (actionType: string) => {
      const map: Record<string, SoundName> = {
        fold: "fold",
        check: "check",
        call: "call",
        raise: "raise",
        allin: "allin",
      };
      const sound = map[actionType];
      if (sound) play(sound);
    },
    [play],
  );

  return { play, playAction, muted, toggleMute };
}
