"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import type { GameState, PlayerState, Speech } from "@/lib/api";
import { cn } from "@/lib/utils";
import { EliminatedIcon } from "@/components/icons";
import Image from "next/image";

const ROLE_IMAGES: Record<string, string> = {
  werewolf: "/werewolf/role/Werewolf.png",
  seer: "/werewolf/role/Seer.png",
  villager: "/werewolf/role/Villager.png",
  guard: "/werewolf/role/Guard.png",
};

export function WerewolfSpectator({ state }: { state: GameState }) {
  const t = useTranslations("werewolf");
  const [godView, setGodView] = useState(false);

  const players = state.players || [];
  const speeches = state.speeches || [];
  const isNight = state.phase === "night" || state.phase === "wolf_kill" || state.phase === "seer_peek";
  const isVote = state.phase === "vote";
  const isGameOver = state.phase === "game_over" || state.phase === "finished";
  const isDiscussion = state.phase === "discussion" || state.phase === "day";

  return (
    <div className="relative flex flex-col gap-6">
      {/* Scene background — cross-fades between day/night */}
      <div className="pointer-events-none absolute inset-0 -z-10 overflow-hidden rounded-xl">
        <AnimatePresence mode="wait">
          <motion.div
            key={isNight ? "night" : "day"}
            initial={{ opacity: 0 }}
            animate={{ opacity: 0.2 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 1 }}
            className="absolute inset-0"
          >
            <Image
              src={isNight ? "/werewolf/scene/night-bg.png" : "/werewolf/scene/day-bg.png"}
              alt=""
              fill
              className="object-cover"
            />
          </motion.div>
        </AnimatePresence>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4 text-sm text-white/50">
          {state.day !== undefined && (
            <span>
              {isNight ? t("night") : t("day")} {state.day}
            </span>
          )}
          <AnimatePresence mode="wait">
            <motion.span
              key={state.phase}
              initial={{ opacity: 0, y: -4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 4 }}
              className={cn(
                "rounded-full px-2 py-0.5",
                isNight && "bg-brand-werewolf/20 text-violet-400",
                isVote && "bg-brand-accent/20 text-brand-accent",
                isGameOver && "bg-brand-danger/20 text-brand-danger",
                isDiscussion && "bg-brand-poker/20 text-brand-poker",
                !isNight && !isVote && !isGameOver && !isDiscussion && "bg-white/10 text-white/50",
              )}
            >
              {state.phase}
            </motion.span>
          </AnimatePresence>
        </div>
        <button
          onClick={() => setGodView((v) => !v)}
          className={cn(
            "rounded-full px-3 py-1 text-xs font-medium transition-colors",
            godView ? "bg-brand-accent/20 text-brand-accent" : "bg-white/10 text-white/40 hover:text-white/70",
          )}
        >
          {godView ? t("godView") : t("suspenseView")}
        </button>
      </div>

      <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
        {/* Player panel */}
        <div className="flex flex-col gap-2">
          {players.map((player, i) => (
            <motion.div
              key={player.id}
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: i * 0.05 }}
            >
              <PlayerCard player={player} godView={godView} />
            </motion.div>
          ))}
        </div>

        {/* Main area */}
        <div className="flex flex-col gap-4">
          {/* Night overlay */}
          <AnimatePresence>
            {isNight && (
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="flex items-center justify-center rounded-xl bg-indigo-950/40 border border-brand-werewolf/20 py-12"
              >
                <motion.div
                  initial={{ scale: 0.8 }}
                  animate={{ scale: 1 }}
                  className="text-center"
                >
                  <motion.div
                    animate={{ rotate: [0, -10, 10, 0] }}
                    transition={{ duration: 3, repeat: Infinity }}
                    className="mb-2 inline-block"
                  >
                    <Image src="/werewolf/scene/night-bg.png" alt="" width={64} height={64} className="rounded-full opacity-60" />
                  </motion.div>
                  <p className="text-lg font-semibold text-indigo-300">{t("nightFalls")}</p>
                </motion.div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Vote phase */}
          <AnimatePresence>
            {isVote && (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                className="rounded-xl bg-amber-950/20 border border-brand-accent/20 px-6 py-4"
              >
                <p className="mb-3 text-sm font-semibold text-brand-accent">{t("voteTime")}</p>
                <VotePanel players={players} godView={godView} />
              </motion.div>
            )}
          </AnimatePresence>

          {/* Game over */}
          <AnimatePresence>
            {isGameOver && (
              <motion.div
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                <GameOverBanner />
              </motion.div>
            )}
          </AnimatePresence>

          {/* Speech timeline */}
          <div className="flex flex-col gap-3">
            {speeches.length === 0 && !isNight && (
              <p className="py-8 text-center text-sm text-white/20">—</p>
            )}
            <AnimatePresence initial={false}>
              {speeches.map((speech, i) => (
                <motion.div
                  key={`${speech.player_id}-${speech.order}`}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: i * 0.05 }}
                >
                  <SpeechBubble
                    speech={speech}
                    players={players}
                    godView={godView}
                    isLatest={i === speeches.length - 1 && isDiscussion}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </div>
      </div>
    </div>
  );
}

function PlayerCard({ player, godView }: { player: PlayerState; godView: boolean }) {
  const t = useTranslations("werewolf");
  const isAlive = player.alive !== false;
  const roleImage = godView && player.role ? ROLE_IMAGES[player.role] : null;

  return (
    <motion.div
      animate={{ opacity: isAlive ? 1 : 0.5 }}
      className={cn(
        "flex items-center gap-3 rounded-lg border px-3 py-2 transition-colors",
        isAlive ? "border-white/10 bg-white/5" : "border-white/5 bg-white/[0.02]",
      )}
    >
      <div className="relative flex h-8 w-8 items-center justify-center rounded-full bg-white/10 overflow-hidden">
        {roleImage ? (
          <Image src={roleImage} alt={player.role || ""} width={32} height={32} className="object-cover" />
        ) : (
          <Image src="/werewolf/role/back.png" alt="" width={32} height={32} className="object-cover opacity-60" />
        )}
        {!isAlive && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/50">
            <EliminatedIcon size={16} />
          </div>
        )}
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium">{player.id.slice(0, 10)}</span>
          <AnimatePresence>
            {godView && player.role && (
              <motion.span
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.8 }}
                className={cn(
                  "rounded-full px-1.5 py-0.5 text-[10px] font-bold",
                  player.role === "werewolf" && "bg-brand-danger/20 text-brand-danger",
                  player.role === "seer" && "bg-purple-500/20 text-purple-400",
                  player.role === "villager" && "bg-blue-500/20 text-blue-300",
                )}
              >
                {t(player.role)}
              </motion.span>
            )}
          </AnimatePresence>
        </div>
        <div className="text-[10px] text-white/40">
          {isAlive ? (
            <span className="text-brand-poker">{t("alive")}</span>
          ) : (
            <span className="text-brand-danger">
              {t("dead")}
              {player.death_cause === "killed" && ` · ${t("killed")}`}
              {player.death_cause === "executed" && ` · ${t("executed")}`}
            </span>
          )}
        </div>
      </div>

      <span className="text-xs text-white/20">#{player.seat}</span>
    </motion.div>
  );
}

function VotePanel({ players, godView }: { players: PlayerState[]; godView: boolean }) {
  const alivePlayers = players.filter((p) => p.alive !== false);

  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
      {alivePlayers.map((player) => {
        const roleImage = godView && player.role ? ROLE_IMAGES[player.role] : null;
        return (
          <div
            key={player.id}
            className="flex items-center gap-2 rounded-md bg-white/5 px-3 py-2"
          >
            <div className="flex h-6 w-6 items-center justify-center rounded-full bg-white/10 overflow-hidden">
              {roleImage ? (
                <Image src={roleImage} alt="" width={24} height={24} className="object-cover" />
              ) : (
                <span className="text-xs font-bold">{player.seat}</span>
              )}
            </div>
            <div className="flex-1 min-w-0">
              <span className="text-xs truncate block">{player.id.slice(0, 8)}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function SpeechBubble({
  speech,
  players,
  godView,
  isLatest,
}: {
  speech: Speech;
  players: PlayerState[];
  godView: boolean;
  isLatest: boolean;
}) {
  const player = players.find((p) => p.id === speech.player_id);
  const role = godView && player?.role ? player.role : null;
  const roleImage = role ? ROLE_IMAGES[role] : null;

  return (
    <div className={cn("flex gap-3", isLatest && "relative")}>
      {isLatest && (
        <motion.div
          className="absolute -left-2 top-0 bottom-0 w-0.5 rounded-full bg-brand-primary"
          initial={{ scaleY: 0 }}
          animate={{ scaleY: 1 }}
          style={{ transformOrigin: "top" }}
        />
      )}

      <div
        className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center rounded-full overflow-hidden",
          role === "werewolf" && "bg-brand-danger/20",
          role === "seer" && "bg-purple-500/20",
          role === "villager" && "bg-blue-500/20",
          !role && "bg-white/10",
        )}
      >
        {roleImage ? (
          <Image src={roleImage} alt="" width={32} height={32} className="object-cover" />
        ) : (
          <span className="text-xs font-bold text-white/40">{speech.seat}</span>
        )}
      </div>
      <div className={cn(
        "flex-1 rounded-lg border px-4 py-2.5",
        isLatest ? "bg-white/[0.07] border-brand-primary/20" : "bg-white/5 border-white/10",
      )}>
        <div className="mb-1 flex items-center gap-2">
          <span className="text-xs font-semibold text-white/70">
            {speech.player_id.slice(0, 10)}
          </span>
          {role && (
            <span className="text-[10px] text-white/30">
              {role}
            </span>
          )}
        </div>
        <p className="text-sm text-white/80 leading-relaxed">{speech.message}</p>
      </div>
    </div>
  );
}

function GameOverBanner() {
  const tc = useTranslations("common");

  return (
    <div className="rounded-xl bg-gradient-to-r from-brand-danger/10 to-brand-werewolf/10 border border-brand-danger/20 px-6 py-8 text-center">
      <motion.div
        initial={{ scale: 0 }}
        animate={{ scale: 1 }}
        transition={{ type: "spring", damping: 10 }}
        className="mb-2"
      >
        <Image src="/werewolf/role/Werewolf.png" alt="" width={48} height={48} className="mx-auto" />
      </motion.div>
      <p className="text-lg font-bold">{tc("gameOver")}</p>
    </div>
  );
}
