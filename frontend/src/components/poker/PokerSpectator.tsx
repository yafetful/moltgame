"use client";

import { useTranslations } from "next-intl";
import { motion, AnimatePresence } from "framer-motion";
import type { GameState, PlayerState } from "@/lib/api";
import { cn } from "@/lib/utils";
import { ChipsIcon, DealerIcon } from "@/components/icons";
import Image from "next/image";

// Seat positions around an elliptical table (for 6 players)
const SEAT_POSITIONS = [
  { x: 50, y: 90 },  // seat 0 (bottom center)
  { x: 10, y: 65 },  // seat 1 (bottom left)
  { x: 10, y: 25 },  // seat 2 (top left)
  { x: 50, y: 5 },   // seat 3 (top center)
  { x: 90, y: 25 },  // seat 4 (top right)
  { x: 90, y: 65 },  // seat 5 (bottom right)
];

// Blind levels (matches backend)
const BLIND_LEVELS = [
  { sb: 10, bb: 20 },
  { sb: 15, bb: 30 },
  { sb: 25, bb: 50 },
  { sb: 50, bb: 100 },
  { sb: 75, bb: 150 },
  { sb: 100, bb: 200 },
  { sb: 150, bb: 300 },
  { sb: 200, bb: 400 },
];

function getBlindLevel(handNum: number): { sb: number; bb: number; level: number } {
  const level = Math.min(Math.floor((handNum - 1) / 10), BLIND_LEVELS.length - 1);
  return { ...BLIND_LEVELS[Math.max(0, level)], level: level + 1 };
}

// Map card notation to SVG file paths
const SUIT_MAP: Record<string, string> = {
  s: "spades", h: "hearts", d: "diamonds", c: "clubs",
  S: "spades", H: "hearts", D: "diamonds", C: "clubs",
};

const RANK_MAP: Record<string, string> = {
  A: "1", "2": "2", "3": "3", "4": "4", "5": "5",
  "6": "6", "7": "7", "8": "8", "9": "9", T: "10",
  J: "11", Q: "12", K: "13",
};

function getCardSVGPath(card: string): string {
  const rank = card.slice(0, -1);
  const suit = card.slice(-1);
  const suitDir = SUIT_MAP[suit] || "spades";
  const rankFile = RANK_MAP[rank] || rank;
  return `/poker/${suitDir}/${rankFile}.svg`;
}

export function PokerSpectator({ state }: { state: GameState }) {
  const t = useTranslations("poker");
  const players = state.players || [];
  const community = state.community || [];
  const totalPot = state.pots?.reduce((s, p) => s + p.amount, 0) ?? 0;
  const handNum = state.hand_num || 0;
  const blinds = getBlindLevel(handNum);
  const isShowdown = state.phase === "showdown" || state.phase === "finished";

  return (
    <div className="relative h-full w-full">
      {/* Header info */}
      <div className="absolute inset-x-0 top-0 z-10 flex h-10 items-center justify-between px-4">
        <div className="flex items-center gap-4 text-sm text-white/50">
          <span>
            {t("hand")} #{handNum}
          </span>
          <PhaseTag phase={state.phase || ""} />
          <span className="flex items-center gap-1 rounded-full bg-white/10 px-2 py-0.5 text-white/40">
            <ChipsIcon size={12} />
            {t("blinds")} {blinds.sb}/{blinds.bb}
          </span>
          {totalPot > 0 && (
            <span className="flex items-center gap-1">
              {t("pot")}:{" "}
              <motion.strong
                key={totalPot}
                initial={{ scale: 1.3, color: "#F5A623" }}
                animate={{ scale: 1, color: "#ffffff" }}
                transition={{ duration: 0.3 }}
                className="inline-block"
              >
                {totalPot}
              </motion.strong>
            </span>
          )}
        </div>
        <span className="text-xs text-white/30">{t("godView")}</span>
      </div>

      {/* Table */}
      <div className="absolute inset-x-0 top-10 bottom-0">
        <div className="relative mx-auto h-full w-full max-w-4xl">
          {/* Table surface — now uses the poker table image */}
          <div className="absolute inset-[12%] overflow-hidden rounded-[50%]">
            <Image
              src="/poker/table.png"
              alt="Poker table"
              fill
              className="object-cover"
              priority
            />
            <div className="absolute inset-0 rounded-[50%] shadow-[inset_0_0_60px_rgba(0,0,0,0.5)] border-4 border-emerald-800/40" />
          </div>

          {/* Community cards */}
          <div className="absolute left-1/2 top-1/2 flex -translate-x-1/2 -translate-y-1/2 gap-2">
            {community.length > 0
              ? community.map((card, i) => (
                  <motion.div
                    key={`${card}-${i}`}
                    initial={{ rotateY: 90, opacity: 0 }}
                    animate={{ rotateY: 0, opacity: 1 }}
                    transition={{ duration: 0.4, delay: i * 0.1 }}
                  >
                    <CardDisplay card={card} />
                  </motion.div>
                ))
              : Array.from({ length: 5 }, (_, i) => (
                  <div key={i} className="h-16 w-11 rounded-md border border-white/10 bg-white/5" />
                ))}
          </div>

          {/* Pot display */}
          <AnimatePresence>
            {totalPot > 0 && (
              <motion.div
                initial={{ scale: 0, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                exit={{ scale: 0, opacity: 0 }}
                className="absolute left-1/2 top-[62%] -translate-x-1/2 flex items-center gap-1 rounded-full bg-brand-accent/20 px-3 py-1 text-xs font-bold text-brand-accent"
              >
                <ChipsIcon size={14} />
                {totalPot}
              </motion.div>
            )}
          </AnimatePresence>

          {/* Player seats */}
          {players.map((player, idx) => {
            const pos = SEAT_POSITIONS[idx] || SEAT_POSITIONS[0];
            return (
              <motion.div
                key={player.id}
                className="absolute -translate-x-1/2 -translate-y-1/2"
                style={{ left: `${pos.x}%`, top: `${pos.y}%` }}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: idx * 0.05 }}
              >
                <PlayerSeat
                  player={player}
                  isActive={state.action_on === player.seat}
                  isShowdown={isShowdown}
                />
              </motion.div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function PhaseTag({ phase }: { phase: string }) {
  const t = useTranslations("poker");
  const phaseLabels: Record<string, string> = {
    preflop: t("preflop"),
    flop: t("flop"),
    turn: t("turn"),
    river: t("river"),
    showdown: t("showdown"),
  };

  return (
    <motion.span
      key={phase}
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      className="rounded-full bg-brand-poker/20 px-2 py-0.5 text-brand-poker"
    >
      {phaseLabels[phase] || phase}
    </motion.span>
  );
}

function PlayerSeat({
  player,
  isActive,
  isShowdown,
}: {
  player: PlayerState;
  isActive: boolean;
  isShowdown: boolean;
}) {
  const cards = player.hole || [];
  const isFolded = player.folded;

  return (
    <motion.div
      animate={{
        opacity: isFolded ? 0.4 : 1,
        scale: isActive ? 1.05 : 1,
      }}
      transition={{ duration: 0.2 }}
      className={cn(
        "flex flex-col items-center gap-1 rounded-xl px-3 py-2",
        isActive && "ring-2 ring-brand-primary bg-brand-primary/10",
      )}
    >
      {/* Active indicator (countdown bar) */}
      {isActive && (
        <motion.div
          className="absolute -top-1 left-1 right-1 h-0.5 rounded-full bg-brand-primary"
          initial={{ scaleX: 1 }}
          animate={{ scaleX: 0 }}
          transition={{ duration: 30, ease: "linear" }}
          style={{ transformOrigin: "left" }}
        />
      )}

      {/* Hole cards */}
      <div className="flex gap-0.5">
        <AnimatePresence mode="wait">
          {cards.length > 0
            ? cards.map((c, i) => (
                <motion.div
                  key={`${c}-${i}`}
                  initial={{ rotateY: 90, opacity: 0 }}
                  animate={{ rotateY: 0, opacity: 1 }}
                  transition={{ duration: 0.3, delay: i * 0.1 }}
                >
                  <CardDisplay card={c} small />
                </motion.div>
              ))
            : [0, 1].map((i) => (
                <Image
                  key={i}
                  src="/poker/back.svg"
                  alt="Card back"
                  width={22}
                  height={32}
                  className="h-8 w-[22px] rounded-sm"
                />
              ))}
        </AnimatePresence>
      </div>

      {/* Name + chips */}
      <div className="text-center">
        <div className="text-xs font-medium truncate max-w-[80px]">
          {player.id.slice(0, 8)}
        </div>
        <div className="flex items-center justify-center gap-0.5 text-[10px] text-brand-accent font-mono">
          <ChipsIcon size={10} />
          {player.chips ?? 0}
        </div>
      </div>

      {/* Bet indicator */}
      <AnimatePresence>
        {player.bet !== undefined && player.bet > 0 && (
          <motion.div
            initial={{ scale: 0, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0, opacity: 0 }}
            className="flex items-center gap-0.5 rounded-full bg-brand-accent/20 px-1.5 py-0.5 text-[9px] font-bold text-brand-accent"
          >
            {player.bet}
          </motion.div>
        )}
      </AnimatePresence>

      {/* All-in badge */}
      <AnimatePresence>
        {player.all_in && (
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            className="rounded bg-brand-danger/20 px-1.5 py-0.5 text-[9px] font-bold text-brand-danger"
          >
            ALL IN
          </motion.div>
        )}
      </AnimatePresence>

      {/* Winner highlight */}
      {isShowdown && !isFolded && player.chips !== undefined && player.chips > 0 && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: [0, 1, 0.5, 1] }}
          transition={{ duration: 1, repeat: 2 }}
          className="absolute inset-0 rounded-xl ring-2 ring-brand-accent/50 pointer-events-none"
        />
      )}
    </motion.div>
  );
}

function CardDisplay({ card, small }: { card: string; small?: boolean }) {
  const svgPath = getCardSVGPath(card);

  return (
    <Image
      src={svgPath}
      alt={card}
      width={small ? 22 : 44}
      height={small ? 32 : 64}
      className={cn(
        "rounded-md shadow-sm",
        small ? "h-8 w-[22px]" : "h-16 w-11",
      )}
    />
  );
}
