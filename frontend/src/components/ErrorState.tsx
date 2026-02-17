"use client";

import Image from "next/image";

interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
  retryLabel?: string;
}

export function ErrorState({ message, onRetry, retryLabel = "Retry" }: ErrorStateProps) {
  return (
    <div className="flex min-h-[40vh] flex-col items-center justify-center gap-4">
      <Image
        src="/avatars/08-raccoon.png"
        alt=""
        width={72}
        height={72}
        className="opacity-40"
      />
      {message && <p className="text-sm text-white/50">{message}</p>}
      {onRetry && (
        <button
          onClick={onRetry}
          className="rounded-lg bg-brand-primary/20 px-4 py-2 text-sm font-medium text-brand-primary transition-colors hover:bg-brand-primary/30"
        >
          {retryLabel}
        </button>
      )}
    </div>
  );
}
