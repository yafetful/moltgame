"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { API, type GameEvent, type GameState, type PlayerState } from "@/lib/api";
import { PokerSpectator } from "@/components/poker/PokerSpectator";
import { WerewolfSpectator } from "@/components/werewolf/WerewolfSpectator";
import { ExportButton } from "@/components/ExportButton";
import { useVideoExporter } from "@/hooks/useVideoExporter";
import { cn } from "@/lib/utils";

type PlaybackSpeed = 0.5 | 1 | 2 | 4;

export default function ReplayPage() {
  const params = useParams();
  const gameID = params.id as string;
  const t = useTranslations("common");

  const [events, setEvents] = useState<GameEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const [currentIdx, setCurrentIdx] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<PlaybackSpeed>(1);
  const [gameType, setGameType] = useState<"poker" | "werewolf" | null>(null);

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const { startRecording, progress: exportProgress, abort: abortExport, reset: resetExport } = useVideoExporter();

  // Load events
  useEffect(() => {
    API.gameEvents(gameID)
      .then((evts) => {
        setEvents(evts);
        // Detect game type from first event
        if (evts.length > 0) {
          const p = evts[0].payload;
          if (p.community !== undefined || p.hand_num !== undefined || p.hole !== undefined) {
            setGameType("poker");
          } else if (p.day !== undefined || p.speeches !== undefined) {
            setGameType("werewolf");
          }
        }
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [gameID]);

  // Build state from events up to currentIdx
  const state = buildStateFromEvents(events, currentIdx);

  // Detect game type from built state if not yet detected
  useEffect(() => {
    if (!gameType && state) {
      if (state.community !== undefined || state.hand_num !== undefined) {
        setGameType("poker");
      } else if (state.day !== undefined || state.speeches !== undefined) {
        setGameType("werewolf");
      }
    }
  }, [gameType, state]);

  // Playback timer
  const advance = useCallback(() => {
    setCurrentIdx((prev) => {
      if (prev >= events.length - 1) {
        setPlaying(false);
        return prev;
      }
      return prev + 1;
    });
  }, [events.length]);

  useEffect(() => {
    if (playing) {
      const interval = 1500 / speed;
      timerRef.current = setTimeout(advance, interval);
    }
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [playing, currentIdx, speed, advance]);

  // Stop playback when export finishes or errors
  useEffect(() => {
    if (exportProgress.phase === "done" || exportProgress.phase === "error") {
      setPlaying(false);
    }
  }, [exportProgress.phase]);

  if (loading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center text-white/40">
        {t("loading")}
      </div>
    );
  }

  if (error || events.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center text-white/40">
        {t("error")}
      </div>
    );
  }

  const handleExport = () => {
    // Reset to beginning and start 1x playback
    setCurrentIdx(0);
    setSpeed(1);
    setPlaying(true);
    resetExport();

    // Record in real-time for the full playback duration
    const durationMs = events.length * 1500; // 1.5s per event at 1x speed
    startRecording(containerRef, durationMs, {
      width: 1280,
      height: 720,
      watermarkText: "moltgame.com",
    });
  };

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      {/* Game display — fixed 16:9 container for stable video export */}
      <div
        ref={containerRef}
        className="relative mx-auto aspect-video w-full max-w-4xl overflow-hidden rounded-xl bg-[#0a0a0a]"
      >
        <div className="absolute inset-0 overflow-hidden">
          {state && gameType === "poker" && <PokerSpectator state={state} />}
          {state && gameType === "werewolf" && <WerewolfSpectator state={state} />}
        </div>
      </div>

      {/* Playback controls */}
      <div className="mt-6 rounded-xl border border-white/10 bg-white/5 px-6 py-4">
        <div className="flex items-center gap-4">
          {/* Play/Pause */}
          <button
            onClick={() => setPlaying((p) => !p)}
            className="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
          >
            {playing ? "⏸" : "▶"}
          </button>

          {/* Progress bar */}
          <div className="flex-1">
            <input
              type="range"
              min={0}
              max={Math.max(0, events.length - 1)}
              value={currentIdx}
              onChange={(e) => {
                setCurrentIdx(Number(e.target.value));
                setPlaying(false);
              }}
              className="w-full accent-emerald-500"
            />
            <div className="mt-1 flex justify-between text-[10px] text-white/30">
              <span>{currentIdx + 1} / {events.length}</span>
              <span>{events[currentIdx]?.event_type}</span>
            </div>
          </div>

          {/* Speed controls */}
          <div className="flex gap-1">
            {([0.5, 1, 2, 4] as PlaybackSpeed[]).map((s) => (
              <button
                key={s}
                onClick={() => setSpeed(s)}
                className={cn(
                  "rounded px-2 py-1 text-xs font-medium transition-colors",
                  speed === s ? "bg-white/15 text-white" : "text-white/40 hover:text-white/70",
                )}
              >
                {s}x
              </button>
            ))}
          </div>

          {/* Step controls */}
          <div className="flex gap-1">
            <button
              onClick={() => {
                setCurrentIdx((i) => Math.max(0, i - 1));
                setPlaying(false);
              }}
              className="rounded px-2 py-1 text-xs text-white/40 hover:text-white/70"
            >
              ◀
            </button>
            <button
              onClick={() => {
                setCurrentIdx((i) => Math.min(events.length - 1, i + 1));
                setPlaying(false);
              }}
              className="rounded px-2 py-1 text-xs text-white/40 hover:text-white/70"
            >
              ▶
            </button>
          </div>

          {/* Export */}
          <ExportButton
            progress={exportProgress}
            onExport={handleExport}
            onCancel={abortExport}
            disabled={exportProgress.phase === "capturing"}
          />
        </div>
      </div>
    </div>
  );
}

/**
 * Build a GameState by replaying events as a state machine.
 * Each event type has specific semantics — we handle them individually
 * rather than shallow-merging payloads.
 */
function buildStateFromEvents(events: GameEvent[], upToIdx: number): GameState | null {
  if (events.length === 0) return null;

  const state: GameState = {
    game_id: events[0].game_id,
    phase: "waiting",
    players: [],
    community: [],
    pots: [],
    hand_num: 0,
  };

  // Helper: find or create a player by seat
  const getPlayer = (seat: number): PlayerState => {
    let p = state.players.find((pl) => pl.seat === seat);
    if (!p) {
      p = { id: "", seat, chips: 0 };
      state.players.push(p);
    }
    return p;
  };

  for (let i = 0; i <= upToIdx && i < events.length; i++) {
    const evt = events[i];
    const d = evt.payload as Record<string, unknown>;

    switch (evt.event_type) {
      case "hand_start": {
        const players = d.players as { id: string; seat: number; chips: number }[];
        state.hand_num = d.hand_num as number;
        state.small_blind = d.small_blind as number;
        state.big_blind = d.big_blind as number;
        state.phase = "preflop";
        state.community = [];
        state.pots = [];
        state.current_bet = 0;
        state.action_on = undefined;
        // Reset players from snapshot, preserving eliminated status
        const eliminated = new Set(
          state.players.filter((p) => p.eliminated).map((p) => p.seat),
        );
        state.players = players.map((pi) => ({
          id: pi.id,
          seat: pi.seat,
          chips: pi.chips,
          folded: false,
          all_in: false,
          bet: 0,
          hole: [],
          eliminated: eliminated.has(pi.seat),
        }));
        break;
      }

      case "blinds_posted": {
        const sbSeat = d.sb_seat as number;
        const sbAmt = d.sb_amount as number;
        const bbSeat = d.bb_seat as number;
        const bbAmt = d.bb_amount as number;
        const sbPlayer = getPlayer(sbSeat);
        sbPlayer.chips = (sbPlayer.chips ?? 0) - sbAmt;
        sbPlayer.bet = sbAmt;
        const bbPlayer = getPlayer(bbSeat);
        bbPlayer.chips = (bbPlayer.chips ?? 0) - bbAmt;
        bbPlayer.bet = bbAmt;
        state.current_bet = bbAmt;
        state.pots = [{ amount: sbAmt + bbAmt }];
        break;
      }

      case "hole_dealt": {
        const seat = d.seat as number;
        const cards = d.cards as string[];
        const p = getPlayer(seat);
        p.hole = cards;
        break;
      }

      case "player_action": {
        // If no hand_start was seen yet, infer we're in hand 1
        if (state.hand_num === 0) {
          state.hand_num = 1;
          state.phase = "preflop";
        }

        const seat = d.seat as number;
        const action = d.action as string;
        const amount = (d.amount as number) || 0;
        const chipsLeft = d.chips_left as number;
        const totalPot = d.total_pot as number;
        const p = getPlayer(seat);

        // Always update player_id (needed when hand_start is missing)
        if (d.player_id) {
          p.id = d.player_id as string;
        }

        if (action === "fold") {
          p.folded = true;
          p.chips = chipsLeft;
        } else if (action === "allin" || action === "all_in") {
          p.all_in = true;
          p.bet = (p.bet ?? 0) + amount;
          p.chips = chipsLeft;
        } else {
          // call, raise, check, bet
          p.bet = (p.bet ?? 0) + amount;
          p.chips = chipsLeft;
          if (action === "raise" || action === "bet") {
            state.current_bet = p.bet;
          }
        }
        state.pots = [{ amount: totalPot }];
        break;
      }

      case "community_dealt": {
        const phase = d.phase as string;
        const board = d.board as string[];
        state.phase = phase;
        state.community = board;
        state.current_bet = 0;
        // Reset per-round bets
        for (const p of state.players) {
          p.bet = 0;
        }
        break;
      }

      case "showdown": {
        state.phase = "showdown";
        const showPlayers = d.players as {
          seat: number;
          player_id: string;
          hole: string[];
          hand_desc: string;
        }[];
        const board = d.board as string[];
        if (board) state.community = board;
        for (const sp of showPlayers) {
          const p = getPlayer(sp.seat);
          p.hole = sp.hole;
        }
        break;
      }

      case "pot_awarded": {
        const winners = d.winners as { seat: number; amount: number }[];
        for (const w of winners) {
          const p = getPlayer(w.seat);
          p.chips = (p.chips ?? 0) + w.amount;
        }
        break;
      }

      case "hand_end": {
        state.phase = "idle";
        state.action_on = undefined;
        state.current_bet = 0;
        // Update chips from snapshot
        const endPlayers = d.players as { id: string; seat: number; chips: number }[];
        for (const ep of endPlayers) {
          const p = getPlayer(ep.seat);
          p.chips = ep.chips;
          p.bet = 0;
          p.folded = false;
          p.all_in = false;
          p.hole = [];
        }
        state.community = [];
        state.pots = [];
        break;
      }

      case "player_eliminated": {
        const seat = d.seat as number;
        const p = getPlayer(seat);
        p.eliminated = true;
        break;
      }

      case "game_over": {
        state.phase = "finished";
        break;
      }
    }
  }

  return state;
}
