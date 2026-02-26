import Image from "next/image";
import PokerCard, { type Suit } from "./PokerCard";

export type PlayerStatus = "normal" | "active" | "folded" | "allIn" | "winner";
export type PlayerRole = "D" | "SB" | "BB";

export interface PlayerSeatProps {
  name: string;
  model: string;
  avatar: string;
  chips: number;
  status: PlayerStatus;
  roles: PlayerRole[];
  cards: { suit: Suit; value: number; faceDown?: boolean }[];
  /** left-side seats (4, 5) are mirrored */
  mirrored?: boolean;
  reason?: string;
  /** Countdown progress 0–1 (1 = full, 0 = empty). Only rendered when active. */
  countdown?: number;
}

const RING_COLOR: Record<PlayerStatus, string> = {
  normal: "black",
  active: "#00d74b",
  folded: "#868686",
  allIn: "black",
  winner: "black",
};

const CHIP_BG: Record<PlayerStatus, string> = {
  normal: "bg-black",
  active: "bg-black",
  folded: "bg-[#868686]",
  allIn: "bg-[#ff4343]",
  winner: "bg-[#00d74b]",
};

// SVG ring constants (64px diameter, 4px stroke)
const RADIUS = 28;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

function AvatarRing({ color, progress }: { color: string; progress?: number }) {
  const hasProgress = progress !== undefined;
  return (
    <svg
      className="absolute inset-0 z-10 -rotate-90"
      width={64}
      height={64}
      viewBox="0 0 64 64"
    >
      {/* Full ring (static or track) */}
      <circle
        cx={32}
        cy={32}
        r={RADIUS}
        fill="none"
        stroke={hasProgress ? "black" : color}
        strokeWidth={4}
        opacity={hasProgress ? 0.15 : 1}
      />
      {/* Progress arc (only when countdown active) */}
      {hasProgress && (
        <circle
          cx={32}
          cy={32}
          r={RADIUS}
          fill="none"
          stroke={color}
          strokeWidth={4}
          strokeLinecap="round"
          strokeDasharray={CIRCUMFERENCE}
          strokeDashoffset={CIRCUMFERENCE * (1 - progress)}
          className="transition-[stroke-dashoffset] duration-1000 ease-linear"
        />
      )}
    </svg>
  );
}

/** Dealer flower badge */
function DealerBadge({ overlap = false }: { overlap?: boolean }) {
  return (
    <div
      className="relative z-[2] flex size-6 shrink-0 items-center justify-center"
      style={overlap ? { marginLeft: -8 } : undefined}
    >
      <img
        src="/icons/dealer-badge.svg"
        alt=""
        className="absolute inset-0 size-full"
      />
      <span className="relative text-xs font-black text-black">D</span>
    </div>
  );
}

/** Circle badge for SB / BB */
function CircleBadge({ label, bg, overlap = false }: { label: string; bg: string; overlap?: boolean }) {
  return (
    <div
      className={`z-[1] flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-full ${bg}`}
      style={overlap ? { marginLeft: -8 } : undefined}
    >
      <span className="text-xs font-medium text-white">{label}</span>
    </div>
  );
}

export default function PlayerSeat({
  name,
  model,
  avatar,
  chips,
  status,
  roles,
  cards,
  mirrored = false,
  reason,
  countdown,
}: PlayerSeatProps) {
  const chipLabel =
    status === "allIn" ? "ALL IN" : `$${chips.toLocaleString()}`;

  return (
    <div className="relative flex w-[300px] flex-col items-center gap-1">
      {/* Hand cards */}
      <div className="relative flex justify-center gap-1">
        {cards.map((c, i) => (
          <div key={i} className="relative">
            <PokerCard
              suit={c.suit}
              value={c.value}
              faceDown={c.faceDown}
              size="sm"
            />
          </div>
        ))}
        {/* Fold overlay centered on both cards */}
        {status === "folded" && (
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="rounded-full bg-[#ff4343] px-2 py-1 text-xs font-medium text-white">
              Fold
            </span>
          </div>
        )}
      </div>

      {/* Player info row */}
      <div
        className={`flex w-full items-center justify-between ${mirrored ? "flex-row-reverse" : ""}`}
      >
        {/* Left group: avatar + name */}
        <div
          className={`flex items-center gap-2 ${mirrored ? "flex-row-reverse" : ""}`}
        >
          {/* Avatar with ring */}
          <div className="relative size-16 shrink-0">
            <AvatarRing
              color={RING_COLOR[status]}
              progress={status === "active" ? countdown : undefined}
            />
            {/* Avatar circle: 56px centered, white bg */}
            <div className="absolute inset-1 overflow-hidden rounded-full bg-white">
              <Image
                src={avatar}
                alt={name}
                width={56}
                height={56}
                className={`size-full object-cover ${status === "folded" ? "grayscale" : ""}`}
              />
            </div>
          </div>

          {/* Name + model */}
          <div
            className={`flex w-[98px] flex-col gap-1 ${mirrored ? "items-end" : "items-start"}`}
          >
            <span className="max-w-[98px] truncate text-base font-medium text-black">
              {name}
            </span>
            <span className="max-w-[98px] truncate text-xs font-medium text-black">
              {model}
            </span>
          </div>
        </div>

        {/* Chips badge */}
        <div
          className={`rounded-full px-4 py-2 text-base font-medium text-white ${CHIP_BG[status]}`}
        >
          {chipLabel}
        </div>
      </div>

      {/* Role badges — centered under avatar */}
      {roles.length > 0 && (
        <div
          className="absolute top-[118px] z-20 flex -translate-x-1/2 items-center"
          style={{ left: mirrored ? "calc(100% - 32px)" : 32 }}
        >
          {roles.map((r, i) =>
            r === "D" ? (
              <DealerBadge key={r} overlap={i > 0} />
            ) : (
              <CircleBadge
                key={r}
                label={r}
                bg={r === "SB" ? "bg-[#868686]" : "bg-black"}
                overlap={i > 0}
              />
            ),
          )}
        </div>
      )}

      {/* Reason speech bubble — anchored to avatar center */}
      {status === "active" && reason && (
        <div
          className={`absolute top-[31px] z-30 ${
            mirrored ? "right-[200px]" : "left-[-23px]"
          }`}
        >
          <div className="whitespace-nowrap rounded-lg bg-black px-2 py-1.5 text-xs font-normal tracking-wide text-white">
            {reason}
          </div>
          <img
            src="/icons/tooltip-arrow.svg"
            alt=""
            className="mx-auto mt-[-0.5px] h-[5px] w-3"
          />
        </div>
      )}
    </div>
  );
}
