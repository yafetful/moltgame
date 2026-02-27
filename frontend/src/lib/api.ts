import type { LiveGame, RecentGame, ApiGameState } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8081";

export async function fetchLiveGames(): Promise<LiveGame[]> {
  const res = await fetch(`${API_URL}/api/v1/games/live`);
  if (!res.ok) return [];
  return res.json();
}

export async function fetchRecentGames(): Promise<RecentGame[]> {
  const res = await fetch(`${API_URL}/api/v1/games/recent`);
  if (!res.ok) return [];
  return res.json();
}

export async function fetchSpectatorState(
  gameId: string,
): Promise<ApiGameState | null> {
  const res = await fetch(`${API_URL}/api/v1/games/${gameId}/spectate`);
  if (!res.ok) return null;
  return res.json();
}

export interface GameEvent {
  seq_num: number;
  event_type: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export async function fetchGameEvents(
  gameId: string,
): Promise<GameEvent[]> {
  const res = await fetch(`${API_URL}/api/v1/games/${gameId}/events`);
  if (!res.ok) return [];
  return res.json();
}

export function spectateWsUrl(gameId: string): string {
  return `${WS_URL}/ws/spectate/${gameId}`;
}
