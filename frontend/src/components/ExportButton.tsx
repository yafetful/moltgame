"use client";

import { useTranslations } from "next-intl";
import type { ExportProgress } from "@/hooks/useVideoExporter";
import { cn } from "@/lib/utils";

interface ExportButtonProps {
  progress: ExportProgress;
  onExport: () => void;
  onCancel: () => void;
  disabled?: boolean;
}

export function ExportButton({ progress, onExport, onCancel, disabled }: ExportButtonProps) {
  const t = useTranslations("common");

  if (progress.phase === "idle" || progress.phase === "error") {
    return (
      <button
        onClick={onExport}
        disabled={disabled}
        className={cn(
          "rounded-lg px-3 py-1.5 text-xs font-medium transition-colors",
          disabled
            ? "cursor-not-allowed bg-white/5 text-white/20"
            : "bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30",
        )}
      >
        {t("exportVideo")}
      </button>
    );
  }

  if (progress.phase === "done") {
    return (
      <span className="rounded-lg bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-400">
        {t("exported")}
      </span>
    );
  }

  // Capturing
  return (
    <div className="flex items-center gap-2">
      <div className="flex items-center gap-2">
        <div className="h-1.5 w-24 overflow-hidden rounded-full bg-white/10">
          <div
            className="h-full rounded-full bg-emerald-500 transition-all duration-300"
            style={{ width: `${progress.percent}%` }}
          />
        </div>
        <span className="text-[10px] text-white/40">
          {progress.percent}%
        </span>
      </div>
      <button
        onClick={onCancel}
        className="rounded px-2 py-0.5 text-[10px] text-red-400 hover:bg-red-500/10"
      >
        {t("cancel")}
      </button>
    </div>
  );
}
