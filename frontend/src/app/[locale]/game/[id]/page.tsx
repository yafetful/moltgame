"use client";

import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Link } from "@/i18n/navigation";
import Image from "next/image";
import { useEffect, useRef, useState, useCallback } from "react";
import Nav from "@/components/Nav";
import PokerTable from "@/components/poker/PokerTable";
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

// ─── Seat positions (relative to 1440x684 game area) ───
const SEAT_POSITIONS: {
  left: number;
  top: number;
  mirrored: boolean;
}[] = [
  { left: 570, top: 0, mirrored: false }, // seat 0 — top
  { left: 1140, top: 175, mirrored: false }, // seat 1 — right-top
  { left: 1140, top: 343, mirrored: false }, // seat 2 — right-bottom
  { left: 570, top: 554, mirrored: false }, // seat 3 — bottom
  { left: 0, top: 343, mirrored: true }, // seat 4 — left-bottom
  { left: 0, top: 175, mirrored: true }, // seat 5 — left-top
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

// Avatar assignments per seat
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
    case "preflop":
      return "preflop";
    case "flop":
      return "flop";
    case "turn":
      return "turn";
    case "river":
    case "showdown":
      return "river";
    default:
      return "starting";
  }
}

function mapStageLabel(phase: string): string {
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
      return "";
  }
}

function mapPlayerStatus(
  p: ApiGameState["players"][number],
  actionOn: number,
): PlayerSeatProps["status"] {
  if (p.eliminated) return "eliminated";
  if (p.folded) return "folded";
  if (p.all_in) return "allIn";
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
  if (seat === sbSeat) roles.push("SB");
  if (seat === bbSeat) roles.push("BB");
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
        ? parseCards(p.hole).map((c) => ({
            ...c,
            faceDown: p.folded,
          }))
        : [
            { suit: "clubs" as const, value: 1, faceDown: true },
            { suit: "clubs" as const, value: 1, faceDown: true },
          ];

      return {
        name: p.name || `Player ${p.seat + 1}`,
        model: "",
        avatar: SEAT_AVATARS[p.seat % SEAT_AVATARS.length],
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
      {/* Action label */}
      <div className="text-center text-sm font-medium text-black/70">
        {label}
      </div>

      {/* Timeline slider */}
      <input
        type="range"
        min={0}
        max={totalFrames - 1}
        value={frameIdx}
        onChange={(e) => onSeek(Number(e.target.value))}
        className="h-2 w-full cursor-pointer appearance-none rounded-full bg-black/10 accent-black"
      />

      {/* Controls row */}
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

// ─── Main Page ───

export default function GamePage() {
  const t = useTranslations("game");
  const params = useParams();
  const gameId = params.id as string;

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
  const [speedIdx, setSpeedIdx] = useState(1); // default 1x
  const playTimer = useRef<ReturnType<typeof setInterval>>(undefined);

  // Hand break countdown (between hands)
  const [handBreakCountdown, setHandBreakCountdown] = useState<
    number | undefined
  >(undefined);

  // Pot award animation
  const [potAward, setPotAward] = useState<{
    winners: { seat: number; amount: number }[];
    startedAt: number;
  } | null>(null);
  const potAwardTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Countdown ring & reason tooltip (live mode)
  const [countdown, setCountdown] = useState(1);
  const [lastActionInfo, setLastActionInfo] = useState<{
    seat: number;
    reason: string;
  } | null>(null);
  const turnStartRef = useRef(Date.now());
  const reasonTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Current state for display
  const currentState = isReplay
    ? frames[frameIdx]?.state ?? null
    : liveState;

  // ─── Live WebSocket ───
  const connectWs = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const ws = new WebSocket(spectateWsUrl(gameId));
    wsRef.current = ws;

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (msg.type === "state" && msg.payload) {
          setLiveState(msg.payload as ApiGameState);
          setError(null);
        } else if (msg.type === "event" && msg.payload) {
          // Events array from poker engine (includes player_action with reason)
          const events = Array.isArray(msg.payload)
            ? msg.payload
            : [msg.payload];
          for (const event of events) {
            if (event.type === "player_action" && event.data?.reason) {
              setLastActionInfo({
                seat: event.data.seat,
                reason: event.data.reason,
              });
              clearTimeout(reasonTimeoutRef.current);
              reasonTimeoutRef.current = setTimeout(
                () => setLastActionInfo(null),
                5000,
              );
            }
            if (event.type === "pot_awarded" && event.data?.winners) {
              const winners = (
                event.data.winners as {
                  seat: number;
                  amount: number;
                }[]
              ).map((w) => ({ seat: w.seat, amount: w.amount }));
              setPotAward({ winners, startedAt: Date.now() });
              clearTimeout(potAwardTimerRef.current);
              potAwardTimerRef.current = setTimeout(
                () => setPotAward(null),
                2000,
              );
            }
          }
        }
      } catch {
        // ignore
      }
    };

    ws.onclose = () => {
      reconnectTimer.current = setTimeout(connectWs, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [gameId]);

  // ─── Initialize: decide live vs replay ───
  useEffect(() => {
    fetchSpectatorState(gameId).then((s) => {
      if (!s) {
        setError("Game not found");
        return;
      }

      if (s.finished) {
        // Replay mode — load events and build frames
        setIsReplay(true);
        // Extract player names from the finished state
        const names: Record<string, string> = {};
        for (const p of s.players) {
          if (p.id && p.name) names[p.id] = p.name;
        }
        fetchGameEvents(gameId).then((events) => {
          const built = buildReplayFrames(gameId, events, names);
          if (built.length > 0) {
            setFrames(built);
            setFrameIdx(0);
          } else {
            // No frames built — show final state
            setLiveState(s);
          }
        });
      } else {
        // Live mode
        setLiveState(s);
        connectWs();
      }
    });

    return () => {
      clearTimeout(reconnectTimer.current);
      clearTimeout(reasonTimeoutRef.current);
      clearTimeout(potAwardTimerRef.current);
      wsRef.current?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId]);

  // ─── Replay playback timer ───
  useEffect(() => {
    if (playing && frames.length > 0) {
      playTimer.current = setInterval(() => {
        setFrameIdx((prev) => {
          if (prev >= frames.length - 1) {
            setPlaying(false);
            return prev;
          }
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
  useEffect(() => {
    if (
      isReplay ||
      !currentState ||
      currentState.action_on < 0 ||
      currentState.finished
    ) {
      return;
    }

    const timer = setInterval(() => {
      const elapsed = Date.now() - turnStartRef.current;
      setCountdown(Math.max(0, 1 - elapsed / TURN_TIMEOUT));
    }, 100);

    return () => clearInterval(timer);
  }, [
    isReplay,
    currentState?.action_on,
    currentState?.hand_num,
    currentState?.phase,
    currentState?.finished,
  ]);

  // ─── Hand break countdown ───
  useEffect(() => {
    if (isReplay || !liveState) {
      setHandBreakCountdown(undefined);
      return;
    }
    const isBreak =
      liveState.phase === "idle" &&
      !liveState.finished &&
      !!liveState.next_hand_at;

    if (!isBreak) {
      setHandBreakCountdown(undefined);
      return;
    }

    const target = Date.parse(liveState.next_hand_at!);
    const calc = () =>
      Math.max(0, Math.ceil((target - Date.now()) / 1000));

    setHandBreakCountdown(calc());
    const timer = setInterval(() => {
      const remaining = calc();
      setHandBreakCountdown(remaining);
      if (remaining <= 0) clearInterval(timer);
    }, 1000);

    return () => clearInterval(timer);
  }, [
    isReplay,
    liveState?.phase,
    liveState?.finished,
    liveState?.next_hand_at,
  ]);

  // ─── Replay pot award detection ───
  useEffect(() => {
    if (!isReplay) return;
    const frame = frames[frameIdx];
    if (frame?.eventType === "pot_awarded" && frame.potAwardWinners) {
      setPotAward({
        winners: frame.potAwardWinners,
        startedAt: Date.now(),
      });
      clearTimeout(potAwardTimerRef.current);
      potAwardTimerRef.current = setTimeout(() => setPotAward(null), 2000);
    } else {
      setPotAward(null);
    }
  }, [isReplay, frameIdx, frames]);

  // Derive display props
  const display = currentState ? apiStateToProps(currentState) : null;

  // Override winner status when pot award is active
  if (display && potAward) {
    const winnerSeats = new Set(potAward.winners.map((w) => w.seat));
    for (let i = 0; i < display.players.length; i++) {
      if (winnerSeats.has(i)) {
        display.players[i] = { ...display.players[i], status: "winner" };
      }
    }
  }

  const replayLabel = isReplay && frames[frameIdx]
    ? frames[frameIdx].label
    : "";
  const replayFrame = isReplay ? frames[frameIdx] : null;

  return (
    <main className="min-h-screen bg-[#e8f5e9]">
      <Nav variant="logo" />

      {/* Header */}
      <div className="mx-auto flex max-w-[1440px] items-center justify-between px-8">
        <div className="flex items-center gap-4">
          <Link
            href="/lobby/poker"
            className="flex items-center gap-1 text-black"
          >
            <img
              src="/icons/arrow-up.svg"
              alt=""
              className="size-4 -rotate-90"
            />
            <span className="text-base font-semibold">{t("back")}</span>
          </Link>
          <div className="flex items-center gap-2">
            <Image
              src="/icons/poker.png"
              alt="Poker"
              width={32}
              height={32}
              className="size-8 object-contain"
            />
            <span className="text-lg font-semibold text-black">
              #{gameId.slice(0, 8)}
            </span>
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
              <span className="text-sm font-medium text-black/60">
                Hand #{currentState.hand_num}
              </span>
              <span className="text-sm font-semibold text-black">
                {currentState.finished
                  ? "Finished"
                  : mapStageLabel(currentState.phase)}
              </span>
            </>
          )}
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
                  countdown={
                    !isReplay && player.status === "active"
                      ? countdown
                      : undefined
                  }
                  reason={
                    replayFrame?.actionSeat === i
                      ? replayFrame.reason
                      : lastActionInfo?.seat === i
                        ? lastActionInfo.reason
                        : undefined
                  }
                />
              </div>
            ))}

            {/* Pot award fly animation */}
            {potAward && (
              <PotAwardOverlay winners={potAward.winners} />
            )}
          </div>
        )}

        {!display && !error && (
          <div className="flex h-[400px] items-center justify-center">
            <p className="text-lg text-black/60">Loading...</p>
          </div>
        )}

        {/* Game Over overlay (live mode only — when game finishes) */}
        {!isReplay && currentState?.finished && display && (
          <div className="mx-auto mt-4 max-w-[720px] rounded-2xl border-2 border-black/10 bg-white/90 p-6 shadow-lg backdrop-blur">
            <h2 className="mb-4 text-center text-2xl font-bold text-black">
              {t("gameOver")}
            </h2>
            <div className="space-y-2">
              {[...currentState.players]
                .sort((a, b) => {
                  // Winner (most chips, not eliminated) first
                  if (a.eliminated !== b.eliminated) return a.eliminated ? 1 : -1;
                  return b.chips - a.chips;
                })
                .map((p, idx) => (
                  <div
                    key={p.id}
                    className={`flex items-center justify-between rounded-xl px-4 py-2 ${
                      idx === 0 && !p.eliminated
                        ? "bg-amber-100 font-semibold"
                        : "bg-black/5"
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <span className="w-6 text-center text-sm font-bold text-black/60">
                        #{idx + 1}
                      </span>
                      <img
                        src={SEAT_AVATARS[p.seat % SEAT_AVATARS.length]}
                        alt=""
                        className="size-8 rounded-full"
                      />
                      <span className="text-sm font-medium text-black">
                        {p.name || `Player ${p.seat + 1}`}
                      </span>
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
            onPlay={() => {
              if (frameIdx >= frames.length - 1) setFrameIdx(0);
              setPlaying(true);
            }}
            onPause={() => setPlaying(false)}
            onStep={() => {
              setPlaying(false);
              setFrameIdx((i) => Math.min(i + 1, frames.length - 1));
            }}
            onStepBack={() => {
              setPlaying(false);
              setFrameIdx((i) => Math.max(i - 1, 0));
            }}
            onSeek={(idx) => {
              setPlaying(false);
              setFrameIdx(idx);
            }}
            onSpeed={() =>
              setSpeedIdx((i) => (i + 1) % SPEED_OPTIONS.length)
            }
          />
        )}
      </div>
    </main>
  );
}
