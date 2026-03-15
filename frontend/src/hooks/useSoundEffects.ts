"use client";

import { useRef, useCallback, useState } from "react";

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

export function useSoundEffects() {
  const audioCache = useRef<Map<SoundName, HTMLAudioElement>>(new Map());
  const [muted, setMuted] = useState(false);

  const toggleMute = useCallback(() => setMuted((m) => !m), []);

  const play = useCallback((name: SoundName, volume = 1.0) => {
    if (typeof window === "undefined") return;
    if (muted) return;
    try {
      let audio = audioCache.current.get(name);
      if (!audio) {
        audio = new Audio(SOUND_PATHS[name]);
        audioCache.current.set(name, audio);
      }
      // Restart if already playing
      audio.currentTime = 0;
      audio.volume = Math.max(0, Math.min(1, volume));
      audio.play().catch(() => {
        // Autoplay may be blocked before user interaction — silently ignore
      });
    } catch {
      // Ignore any audio errors
    }
  }, [muted]);

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
