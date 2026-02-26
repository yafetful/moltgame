import Image from "next/image";
import PokerCard, { type Suit } from "./PokerCard";

export type Stage = "starting" | "preflop" | "flop" | "turn" | "river";
export type ActionType =
  | "bet"
  | "check"
  | "call"
  | "allIn"
  | "fold"
  | "raise";

export interface CommunityCard {
  suit: Suit;
  value: number;
  faceDown?: boolean;
}

export interface BetPosition {
  seatIndex: number;
  amount: number;
  chipIcon?: string;
  action?: ActionType;
}

export interface PokerTableProps {
  stage: Stage;
  stageLabel: string;
  handNumber?: string;
  countdown?: number;
  communityCards: CommunityCard[];
  pot: number;
  bets: BetPosition[];
}

const ACTION_COLOR: Record<ActionType, string> = {
  bet: "bg-[#22c55e]",
  check: "bg-[#6b21a8]",
  call: "bg-[#3b82f6]",
  allIn: "bg-[#ef4444]",
  fold: "bg-[#eab308]",
  raise: "bg-[#f97316]",
};

const ACTION_LABEL: Record<ActionType, string> = {
  bet: "Bet",
  check: "Check",
  call: "Call",
  allIn: "All-in",
  fold: "Fold",
  raise: "Raise",
};

/** Bet chip positions relative to the 720x360 table */
const BET_POSITIONS: { left: number; top: number; seatIndex: number }[] = [
  { left: 327, top: 27, seatIndex: 0 },
  { left: 576, top: 90, seatIndex: 1 },
  { left: 576, top: 224, seatIndex: 2 },
  { left: 327, top: 283, seatIndex: 3 },
  { left: 77, top: 224, seatIndex: 4 },
  { left: 77, top: 90, seatIndex: 5 },
];

export default function PokerTable({
  stage,
  stageLabel,
  handNumber,
  countdown,
  communityCards,
  pot,
  bets,
}: PokerTableProps) {
  const showPot = stage !== "starting" && stage !== "preflop";

  return (
    <div className="relative h-[360px] w-[720px]">
      {/* Table background */}
      <Image
        src="/poker/table.png"
        alt="poker table"
        width={960}
        height={480}
        className="absolute inset-0 size-full object-contain"
        priority
      />

      {/* Center area */}
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-2">
        {/* Stage label or hand number */}
        {stage === "starting" ? (
          <div className="flex flex-col items-center">
            <span className="text-lg font-bold text-white/80">
              {handNumber}
            </span>
            {countdown !== undefined && (
              <span className="text-sm text-white/60">{countdown}s</span>
            )}
          </div>
        ) : (
          <>
            <span className="text-sm font-semibold text-white/70">
              {stageLabel}
            </span>
            {/* Community cards */}
            <div className="flex gap-2">
              {communityCards.map((card, i) => (
                <PokerCard
                  key={i}
                  suit={card.suit}
                  value={card.value}
                  faceDown={card.faceDown}
                  size="md"
                />
              ))}
            </div>
            {/* Pot */}
            {showPot && (
              <div className="flex items-center gap-1 rounded-full bg-black/40 px-3 py-1">
                <span className="text-xs font-semibold text-white">
                  ${pot.toLocaleString()}
                </span>
              </div>
            )}
          </>
        )}
      </div>

      {/* Bet positions */}
      {bets.map((bet) => {
        const pos = BET_POSITIONS.find((p) => p.seatIndex === bet.seatIndex);
        if (!pos) return null;
        return (
          <div
            key={bet.seatIndex}
            className="absolute flex flex-col items-center gap-0.5"
            style={{ left: pos.left, top: pos.top }}
          >
            <div className="flex items-center gap-1">
              {bet.chipIcon && (
                <Image
                  src={bet.chipIcon}
                  alt=""
                  width={16}
                  height={16}
                  className="size-4"
                />
              )}
              <span className="text-xs font-semibold text-white">
                ${bet.amount.toLocaleString()}
              </span>
            </div>
            {bet.action && (
              <span
                className={`rounded-sm px-1.5 py-0.5 text-[10px] font-bold text-white ${ACTION_COLOR[bet.action]}`}
              >
                {ACTION_LABEL[bet.action]}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
