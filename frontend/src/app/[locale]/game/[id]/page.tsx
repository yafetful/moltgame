"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { API, type GameState, type WSMessage } from "@/lib/api";
import { PokerSpectator } from "@/components/poker/PokerSpectator";
import { WerewolfSpectator } from "@/components/werewolf/WerewolfSpectator";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";

export default function GamePage() {
  const params = useParams();
  const gameID = params.id as string;
  const t = useTranslations("common");
  const tGame = useTranslations("game");
  const [state, setState] = useState<GameState | null>(null);
  const [gameType, setGameType] = useState<"poker" | "werewolf" | null>(null);
  const [gameOver, setGameOver] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const detectType = useCallback((s: GameState) => {
    if (s.community !== undefined || s.hand_num !== undefined) {
      setGameType("poker");
    } else if (s.day !== undefined || s.speeches !== undefined) {
      setGameType("werewolf");
    }
  }, []);

  // Primary: REST polling (works immediately)
  useEffect(() => {
    let active = true;

    const poll = async () => {
      try {
        const s = await API.gameSpectate(gameID);
        if (!active) return;
        setState(s);
        detectType(s);
        setError(null);
      } catch (err) {
        if (!active) return;
        if (err && typeof err === "object" && "status" in err && (err as { status: number }).status === 404) {
          setGameOver(true);
          if (pollRef.current) clearInterval(pollRef.current);
        } else {
          setError(tGame("connectionError"));
        }
      }
    };

    // Initial fetch
    poll();
    // Poll every 1.5s
    pollRef.current = setInterval(poll, 1500);

    return () => {
      active = false;
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [gameID, detectType, tGame]);

  // Optional: try WebSocket for real-time updates (upgrades from polling if available)
  useEffect(() => {
    const wsUrl = API.wsSpectate(gameID);
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      try {
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
          // WebSocket connected — slow down polling
          if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
          }
        };

        ws.onmessage = (event) => {
          try {
            const msg = JSON.parse(event.data) as WSMessage;
            if (msg.type === "state" && msg.payload) {
              const s = msg.payload as GameState;
              setState(s);
              detectType(s);
            }
          } catch {
            // ignore
          }
        };

        ws.onclose = () => {
          // Fall back to polling
          if (!pollRef.current && !gameOver) {
            pollRef.current = setInterval(async () => {
              try {
                const s = await API.gameSpectate(gameID);
                setState(s);
              } catch {
                // ignore
              }
            }, 1500);
          }
          // Try reconnecting WS after 5s
          if (!gameOver) {
            reconnectTimer = setTimeout(connect, 5000);
          }
        };

        ws.onerror = () => {
          ws.close();
        };
      } catch {
        // WebSocket not available, polling is the fallback
      }
    };

    connect();

    return () => {
      if (reconnectTimer) clearTimeout(reconnectTimer);
      wsRef.current?.close();
    };
  }, [gameID, detectType, gameOver]);

  // Game over — show replay link
  if (gameOver) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-20 text-center">
        <h2 className="mb-4 text-2xl font-bold">{t("gameOver")}</h2>
        <p className="mb-8 text-white/50">{tGame("gameFinished")}</p>
        <div className="flex items-center justify-center gap-4">
          <Link
            href={`/game/${gameID}/replay`}
            className="rounded-lg bg-emerald-500 px-6 py-3 font-semibold text-black transition-colors hover:bg-emerald-400"
          >
            {tGame("watchReplay")}
          </Link>
          <Link
            href="/lobby"
            className="rounded-lg border border-white/20 px-6 py-3 font-semibold text-white transition-colors hover:bg-white/5"
          >
            {tGame("backToLobby")}
          </Link>
        </div>
      </div>
    );
  }

  // Error state
  if (error && !state) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 text-white/40">
        <p>{error}</p>
        <button
          onClick={() => window.location.reload()}
          className="rounded-lg border border-white/20 px-4 py-2 text-sm text-white/60 hover:bg-white/5"
        >
          {t("retry")}
        </button>
      </div>
    );
  }

  // Loading
  if (!state) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center text-white/40">
        {t("loading")}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      {gameType === "poker" && <PokerSpectator state={state} />}
      {gameType === "werewolf" && <WerewolfSpectator state={state} />}
      {!gameType && (
        <div className="text-center text-white/40">{t("loading")}</div>
      )}
    </div>
  );
}
