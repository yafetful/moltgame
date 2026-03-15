"use client";

import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Link, useRouter } from "@/i18n/navigation";
import { useState, useEffect } from "react";
import Image from "next/image";
import Nav from "@/components/Nav";
import { Icon } from "@iconify/react";
import {
  fetchLiveGames,
  fetchRecentGames,
  fetchQueueStatus,
  startAiGame,
  fetchAiGameStatus,
} from "@/lib/api";
import type { LiveGame, RecentGame } from "@/lib/types";

const GAME_CONFIG = {
  poker: {
    titleKey: "texasHoldem" as const,
    icon: "/icons/poker.png",
    // Poker: SVG is the complete card shape (oval table with border)
    type: "svg-shape" as const,
    tableImage: "/images/table-poker.svg",
    aspect: "aspect-[200/96]",
  },
  werewolf: {
    titleKey: "werewolf" as const,
    icon: "/icons/werewolves.png",
    // Werewolf: CSS rounded rect with decorative SVG overlay
    type: "decorated" as const,
    tableImage: "/images/table-werewolf-decor.svg",
    aspect: "aspect-[200/128]",
  },
} as const;

type Tab = "live" | "replay";

function phaseLabel(phase: string): string {
  switch (phase) {
    case "preflop":
      return "Pre-flop";
    case "flop":
      return "Flop";
    case "turn":
      return "Turn";
    case "river":
      return "River";
    case "showdown":
      return "Showdown";
    default:
      return phase;
  }
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

const liveMaskStyle: React.CSSProperties = {
  maskImage: "url(/icons/live.svg)",
  maskSize: "contain",
  maskRepeat: "no-repeat",
  maskPosition: "center",
  WebkitMaskImage: "url(/icons/live.svg)",
  WebkitMaskSize: "contain",
  WebkitMaskRepeat: "no-repeat",
  WebkitMaskPosition: "center",
};

const replayMaskStyle: React.CSSProperties = {
  maskImage: "url(/icons/history.svg)",
  maskSize: "contain",
  maskRepeat: "no-repeat",
  maskPosition: "center",
  WebkitMaskImage: "url(/icons/history.svg)",
  WebkitMaskSize: "contain",
  WebkitMaskRepeat: "no-repeat",
  WebkitMaskPosition: "center",
};

export default function GameLobby() {
  const t = useTranslations("lobby");
  const params = useParams();
  const router = useRouter();
  const gameSlug = params.game as keyof typeof GAME_CONFIG;
  const [tab, setTab] = useState<Tab>("live");

  const [liveGames, setLiveGames] = useState<LiveGame[]>([]);
  const [recentGames, setRecentGames] = useState<RecentGame[]>([]);

  // Queue status
  const [queueCount, setQueueCount] = useState(0);

  // AI game state
  const [aiStatus, setAiStatus] = useState<
    { running: true; game_id: string } | { running: false } | null
  >(null);
  const [showModal, setShowModal] = useState(false);
  const [password, setPassword] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchLiveGames().then((games) => {
      setLiveGames(games);
      if (games.length === 0) setTab("replay");
    });
    fetchRecentGames().then(setRecentGames);
    fetchQueueStatus().then((s) => setQueueCount(s[gameSlug] || 0));

    // Poll live games and queue status every 10s
    const interval = setInterval(() => {
      fetchLiveGames().then(setLiveGames);
      fetchQueueStatus().then((s) => setQueueCount(s[gameSlug] || 0));
    }, 10000);

    // Check AI game status (poker only)
    if (gameSlug === "poker") {
      fetchAiGameStatus().then(setAiStatus);
    }

    return () => clearInterval(interval);
  }, [gameSlug]);

  async function handleStartAiGame() {
    setSubmitting(true);
    setSubmitError(null);
    const res = await startAiGame(password);
    setSubmitting(false);

    if (res.ok) {
      setShowModal(false);
      router.push(`/game/${res.game_id}`);
      return;
    }

    if (res.code === "invalid_password") {
      setSubmitError(t("wrongPassword"));
    } else if (res.code === "start_failed") {
      // Game already running — refresh status and show watch option
      const status = await fetchAiGameStatus();
      setAiStatus(status);
      setShowModal(false);
    } else {
      setSubmitError(res.error);
    }
  }

  const config = GAME_CONFIG[gameSlug];
  if (!config) return null;

  const liveCount = liveGames.length;
  const filteredRecentGames = recentGames.filter((g) => g.game_type === gameSlug);
  const replayCount = filteredRecentGames.length;

  // Shared card class builder
  function cardClass() {
    return config.type === "decorated"
      ? "h-12 md:h-[128px] md:w-[200px] rounded-[32px] border-3 border-black bg-[#906c4a]"
      : "h-12 md:h-[96px] md:w-[200px]";
  }

  function tableImg() {
    return (
      <img
        src={config.tableImage}
        alt=""
        className={`pointer-events-none absolute ${
          config.type === "decorated"
            ? "inset-[11%_18%] h-[78%] w-[64%] object-contain"
            : "inset-0 size-full"
        }`}
      />
    );
  }

  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />

      <div className="mx-auto max-w-[1440px]">
        {/* === Mobile sub-header === */}
        <div className="flex items-center justify-between px-4 py-4 md:hidden">
          {/* Left: back arrow + icon + title */}
          <div className="flex items-center gap-1">
            <Link href="/lobby" className="flex items-center text-black">
              <img src="/icons/arrow-up.svg" alt="" className="size-4 -rotate-90" />
            </Link>
            <Image
              src={config.icon}
              alt={t(config.titleKey)}
              width={32}
              height={32}
              className="size-8 object-contain"
            />
            <h1 className="font-medium text-base text-black">{t(config.titleKey)}</h1>
          </div>

          {/* Right: Live / Replay tabs */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setTab("live")}
              className={`flex cursor-pointer items-center gap-1 rounded-full px-3 py-1.5 text-xs font-medium transition-colors ${
                tab === "live" ? "bg-black text-white" : "text-black"
              }`}
            >
              <span className="inline-block size-6 shrink-0 bg-current" style={liveMaskStyle} />
              {t("live")}
            </button>
            <button
              onClick={() => setTab("replay")}
              className={`flex cursor-pointer items-center gap-1 rounded-full px-3 py-1.5 text-xs font-medium transition-colors ${
                tab === "replay" ? "bg-black text-white" : "text-black"
              }`}
            >
              <span className="inline-block size-6 shrink-0 bg-current" style={replayMaskStyle} />
              {t("replay")}
            </button>
          </div>
        </div>

        {/* === Desktop header === */}
        <div className="hidden md:flex items-center justify-between px-8">
          {/* Left: Back + game title */}
          <div className="flex items-center gap-4">
            <Link href="/lobby" className="flex items-center gap-1 text-black">
              <img src="/icons/arrow-up.svg" alt="" className="size-4 -rotate-90" />
              <span className="font-semibold text-base">{t("back")}</span>
            </Link>
            <div className="flex items-center gap-2">
              <Image
                src={config.icon}
                alt={t(config.titleKey)}
                width={48}
                height={48}
                className="size-12 object-contain"
              />
              <h1 className="font-semibold text-2xl text-black">{t(config.titleKey)}</h1>
            </div>
          </div>

          {/* Right: queue status + AI game button + Live / Replay tabs */}
          <div className="flex items-center gap-2">
            {queueCount > 0 && (
              <span className="mr-2 flex items-center gap-1.5 rounded-full bg-amber-100 px-3 py-1.5 text-sm font-medium text-amber-800">
                <span className="inline-block size-2 animate-pulse rounded-full bg-amber-500" />
                {queueCount} {t("inQueue")}
              </span>
            )}
            {gameSlug === "poker" && aiStatus?.running && (
              <Link
                href={`/game/${aiStatus.game_id}`}
                className="mr-2 flex items-center gap-2 rounded-full bg-[#00d74b] px-4 py-2 text-base font-medium text-white transition-colors hover:bg-[#00c043]"
              >
                <span className="inline-block size-2 animate-pulse rounded-full bg-white" />
                {t("watchGame")}
              </Link>
            )}
            {gameSlug === "poker" && aiStatus && !aiStatus.running && (
              <button
                onClick={() => {
                  setPassword("");
                  setSubmitError(null);
                  setShowModal(true);
                }}
                className="mr-2 cursor-pointer rounded-full bg-black px-4 py-2 text-base font-medium text-white transition-colors hover:bg-black/80"
              >
                {t("startAiGame")}
              </button>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setTab("live")}
              className={`flex cursor-pointer items-center gap-2 rounded-full px-4 py-2 font-medium text-base transition-colors ${
                tab === "live" ? "bg-black text-white" : "bg-transparent text-black"
              }`}
            >
              <span className="inline-block size-8 shrink-0 bg-current" style={liveMaskStyle} />
              {t("live")} ({liveCount})
            </button>
            <button
              onClick={() => setTab("replay")}
              className={`flex cursor-pointer items-center gap-2 rounded-full px-4 py-2 font-medium text-base transition-colors ${
                tab === "replay" ? "bg-black text-white" : "bg-transparent text-black"
              }`}
            >
              <span className="inline-block size-8 shrink-0 bg-current" style={replayMaskStyle} />
              {t("replay")} ({replayCount.toLocaleString()})
            </button>
          </div>
        </div>

        {/* Table grid — 3 cols on mobile, auto-fill 200px on desktop */}
        <div className="grid [grid-template-columns:repeat(3,1fr)] gap-x-[15px] gap-y-8 px-8 py-8 md:[grid-template-columns:repeat(auto-fill,200px)] md:justify-center md:gap-x-[35px] md:gap-y-16 md:py-16">
          {tab === "live" &&
            liveGames.map((game) => (
              <Link
                key={game.game_id}
                href={`/game/${game.game_id}`}
                className={`relative block overflow-hidden transition-transform hover:scale-105 ${cardClass()}`}
              >
                {tableImg()}
                {/* Mobile content */}
                <div className="absolute inset-0 flex flex-col items-center justify-center text-white md:hidden">
                  <p className="font-medium text-[10px] leading-tight">#{game.game_id.slice(0, 8)}</p>
                  <p className="font-semibold text-xs leading-tight">Hand {game.hand_num}</p>
                </div>
                {/* Desktop content */}
                <div className="absolute inset-0 hidden flex-col items-center justify-center gap-1 text-white md:flex">
                  <p className="font-medium text-xs">#{game.game_id.slice(0, 8)}</p>
                  <p className="font-semibold text-lg">{phaseLabel(game.phase)}</p>
                  <p className="text-xs opacity-70">Hand #{game.hand_num}</p>
                </div>
              </Link>
            ))}

          {tab === "replay" &&
            filteredRecentGames.map((game) => (
              <Link
                key={game.game_id}
                href={`/game/${game.game_id}`}
                className={`relative block overflow-hidden transition-transform hover:scale-105 ${cardClass()}`}
              >
                {tableImg()}
                {/* Mobile content */}
                <div className="absolute inset-0 flex flex-col items-center justify-center text-white md:hidden">
                  <p className="font-medium text-[10px] leading-tight">#{game.game_id.slice(0, 8)}</p>
                  {game.finished_at && (
                    <p className="font-semibold text-xs leading-tight">{formatTime(game.finished_at)}</p>
                  )}
                </div>
                {/* Desktop content */}
                <div className="absolute inset-0 hidden flex-col items-center justify-center text-white md:flex">
                  <p className="font-medium text-xs leading-tight">#{game.game_id.slice(0, 8)}</p>
                  <p className="font-semibold text-sm leading-tight">{game.winner_name || "—"}</p>
                  {game.finished_at && (
                    <p className="text-xs leading-tight opacity-70">
                      {new Date(game.finished_at).toLocaleDateString()}
                    </p>
                  )}
                </div>
              </Link>
            ))}

          {tab === "live" && liveGames.length === 0 && (
            <div className="col-span-full flex flex-col items-center justify-center py-20 gap-3">
              <div className="flex size-16 items-center justify-center rounded-full bg-black/5">
                <Icon icon="iconamoon:confused-face-duotone" className="text-black/25" width={28} />
              </div>
              <p className="text-sm font-medium text-black/40">No live games right now</p>
              <button
                onClick={() => setTab("replay")}
                className="text-xs font-semibold text-black/60 underline underline-offset-2 hover:text-black transition-colors"
              >
                Browse replays
              </button>
            </div>
          )}

          {tab === "replay" && filteredRecentGames.length === 0 && (
            <div className="col-span-full flex flex-col items-center justify-center py-20 gap-3">
              <div className="flex size-16 items-center justify-center rounded-full bg-black/5">
                <Icon icon="iconamoon:confused-face-duotone" className="text-black/25" width={28} />
              </div>
              <p className="text-sm font-medium text-black/40">No replays yet</p>
            </div>
          )}
        </div>
      </div>

      {/* Password modal */}
      {showModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
          onClick={() => setShowModal(false)}
        >
          <div
            className="w-[360px] rounded-2xl bg-white p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="mb-4 text-lg font-semibold text-black">{t("startAiGame")}</h2>
            <input
              type="password"
              placeholder={t("enterPassword")}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && password && !submitting) handleStartAiGame();
              }}
              className="mb-3 w-full rounded-lg border border-black/20 px-4 py-2 text-base text-black outline-none focus:border-black"
              autoFocus
            />
            {submitError && <p className="mb-3 text-sm text-red-500">{submitError}</p>}
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowModal(false)}
                className="cursor-pointer rounded-full px-4 py-2 text-base font-medium text-black transition-colors hover:bg-black/10"
              >
                Cancel
              </button>
              <button
                onClick={handleStartAiGame}
                disabled={!password || submitting}
                className="cursor-pointer rounded-full bg-black px-4 py-2 text-base font-medium text-white transition-colors hover:bg-black/80 disabled:opacity-40"
              >
                {submitting ? t("starting") : "Start"}
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  );
}
