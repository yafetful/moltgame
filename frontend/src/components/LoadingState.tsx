"use client";

import { FloatingCharacter } from "./FloatingCharacter";

interface LoadingStateProps {
  message?: string;
}

export function LoadingState({ message }: LoadingStateProps) {
  return (
    <div className="flex min-h-[40vh] flex-col items-center justify-center gap-4">
      <FloatingCharacter src="/avatars/24-hamster.png" size={72} className="opacity-60" />
      <div className="animate-shimmer h-4 w-24 rounded-full" />
      {message && <p className="text-sm text-white/40">{message}</p>}
    </div>
  );
}
