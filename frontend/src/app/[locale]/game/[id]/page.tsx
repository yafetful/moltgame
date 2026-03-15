"use client";

import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Link } from "@/i18n/navigation";
import Image from "next/image";
import { useEffect, useRef, useState, useCallback } from "react";
import Nav from "@/components/Nav";
import PokerTable from "@/components/poker/PokerTable";
import PokerCard from "@/components/poker/PokerCard";
import PlayerSeat from "@/components/poker/PlayerSeat";
import PotAwardOverlay from "@/components/poker/PotAwardOverlay";
import type { PlayerSeatProps } from "@/components/poker/PlayerSeat";
import type {
  CommunityCard,
  BetPosition,
  Stage,
} from "@/components/poker/PokerTable";
import type { ApiGameState } from "@/lib/types";
import {
  fetchSpectatorState,
  fetchGameEvents,
  spectateWsUrl,
} from "@/lib/api";
import { parseCards } from "@/lib/cardUtils";
import { buildReplayFrames } from "@/lib/replayEngine";
import type { ReplayFrame } from "@/lib/replayEngine";
import type { PlayerRole } from "@/components/poker/PlayerSeat";
import { useSoundEffects } from "@/hooks/useSoundEffects";

// ─── Seat positions (relative to 1440x684 game area) ───
const SEAT_POSITIONS: {
  left: number;
  top: number;
  mirrored: boolean;
}[] = [
  { left: 570, top: 0, mirrored: false },    // seat 0 — top
  { left: 1140, top: 175, mirrored: false }, // seat 1 — right-top
  { left: 1140, top: 343, mirrored: false }, // seat 2 — right-bottom
  { left: 570, top: 554, mirrored: false },  // seat 3 — bottom
  { left: 0, top: 343, mirrored: true },     // seat 4 — left-bottom
  { left: 0, top: 175, mirrored: true },     // seat 5 — left-top
];

// ─── Mobile seat positions (relative to 402px-wide game container, after 64px header) ───
const MOBILE_SEAT_POSITIONS: { left: number; top: number; mirrored: boolean }[] = [
  { left: 107, top: 92,  mirrored: false }, // seat 0 — top
  { left: 259, top: 210, mirrored: false }, // seat 1 — right-top
  { left: 259, top: 459, mirrored: false }, // seat 2 — right-bottom
  { left: 105, top: 672, mirrored: false }, // seat 3 — bottom
  { left: 26,  top: 459, mirrored: true  }, // seat 4 — left-bottom
  { left: 25,  top: 210, mirrored: true  }, // seat 5 — left-top
];

// ─── Mobile bet chip positions (within the 320×640 table div, matching Figma) ───
const MOBILE_BET_POSITIONS: { left: number; top: number; seatIndex: number }[] = [
  { left: 127, top: 95,  seatIndex: 0 },
  { left: 225, top: 194, seatIndex: 1 },
  { left: 225, top: 443, seatIndex: 2 },
  { left: 128, top: 494, seatIndex: 3 },
  { left: 28,  top: 443, seatIndex: 4 },
  { left: 28,  top: 193, seatIndex: 5 },
];

// Chip icon per seat (matches Figma design colors)
const SEAT_CHIPS = [
  "/poker/chips/chip-seat0.svg", // seat 0 — green
  "/poker/chips/chip-seat1.svg", // seat 1 — blue-gray
  "/poker/chips/chip-seat2.svg", // seat 2 — pink
  "/poker/chips/chip-seat3.svg", // seat 3 — gold
  "/poker/chips/chip-seat4.svg", // seat 4 — cyan
  "/poker/chips/chip-seat5.svg", // seat 5 — purple
];

// Fallback avatar assignments per seat (used when avatar_url is not set)
const SEAT_AVATARS = [
  "/avatars/01-fox.png",
  "/avatars/02-koala.png",
  "/avatars/03-owl.png",
  "/avatars/04-cat.png",
  "/avatars/05-bear.png",
  "/avatars/06-rabbit.png",
];

const SPEED_OPTIONS = [
  { label: "0.5x", ms: 2000 },
  { label: "1x", ms: 1000 },
  { label: "2x", ms: 500 },
  { label: "4x", ms: 250 },
];

const TURN_TIMEOUT = 30_000; // 30s, matches backend turnTimeout

// ─── Map API state to component props ───

function mapPhase(phase: string): Stage {
  switch (phase) {
    case "preflop": return "preflop";
    case "flop":    return "flop";
    case "turn":    return "turn";
    case "river":
    case "showdown": return "river";
    default: return "starting";
  }
}

function mapStageLabel(phase: string): string {
  switch (phase) {
    case "preflop":  return "Pre-flop";
    case "flop":     return "Flop";
    case "turn":     return "Turn";
    case "river":    return "River";
    case "showdown": return "Showdown";
    default: return "";
  }
}

function mapPlayerStatus(
  p: ApiGameState["players"][number],
  actionOn: number,
): PlayerSeatProps["status"] {
  if (p.eliminated) return "eliminated";
  if (p.folded)     return "folded";
  if (p.all_in)     return "allIn";
  if (p.seat === actionOn) return "active";
  return "normal";
}

function mapPlayerRoles(
  seat: number,
  dealerSeat: number,
  sbSeat: number,
  bbSeat: number,
): PlayerRole[] {
  const roles: PlayerRole[] = [];
  if (seat === dealerSeat) roles.push("D");
  if (seat === sbSeat)     roles.push("SB");
  if (seat === bbSeat)     roles.push("BB");
  return roles;
}

function calcBlindSeats(
  players: ApiGameState["players"],
  dealerSeat: number,
): { sb: number; bb: number } {
  const alive = players.filter((p) => !p.eliminated);
  if (alive.length <= 1) return { sb: -1, bb: -1 };

  if (alive.length === 2) {
    const sbSeat = dealerSeat;
    const bbSeat = alive.find((p) => p.seat !== dealerSeat)!.seat;
    return { sb: sbSeat, bb: bbSeat };
  }

  const sortedAlive = [...alive].sort((a, b) => a.seat - b.seat);
  const findNext = (after: number) => {
    const n = players.length;
    for (let i = 1; i < n; i++) {
      const seat = (after + i) % n;
      if (sortedAlive.some((p) => p.seat === seat)) return seat;
    }
    return after;
  };
  const sb = findNext(dealerSeat);
  const bb = findNext(sb);
  return { sb, bb };
}

function apiStateToProps(state: ApiGameState) {
  const stage = mapPhase(state.phase);
  const stageLabel = mapStageLabel(state.phase);

  const communityCards: CommunityCard[] = [];
  const totalSlots = 5;
  const dealt = state.community?.length || 0;
  for (let i = 0; i < totalSlots; i++) {
    if (i < dealt) {
      const parsed = parseCards([state.community[i]])[0];
      communityCards.push({ ...parsed, faceDown: false });
    } else {
      communityCards.push({ suit: "clubs", value: 1, faceDown: true });
    }
  }

  const pot = state.pots?.reduce((sum, p) => sum + p.amount, 0) || 0;

  const { sb: sbSeat, bb: bbSeat } = calcBlindSeats(
    state.players,
    state.dealer_seat,
  );

  const players: Omit<PlayerSeatProps, "mirrored">[] = state.players.map(
    (p) => {
      const status = mapPlayerStatus(p, state.action_on);
      const cards = p.hole
        ? parseCards(p.hole).map((c) => ({ ...c, faceDown: p.folded }))
        : [
            { suit: "clubs" as const, value: 1, faceDown: true },
            { suit: "clubs" as const, value: 1, faceDown: true },
          ];

      return {
        name: p.name || `Player ${p.seat + 1}`,
        model: "",
        avatar: p.avatar_url || SEAT_AVATARS[p.seat % SEAT_AVATARS.length],
        chips: p.chips,
        status,
        roles: mapPlayerRoles(p.seat, state.dealer_seat, sbSeat, bbSeat),
        cards,
      };
    },
  );

  const bets: BetPosition[] = state.players
    .filter((p) => p.bet > 0)
    .map((p) => ({
      seatIndex: p.seat,
      amount: p.bet,
      chipIcon: SEAT_CHIPS[p.seat % SEAT_CHIPS.length],
    }));

  return { stage, stageLabel, communityCards, pot, players, bets };
}

// ─── Replay Controls ───

function ReplayControls({
  frameIdx,
  totalFrames,
  playing,
  speedIdx,
  label,
  onPlay,
  onPause,
  onStep,
  onStepBack,
  onSeek,
  onSpeed,
}: {
  frameIdx: number;
  totalFrames: number;
  playing: boolean;
  speedIdx: number;
  label: string;
  onPlay: () => void;
  onPause: () => void;
  onStep: () => void;
  onStepBack: () => void;
  onSeek: (idx: number) => void;
  onSpeed: () => void;
}) {
  return (
    <div className="mx-auto mt-2 flex max-w-[720px] flex-col gap-2">
      <div className="text-center text-sm font-medium text-black/70">{label}</div>
      <input
        type="range"
        min={0}
        max={totalFrames - 1}
        value={frameIdx}
        onChange={(e) => onSeek(Number(e.target.value))}
        className="h-2 w-full cursor-pointer appearance-none rounded-full bg-black/10 accent-black"
      />
      <div className="flex items-center justify-center gap-3">
        <button
          onClick={onStepBack}
          disabled={frameIdx <= 0}
          className="rounded-full bg-black/10 px-3 py-1.5 text-sm font-medium text-black transition-colors hover:bg-black/20 disabled:opacity-30"
        >
          Prev
        </button>
        <button
          onClick={playing ? onPause : onPlay}
          className="rounded-full bg-black px-5 py-1.5 text-sm font-semibold text-white transition-colors hover:bg-black/80"
        >
          {playing ? "Pause" : "Play"}
        </button>
        <button
          onClick={onStep}
          disabled={frameIdx >= totalFrames - 1}
          className="rounded-full bg-black/10 px-3 py-1.5 text-sm font-medium text-black transition-colors hover:bg-black/20 disabled:opacity-30"
        >
          Next
        </button>
        <button
          onClick={onSpeed}
          className="rounded-full bg-black/10 px-3 py-1.5 text-sm font-medium text-black transition-colors hover:bg-black/20"
        >
          {SPEED_OPTIONS[speedIdx].label}
        </button>
        <span className="text-xs text-black/50">
          {frameIdx + 1} / {totalFrames}
        </span>
      </div>
    </div>
  );
}

// ─── Mobile Player Seat ───

const MOBILE_RING_COLOR: Record<PlayerSeatProps["status"], string> = {
  normal:     "black",
  active:     "#00d74b",
  folded:     "#868686",
  allIn:      "black",
  winner:     "#00d74b",
  eliminated: "#868686",
};

const MOBILE_RADIUS = 21;
const MOBILE_CIRCUMFERENCE = 2 * Math.PI * MOBILE_RADIUS;

function MobilePlayerSeat({
  name,
  chips,
  status,
  avatar,
  roles,
  cards,
  mirrored = false,
  countdown,
  reason,
}: Omit<PlayerSeatProps, "model" | "handDesc">) {
  const chipLabel =
    status === "allIn"      ? "ALL IN" :
    status === "eliminated" ? "OUT"    :
    `$${chips.toLocaleString()}`;

  const labelBg =
    status === "winner" || status === "active"
      ? "bg-[#00d74b]"
      : status === "allIn"
        ? "bg-[#ff4343]"
        : "bg-black";

  // Avatar center x within the 120px seat
  const avatarCenterX = mirrored ? 24 : 96;

  return (
    <div
      className={`relative flex w-[120px] items-center gap-1 ${mirrored ? "flex-row-reverse" : ""} ${status === "eliminated" ? "opacity-50" : ""}`}
    >
      {/* Floating name + chips label above avatar */}
      <div
        className="absolute z-20"
        style={{ left: avatarCenterX, top: -38, transform: "translateX(-50%)" }}
      >
        <div className={`flex flex-col items-center rounded-md px-2 py-0.5 ${labelBg}`}>
          <span className="max-w-[66px] truncate text-[10px] font-medium text-white">
            {name}
          </span>
          <span className="text-xs font-semibold text-white">{chipLabel}</span>
        </div>
      </div>

      {/* Hole cards (or spacer to keep avatar position when eliminated) */}
      {status === "eliminated" ? (
        <div className="h-[42px] w-[68px] shrink-0" />
      ) : (
        <div className="relative flex h-[42px] gap-1">
          {cards.map((c, i) => (
            <PokerCard
              key={i}
              suit={c.suit}
              value={c.value}
              faceDown={c.faceDown}
              size="xs"
            />
          ))}
          {status === "folded" && (
            <div className="absolute inset-0 flex items-center justify-center">
              <span className="rounded-full bg-[#ff4343] px-1.5 py-0.5 text-[10px] font-medium text-white">
                Fold
              </span>
            </div>
          )}
        </div>
      )}

      {/* Avatar with ring */}
      <div className="relative size-12 shrink-0">
        <svg
          className="absolute inset-0 z-10 -rotate-90"
          width="100%"
          height="100%"
          viewBox="0 0 48 48"
        >
          <circle
            cx={24} cy={24} r={MOBILE_RADIUS}
            fill="none"
            stroke={countdown !== undefined ? "black" : MOBILE_RING_COLOR[status]}
            strokeWidth={3}
            opacity={countdown !== undefined ? 0.15 : 1}
          />
          {countdown !== undefined && (
            <circle
              cx={24} cy={24} r={MOBILE_RADIUS}
              fill="none"
              stroke={MOBILE_RING_COLOR[status]}
              strokeWidth={3}
              strokeLinecap="round"
              strokeDasharray={MOBILE_CIRCUMFERENCE}
              strokeDashoffset={MOBILE_CIRCUMFERENCE * (1 - countdown)}
              className="transition-[stroke-dashoffset] duration-1000 ease-linear"
            />
          )}
        </svg>
        <div className="absolute inset-[3px] overflow-hidden rounded-full bg-white">
          <Image
            src={avatar}
            alt={name}
            width={42}
            height={42}
            className={`size-full object-cover ${status === "folded" || status === "eliminated" ? "grayscale" : ""}`}
          />
        </div>

        {/* Role badges */}
        {roles.length > 0 && status !== "eliminated" && (
          <div className="absolute -bottom-[10px] left-1/2 z-20 flex -translate-x-1/2 items-center">
            {roles.map((r, i) =>
              r === "D" ? (
                <div
                  key={r}
                  className="relative z-[2] flex size-5 shrink-0 items-center justify-center"
                  style={i > 0 ? { marginLeft: -6 } : undefined}
                >
                  <img src="/icons/dealer-badge.svg" alt="" className="absolute inset-0 size-full" />
                  <span className="relative text-[10px] font-black text-black">D</span>
                </div>
              ) : (
                <div
                  key={r}
                  className={`z-[1] flex size-5 shrink-0 items-center justify-center overflow-hidden rounded-full ${r === "SB" ? "bg-[#868686]" : "bg-black"}`}
                  style={i > 0 ? { marginLeft: -6 } : undefined}
                >
                  <span className="text-[10px] font-medium text-white">{r}</span>
                </div>
              ),
            )}
          </div>
        )}
      </div>

      {/* Reason tooltip */}
      {reason && (
        <div
          className="absolute z-30 whitespace-nowrap rounded-lg bg-black px-2 py-1 text-[10px] text-white"
          style={{ left: avatarCenterX, bottom: 54, transform: "translateX(-50%)" }}
        >
          {reason}
        </div>
      )}
    </div>
  );
}

// ─── Main Page ───

export default function GamePage() {
  const t = useTranslations("game");
  const params = useParams();
  const gameId = params.id as string;
  const { play, playAction, muted, toggleMute } = useSoundEffects();

  // Shared
  const [error, setError] = useState<string | null>(null);

  // Live mode
  const [liveState, setLiveState] = useState<ApiGameState | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Replay mode
  const [isReplay, setIsReplay] = useState(false);
  const [frames, setFrames] = useState<ReplayFrame[]>([]);
  const [frameIdx, setFrameIdx] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speedIdx, setSpeedIdx] = useState(1);
  const playTimer = useRef<ReturnType<typeof setInterval>>(undefined);

  // Hand break countdown (between hands)
  const [handBreakCountdown, setHandBreakCountdown] = useState<number | undefined>(undefined);

  // Pot award animation
  const [potAward, setPotAward] = useState<{
    winners: { seat: number; amount: number }[];
    startedAt: number;
  } | null>(null);
  const potAwardTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // All-in run-out animation
  const communityAnimatingRef = useRef(false);
  const pendingLiveStateRef = useRef<ApiGameState | null>(null);

  // Final rankings from game_over event
  const [finalRanks, setFinalRanks] = useState<Record<number, number>>({});
  // Showdown hand descriptions
  const [handDescs, setHandDescs] = useState<Record<number, string>>({});

  // Countdown ring & reason tooltip (live mode)
  const [countdown, setCountdown] = useState(1);
  const [lastActionInfo, setLastActionInfo] = useState<{ seat: number; reason: string } | null>(null);
  const turnStartRef = useRef(Date.now());
  const reasonTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Mobile clock
  const [mobileTime, setMobileTime] = useState(() => {
    const d = new Date();
    return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
  });

  // Mobile canvas scale: measured directly from the canvas area's rendered dimensions
  const canvasAreaRef = useRef<HTMLDivElement>(null);
  const [mobileScale, setMobileScale] = useState(1);
  useEffect(() => {
    const el = canvasAreaRef.current;
    if (!el) return;
    const obs = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setMobileScale(Math.min(width / 402, height / 810, 1));
    });
    obs.observe(el);
    return () => obs.disconnect();
  }, []);

  // Current state for display
  const currentState = isReplay ? frames[frameIdx]?.state ?? null : liveState;

  // ─── Live WebSocket ───
  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const ws = new WebSocket(spectateWsUrl(gameId));
    wsRef.current = ws;

    ws.onopen = () => {
      fetchSpectatorState(gameId).then((s) => {
        if (s && !s.finished && !communityAnimatingRef.current) {
          setLiveState(s);
          setError(null);
        }
      });
    };

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (msg.type === "state" && msg.payload) {
          if (communityAnimatingRef.current) {
            pendingLiveStateRef.current = msg.payload as ApiGameState;
          } else {
            setLiveState(msg.payload as ApiGameState);
            setError(null);
          }
          return;
        }

        if (msg.type !== "event" || !msg.payload) return;

        const events: { type: string; data?: Record<string, unknown> }[] =
          Array.isArray(msg.payload) ? msg.payload : [msg.payload];

        for (const event of events) {
          if (event.type === "player_action" && event.data?.reason) {
            setLastActionInfo({ seat: event.data.seat as number, reason: event.data.reason as string });
            clearTimeout(reasonTimeoutRef.current);
            reasonTimeoutRef.current = setTimeout(() => setLastActionInfo(null), 5000);
          }
          if (event.type === "player_action" && event.data?.action_type) {
            playAction(event.data.action_type as string);
          }
          if (event.type === "showdown" && event.data?.players) {
            const descs: Record<number, string> = {};
            const sdPlayers = event.data.players as { seat: number; hand_desc: string }[];
            for (const sp of sdPlayers) {
              if (sp.hand_desc) descs[sp.seat] = sp.hand_desc;
            }
            setHandDescs(descs);
            play("showdown");
          }
          if (event.type === "hand_start") {
            setHandDescs({});
          }
          if (event.type === "player_eliminated") {
            play("eliminated");
          }
          if (event.type === "game_over" && event.data?.rankings) {
            const ranks: Record<number, number> = {};
            const rankings = event.data.rankings as { rank: number; seat: number }[];
            for (const r of rankings) ranks[r.seat] = r.rank;
            setFinalRanks(ranks);
            play("game-over");
          }
        }

        const communityDeals = events.filter((e) => e.type === "community_dealt");

        if (communityDeals.length > 0) {
          const DEAL_INTERVAL_MS = 1500;
          const SHOWDOWN_PAUSE_MS = 1500;
          const POT_AWARD_PAUSE_MS = 1500;
          communityAnimatingRef.current = true;

          communityDeals.forEach((deal, i) => {
            setTimeout(() => {
              play("deal");
              const board = (deal.data?.board ?? []) as string[];
              const phase = (deal.data?.phase ?? "") as string;
              setLiveState((prev) => prev ? { ...prev, community: board, phase } : prev);

              if (i === communityDeals.length - 1) {
                setTimeout(() => {
                  if (pendingLiveStateRef.current) {
                    setLiveState(pendingLiveStateRef.current);
                    setError(null);
                    pendingLiveStateRef.current = null;
                  } else {
                    fetchSpectatorState(gameId).then((s) => {
                      if (s) { setLiveState(s); setError(null); }
                    });
                  }
                  communityAnimatingRef.current = false;
                }, SHOWDOWN_PAUSE_MS);
              }
            }, (i + 1) * DEAL_INTERVAL_MS);
          });

          const potDelay = communityDeals.length * DEAL_INTERVAL_MS + SHOWDOWN_PAUSE_MS + POT_AWARD_PAUSE_MS;
          const allPotWinners = events
            .filter((e) => e.type === "pot_awarded" && e.data?.winners)
            .flatMap((e) => (e.data!.winners as { seat: number; amount: number }[]).map((w) => ({ seat: w.seat, amount: w.amount })));
          if (allPotWinners.length > 0) {
            setTimeout(() => {
              play("pot-win");
              setPotAward({ winners: allPotWinners, startedAt: Date.now() });
              clearTimeout(potAwardTimerRef.current);
              potAwardTimerRef.current = setTimeout(() => setPotAward(null), 2000);
            }, potDelay);
          }
        } else {
          const allPotWinners = events
            .filter((e) => e.type === "pot_awarded" && e.data?.winners)
            .flatMap((e) => (e.data!.winners as { seat: number; amount: number }[]).map((w) => ({ seat: w.seat, amount: w.amount })));
          if (allPotWinners.length > 0) {
            play("pot-win");
            setPotAward({ winners: allPotWinners, startedAt: Date.now() });
            clearTimeout(potAwardTimerRef.current);
            potAwardTimerRef.current = setTimeout(() => setPotAward(null), 2000);
          }
        }
      } catch {
        // ignore
      }
    };

    ws.onclose = () => { reconnectTimer.current = setTimeout(connectWs, 3000); };
    ws.onerror = () => { ws.close(); };
  }, [gameId]);

  // ─── Initialize: decide live vs replay ───
  useEffect(() => {
    fetchSpectatorState(gameId).then((s) => {
      if (!s) { setError("Game not found"); return; }

      if (s.finished) {
        setIsReplay(true);
        const names: Record<string, string> = {};
        const avatars: Record<string, string> = {};
        for (const p of s.players) {
          if (p.id && p.name)       names[p.id]   = p.name;
          if (p.id && p.avatar_url) avatars[p.id] = p.avatar_url;
        }
        fetchGameEvents(gameId).then((events) => {
          const built = buildReplayFrames(gameId, events, names, avatars);
          if (built.length > 0) {
            setFrames(built);
            setFrameIdx(0);
          } else {
            setLiveState(s);
          }
        });
      } else {
        setLiveState(s);
        connectWs();
      }
    });

    return () => {
      clearTimeout(reconnectTimer.current);
      clearTimeout(reasonTimeoutRef.current);
      clearTimeout(potAwardTimerRef.current);
      communityAnimatingRef.current = false;
      pendingLiveStateRef.current = null;
      wsRef.current?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId]);

  // ─── Replay playback timer ───
  useEffect(() => {
    if (playing && frames.length > 0) {
      playTimer.current = setInterval(() => {
        setFrameIdx((prev) => {
          if (prev >= frames.length - 1) { setPlaying(false); return prev; }
          return prev + 1;
        });
      }, SPEED_OPTIONS[speedIdx].ms);
    }
    return () => clearInterval(playTimer.current);
  }, [playing, speedIdx, frames.length]);

  // ─── Countdown ring: reset when turn changes ───
  useEffect(() => {
    if (!currentState || currentState.action_on < 0) return;
    turnStartRef.current = Date.now();
    setCountdown(1);
  }, [currentState?.action_on, currentState?.hand_num, currentState?.phase]);

  // ─── Countdown ring: animate in live mode ───
  const timeoutTickedRef = useRef(false);
  useEffect(() => {
    if (isReplay || !currentState || currentState.action_on < 0 || currentState.finished) return;
    timeoutTickedRef.current = false;
    const timer = setInterval(() => {
      const elapsed = Date.now() - turnStartRef.current;
      const next = Math.max(0, 1 - elapsed / TURN_TIMEOUT);
      setCountdown(next);
      // Play timeout warning once when under 20% remaining (≈6s left)
      if (next < 0.2 && !timeoutTickedRef.current) {
        timeoutTickedRef.current = true;
        play("timeout-tick");
      }
    }, 100);
    return () => clearInterval(timer);
  }, [isReplay, currentState?.action_on, currentState?.hand_num, currentState?.phase, currentState?.finished, play]);

  // ─── Hand break countdown ───
  useEffect(() => {
    if (isReplay || !liveState) { setHandBreakCountdown(undefined); return; }
    const isBreak = liveState.phase === "idle" && !liveState.finished && !!liveState.next_hand_at;
    if (!isBreak) { setHandBreakCountdown(undefined); return; }

    const target = Date.parse(liveState.next_hand_at!);
    const calc = () => Math.max(0, Math.ceil((target - Date.now()) / 1000));
    setHandBreakCountdown(calc());
    const timer = setInterval(() => {
      const remaining = calc();
      setHandBreakCountdown(remaining);
      if (remaining <= 0) clearInterval(timer);
    }, 1000);
    return () => clearInterval(timer);
  }, [isReplay, liveState?.phase, liveState?.finished, liveState?.next_hand_at]);

  // ─── Replay pot award detection + sounds ───
  useEffect(() => {
    if (!isReplay) return;
    const frame = frames[frameIdx];
    if (!frame) return;
    if (frame.eventType === "pot_awarded" && frame.potAwardWinners) {
      play("pot-win");
      setPotAward({ winners: frame.potAwardWinners, startedAt: Date.now() });
      clearTimeout(potAwardTimerRef.current);
      potAwardTimerRef.current = setTimeout(() => setPotAward(null), 2000);
    }
    if (frame.eventType === "player_action" && frame.actionSeat !== undefined) {
      // Extract action type from label (e.g. "Alice folds", "Bob checks")
      const label = frame.label.toLowerCase();
      if (label.includes("all in")) playAction("allin");
      else if (label.includes("fold")) playAction("fold");
      else if (label.includes("check")) playAction("check");
      else if (label.includes("call")) playAction("call");
      else if (label.includes("raise")) playAction("raise");
    }
    if (frame.eventType === "community_dealt") play("deal");
    if (frame.eventType === "showdown") play("showdown");
    if (frame.eventType === "player_eliminated") play("eliminated");
    if (frame.eventType === "game_over") play("game-over");
  }, [isReplay, frameIdx, frames, play, playAction]);

  // ─── Mobile clock ───
  useEffect(() => {
    const update = () => {
      const d = new Date();
      setMobileTime(`${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`);
    };
    const timer = setInterval(update, 60000);
    return () => clearInterval(timer);
  }, []);

  // Derive display props
  const display = currentState ? apiStateToProps(currentState) : null;

  // Override winner status when pot award is active
  if (display && potAward) {
    const winnerSeats = new Set(potAward.winners.map((w) => w.seat));
    for (let i = 0; i < display.players.length; i++) {
      if (winnerSeats.has(i)) display.players[i] = { ...display.players[i], status: "winner" };
    }
  }

  const replayLabel = isReplay && frames[frameIdx] ? frames[frameIdx].label : "";
  const replayFrame = isReplay ? frames[frameIdx] : null;

  // Shared replay controls callbacks
  const replayCallbacks = {
    onPlay:     () => { if (frameIdx >= frames.length - 1) setFrameIdx(0); setPlaying(true); },
    onPause:    () => setPlaying(false),
    onStep:     () => { setPlaying(false); setFrameIdx((i) => Math.min(i + 1, frames.length - 1)); },
    onStepBack: () => { setPlaying(false); setPotAward(null); setFrameIdx((i) => Math.max(i - 1, 0)); },
    onSeek:     (idx: number) => { setPlaying(false); setPotAward(null); setFrameIdx(idx); },
    onSpeed:    () => setSpeedIdx((i) => (i + 1) % SPEED_OPTIONS.length),
  };

  return (
    <main className="bg-[#e8f5e9]">

      {/* ═══════════════════════════════════════════════
          MOBILE LAYOUT
      ═══════════════════════════════════════════════ */}
      <div className="flex flex-col md:hidden" style={{ height: "100dvh" }}>

        {/* Mobile header */}
        <div className="flex h-16 shrink-0 items-center justify-between bg-[#e8f5e9] px-4">
          {/* Left: back + icon + game ID */}
          <Link href="/lobby/poker" className="flex items-center gap-1 text-black">
            <img src="/icons/arrow-up.svg" alt="" className="size-4 -rotate-90" />
            <Image src="/icons/poker.png" alt="Poker" width={32} height={32} className="size-8 object-contain" />
            <span className="text-base font-medium text-black">#{gameId.slice(0, 8)}</span>
          </Link>

          {/* Right: Live/Replay indicator + mute */}
          <div className="flex items-center gap-4">
            {isReplay ? (
              <span className="rounded-full bg-black/10 px-3 py-1 text-xs font-semibold text-black/60">
                REPLAY
              </span>
            ) : (
              <>
                <div className="flex items-center gap-1 px-2 py-1">
                  <span
                    className="inline-block size-6 shrink-0 bg-black"
                    style={{ maskImage: "url(/icons/live.svg)", maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center", WebkitMaskImage: "url(/icons/live.svg)", WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center" }}
                  />
                  <span className="text-xs font-medium text-black">{mobileTime}</span>
                </div>
                {currentState && currentState.action_on >= 0 && (
                  <div className="flex items-center gap-1 px-2 py-1">
                    <span
                      className="inline-block size-6 shrink-0 bg-black"
                      style={{ maskImage: "url(/icons/history.svg)", maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center", WebkitMaskImage: "url(/icons/history.svg)", WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center" }}
                    />
                    <span className="text-xs font-medium text-black">Decision</span>
                  </div>
                )}
              </>
            )}
            <button onClick={toggleMute} className="flex size-8 items-center justify-center rounded-full hover:bg-black/10">
              <span className="inline-block size-5 shrink-0 bg-black/60" style={{
                maskImage: muted ? "url(/icons/mute.svg)" : "url(/icons/sound.svg)",
                maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center",
                WebkitMaskImage: muted ? "url(/icons/mute.svg)" : "url(/icons/sound.svg)",
                WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center",
              }} />
            </button>
          </div>
        </div>

        {/* Canvas area — flex-1, centers content both axes; ResizeObserver drives mobileScale */}
        <div ref={canvasAreaRef} className="relative flex flex-1 items-center justify-center overflow-hidden">
        <div style={{ zoom: mobileScale }}>
        <div className="relative mx-auto" style={{ width: 402, height: 810 }}>

          {/* Mobile poker table image */}
          <div
            className="absolute"
            style={{ left: 41, top: 64, width: 320, height: 640 }}
          >
            <Image
              src="/poker/mobile_table.png"
              width={640}
              height={1280}
              className="absolute inset-0 size-full"
              alt=""
              priority
            />

            {/* Community cards + stage label + pot */}
            {display && (
              <div
                className="absolute flex flex-col items-center gap-2"
                style={{ left: 32, top: 245, width: 256 }}
              >
                <span className="text-base font-semibold text-white/80">
                  {handBreakCountdown !== undefined
                    ? `Next Hand #${currentState!.hand_num + 1}`
                    : display.stageLabel}
                </span>
                {handBreakCountdown !== undefined && (
                  <span className="text-sm text-white/60">{handBreakCountdown}s</span>
                )}
                {display.stage !== "starting" && (
                  <div className="flex gap-1">
                    {display.communityCards.map((card, i) => (
                      <PokerCard
                        key={i}
                        suit={card.suit}
                        value={card.value}
                        faceDown={card.faceDown}
                        size="xs"
                      />
                    ))}
                  </div>
                )}
                {display.stage !== "starting" && display.stage !== "preflop" && display.pot > 0 && (
                  <div className="rounded-full bg-[#f5a623] px-4 py-1">
                    <span className="text-sm font-semibold text-black">
                      ${display.pot.toLocaleString()}
                    </span>
                  </div>
                )}
              </div>
            )}

            {/* Bet chips (within table coordinate space) */}
            {display?.bets.map((bet) => {
              const pos = MOBILE_BET_POSITIONS.find((p) => p.seatIndex === bet.seatIndex);
              if (!pos) return null;
              return (
                <div
                  key={bet.seatIndex}
                  className="absolute flex items-center gap-1"
                  style={{ left: pos.left, top: pos.top }}
                >
                  {bet.chipIcon && (
                    <Image src={bet.chipIcon} alt="" width={20} height={20} className="size-5" />
                  )}
                  <span className="text-xs font-semibold text-white [text-shadow:0_1px_2px_rgba(0,0,0,0.8)]">
                    ${bet.amount.toLocaleString()}
                  </span>
                </div>
              );
            })}
          </div>

          {/* Player seats (in game canvas coordinate space) */}
          {display?.players.map((player, i) => {
            const pos = MOBILE_SEAT_POSITIONS[i];
            if (!pos) return null;
            return (
              <div key={i} className="absolute" style={{ left: pos.left, top: pos.top }}>
                <MobilePlayerSeat
                  {...player}
                  mirrored={pos.mirrored}
                  countdown={!isReplay && player.status === "active" ? countdown : undefined}
                  reason={
                    replayFrame?.actionSeat === i
                      ? replayFrame.reason
                      : lastActionInfo?.seat === i
                        ? lastActionInfo.reason
                        : undefined
                  }
                />
              </div>
            );
          })}

          {/* Pot award animation */}
          {potAward && <PotAwardOverlay winners={potAward.winners} mobile />}

          {/* Loading / error */}
          {!display && !error && (
            <div className="flex h-[400px] items-center justify-center">
              <p className="text-lg text-black/60">Loading...</p>
            </div>
          )}
          {error && !currentState && (
            <div className="flex h-[400px] items-center justify-center">
              <p className="text-lg text-black/60">{error}</p>
            </div>
          )}

          {/* Game over overlay */}
          {!isReplay && currentState?.finished && display && (
            <div className="absolute inset-x-4 top-16 z-40 rounded-2xl bg-white/95 p-4 shadow-xl backdrop-blur">
              <h2 className="mb-3 text-center text-xl font-bold text-black">{t("gameOver")}</h2>
              <div className="space-y-1.5">
                {[...currentState.players]
                  .sort((a, b) => {
                    const rankA = finalRanks[a.seat];
                    const rankB = finalRanks[b.seat];
                    if (rankA !== undefined && rankB !== undefined) return rankA - rankB;
                    if (a.eliminated !== b.eliminated) return a.eliminated ? 1 : -1;
                    return b.chips - a.chips;
                  })
                  .map((p, idx) => (
                    <div
                      key={p.id}
                      className={`flex items-center justify-between rounded-xl px-3 py-1.5 ${idx === 0 && !p.eliminated ? "bg-amber-100 font-semibold" : "bg-black/5"}`}
                    >
                      <div className="flex items-center gap-2">
                        <span className="w-5 text-center text-xs font-bold text-black/60">#{idx + 1}</span>
                        <img
                          src={p.avatar_url || SEAT_AVATARS[p.seat % SEAT_AVATARS.length]}
                          alt=""
                          className="size-6 rounded-full"
                        />
                        <span className="text-sm font-medium text-black">{p.name || `Player ${p.seat + 1}`}</span>
                        {idx === 0 && !p.eliminated && (
                          <span className="rounded-full bg-amber-400 px-2 py-0.5 text-xs font-bold text-white">
                            {t("winner")}
                          </span>
                        )}
                      </div>
                      <span className="text-xs font-medium text-black/60">
                        {p.eliminated ? t("eliminated") : `${p.chips.toLocaleString()} ${t("chips")}`}
                      </span>
                    </div>
                  ))}
              </div>
              <div className="mt-3 flex justify-center">
                <Link
                  href="/lobby/poker"
                  className="rounded-full bg-black px-5 py-2 text-sm font-semibold text-white transition-colors hover:bg-black/80"
                >
                  {t("returnToLobby")}
                </Link>
              </div>
            </div>
          )}
        </div>
        </div>{/* end zoom wrapper */}
        </div>{/* end canvas area */}

        {/* Bottom: replay controls */}
        {isReplay && frames.length > 0 ? (
          <div className="shrink-0 bg-[#e8f5e9]/90 px-4 pb-6 pt-2 backdrop-blur">
            <ReplayControls
              frameIdx={frameIdx}
              totalFrames={frames.length}
              playing={playing}
              speedIdx={speedIdx}
              label={replayLabel}
              {...replayCallbacks}
            />
          </div>
        ) : (
          <div className="h-4 shrink-0" />
        )}
      </div>

      {/* ═══════════════════════════════════════════════
          DESKTOP LAYOUT
      ═══════════════════════════════════════════════ */}
      <div className="hidden min-h-screen md:block">
        <Nav variant="logo" />

        {/* Header */}
        <div className="mx-auto flex max-w-[1440px] items-center justify-between px-8">
          <div className="flex items-center gap-4">
            <Link href="/lobby/poker" className="flex items-center gap-1 text-black">
              <img src="/icons/arrow-up.svg" alt="" className="size-4 -rotate-90" />
              <span className="text-base font-semibold">{t("back")}</span>
            </Link>
            <div className="flex items-center gap-2">
              <Image src="/icons/poker.png" alt="Poker" width={32} height={32} className="size-8 object-contain" />
              <span className="text-lg font-semibold text-black">#{gameId.slice(0, 8)}</span>
            </div>
            {isReplay && (
              <span className="rounded-full bg-black/10 px-3 py-1 text-xs font-semibold text-black/60">
                REPLAY
              </span>
            )}
          </div>

          <div className="flex items-center gap-4">
            {currentState && (
              <>
                <span className="text-sm font-medium text-black/60">Hand #{currentState.hand_num}</span>
                <span className="text-sm font-semibold text-black">
                  {currentState.finished ? "Finished" : mapStageLabel(currentState.phase)}
                </span>
              </>
            )}
            <button onClick={toggleMute} className="flex size-8 items-center justify-center rounded-full text-black/40 hover:bg-black/10" title={muted ? "Unmute" : "Mute"}>
              <span className="inline-block size-5 shrink-0 bg-black/50" style={{
                maskImage: muted ? "url(/icons/mute.svg)" : "url(/icons/sound.svg)",
                maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center",
                WebkitMaskImage: muted ? "url(/icons/mute.svg)" : "url(/icons/sound.svg)",
                WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center",
              }} />
            </button>
          </div>
        </div>

        {/* Game area */}
        <div className="mx-auto mt-4 max-w-[1440px] px-8 pb-16">
          {error && !currentState && (
            <div className="flex h-[400px] items-center justify-center">
              <p className="text-lg text-black/60">{error}</p>
            </div>
          )}

          {display && (
            <div className="relative mx-auto h-[684px] w-[1440px]">
              <div className="absolute" style={{ left: 360, top: 162 }}>
                <PokerTable
                  stage={display.stage}
                  stageLabel={display.stageLabel}
                  handNumber={
                    currentState
                      ? handBreakCountdown !== undefined
                        ? `Next Hand #${currentState.hand_num + 1}`
                        : `Hand #${currentState.hand_num}`
                      : undefined
                  }
                  countdown={handBreakCountdown}
                  communityCards={display.communityCards}
                  pot={display.pot}
                  bets={display.bets}
                />
              </div>

              {display.players.map((player, i) => (
                <div
                  key={i}
                  className="absolute"
                  style={{
                    left: SEAT_POSITIONS[i]?.left ?? 0,
                    top: SEAT_POSITIONS[i]?.top ?? 0,
                  }}
                >
                  <PlayerSeat
                    {...player}
                    mirrored={SEAT_POSITIONS[i]?.mirrored ?? false}
                    countdown={!isReplay && player.status === "active" ? countdown : undefined}
                    reason={
                      replayFrame?.actionSeat === i
                        ? replayFrame.reason
                        : lastActionInfo?.seat === i
                          ? lastActionInfo.reason
                          : undefined
                    }
                    handDesc={isReplay ? replayFrame?.playerHandDescs?.[i] : handDescs[i]}
                  />
                </div>
              ))}

              {potAward && <PotAwardOverlay winners={potAward.winners} />}
            </div>
          )}

          {!display && !error && (
            <div className="flex h-[400px] items-center justify-center">
              <p className="text-lg text-black/60">Loading...</p>
            </div>
          )}

          {/* Game Over overlay */}
          {!isReplay && currentState?.finished && display && (
            <div className="mx-auto mt-4 max-w-[720px] rounded-2xl border-2 border-black/10 bg-white/90 p-6 shadow-lg backdrop-blur">
              <h2 className="mb-4 text-center text-2xl font-bold text-black">{t("gameOver")}</h2>
              <div className="space-y-2">
                {[...currentState.players]
                  .sort((a, b) => {
                    const rankA = finalRanks[a.seat];
                    const rankB = finalRanks[b.seat];
                    if (rankA !== undefined && rankB !== undefined) return rankA - rankB;
                    if (a.eliminated !== b.eliminated) return a.eliminated ? 1 : -1;
                    return b.chips - a.chips;
                  })
                  .map((p, idx) => (
                    <div
                      key={p.id}
                      className={`flex items-center justify-between rounded-xl px-4 py-2 ${idx === 0 && !p.eliminated ? "bg-amber-100 font-semibold" : "bg-black/5"}`}
                    >
                      <div className="flex items-center gap-3">
                        <span className="w-6 text-center text-sm font-bold text-black/60">#{idx + 1}</span>
                        <img
                          src={p.avatar_url || SEAT_AVATARS[p.seat % SEAT_AVATARS.length]}
                          alt=""
                          className="size-8 rounded-full"
                        />
                        <span className="text-sm font-medium text-black">{p.name || `Player ${p.seat + 1}`}</span>
                        {idx === 0 && !p.eliminated && (
                          <span className="rounded-full bg-amber-400 px-2 py-0.5 text-xs font-bold text-white">
                            {t("winner")}
                          </span>
                        )}
                      </div>
                      <span className="text-sm font-medium text-black/60">
                        {p.eliminated ? t("eliminated") : `${p.chips.toLocaleString()} ${t("chips")}`}
                      </span>
                    </div>
                  ))}
              </div>
              <div className="mt-4 flex justify-center">
                <Link
                  href="/lobby/poker"
                  className="rounded-full bg-black px-6 py-2 text-sm font-semibold text-white transition-colors hover:bg-black/80"
                >
                  {t("returnToLobby")}
                </Link>
              </div>
            </div>
          )}

          {/* Replay controls */}
          {isReplay && frames.length > 0 && (
            <ReplayControls
              frameIdx={frameIdx}
              totalFrames={frames.length}
              playing={playing}
              speedIdx={speedIdx}
              label={replayLabel}
              {...replayCallbacks}
            />
          )}
        </div>
      </div>
    </main>
  );
}
