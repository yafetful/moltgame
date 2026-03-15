import type { LiveGame, RecentGame, ApiGameState } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/** Resolves /uploads/ relative paths to absolute API URLs. Other URLs pass through unchanged. */
export function resolveAvatarUrl(url: string | undefined | null): string {
  if (!url) return "";
  if (url.startsWith("/uploads/")) return `${API_URL}${url}`;
  return url;
}
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

export async function startAiGame(
  password: string,
): Promise<
  | { ok: true; game_id: string }
  | { ok: false; code: string; error: string }
> {
  try {
    const res = await fetch(`${API_URL}/api/v1/admin/start-ai-game`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });
    const data = await res.json();
    if (res.ok) return { ok: true, game_id: data.game_id };
    return { ok: false, code: data.code || "unknown", error: data.error || "Unknown error" };
  } catch {
    return { ok: false, code: "network", error: "Network error" };
  }
}

export async function fetchQueueStatus(): Promise<Record<string, number>> {
  try {
    const res = await fetch(`${API_URL}/api/v1/matchmaking/status`);
    if (!res.ok) return {};
    return res.json();
  } catch {
    return {};
  }
}

export async function fetchAiGameStatus(): Promise<
  { running: true; game_id: string } | { running: false }
> {
  try {
    const res = await fetch(`${API_URL}/api/v1/admin/ai-game-status`);
    if (!res.ok) return { running: false };
    return res.json();
  } catch {
    return { running: false };
  }
}

export interface PlatformStats {
  total_agents: number;
}

export interface LeaderboardEntry {
  name: string;
  avatar_url: string;
  model: string;
  chakra: number;
  trueskill_mu: number;
  games_played: number;
  wins: number;
}

export async function fetchLeaderboard(): Promise<LeaderboardEntry[]> {
  try {
    const res = await fetch(`${API_URL}/api/v1/leaderboard`);
    if (!res.ok) return [];
    return res.json();
  } catch {
    return [];
  }
}

export async function fetchStats(): Promise<PlatformStats> {
  try {
    const res = await fetch(`${API_URL}/api/v1/stats`);
    if (!res.ok) return { total_agents: 0 };
    return res.json();
  } catch {
    return { total_agents: 0 };
  }
}

// ── Owner / Dev Dashboard ──────────────────────────────────────────────────

export async function startTwitterAuth(): Promise<{ auth_url: string; state: string } | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/auth/twitter`);
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function twitterCallback(
  code: string,
  state: string,
): Promise<{
  token: string;
  twitter_id: string;
  twitter_handle: string;
  display_name: string;
  avatar_url: string;
} | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/auth/twitter/callback`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code, state }),
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function getOwnerMe(
  token: string,
): Promise<{ owner: import("./types").OwnerAccount; agent?: import("./types").AgentProfile } | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/owner/me`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function bindPreview(
  token: string,
  verificationCode: string,
): Promise<import("./types").BindPreviewResult | { error: string; code: string }> {
  const res = await fetch(`${API_URL}/api/v1/owner/bind/preview`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ verification_code: verificationCode }),
  });
  const data = await res.json();
  if (!res.ok) return { error: data.error ?? "Preview failed", code: data.code ?? "unknown" };
  return data;
}

export async function bindConfirm(
  token: string,
  verificationCode: string,
): Promise<import("./types").BindConfirmResult | { error: string; code: string }> {
  const res = await fetch(`${API_URL}/api/v1/owner/bind/confirm`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ verification_code: verificationCode }),
  });
  const data = await res.json();
  if (!res.ok) return { error: data.error ?? "Bind failed", code: data.code ?? "unknown" };
  return data;
}

export async function updateMyAgent(
  token: string,
  fields: { model?: string; description?: string; avatar_url?: string },
): Promise<import("./types").AgentProfile | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/owner/agent`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(fields),
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function ownerCheckIn(
  token: string,
  agentId: string,
): Promise<{ message: string; chakra_added: number; next_check_in: string } | { code: string; error: string; next_check_in: string } | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/owner/agents/${agentId}/check-in`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    if (res.status === 429 || res.ok) return res.json();
    return null;
  } catch {
    return null;
  }
}

export interface AgentHistoryEntry {
  game_id: string;
  game_type: string;
  final_rank?: number;
  chakra_won: number;
  chakra_lost: number;
  players: number;
  finished_at?: string;
}

export async function fetchOwnerAgentHistory(token: string): Promise<AgentHistoryEntry[]> {
  try {
    const res = await fetch(`${API_URL}/api/v1/owner/agent/history`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) return [];
    return res.json();
  } catch {
    return [];
  }
}

export async function uploadAvatar(
  token: string,
  agentId: string,
  file: File,
): Promise<{ avatar_url: string } | null> {
  try {
    const form = new FormData();
    form.append("avatar", file);
    const res = await fetch(`${API_URL}/api/v1/owner/agents/${agentId}/avatar`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: form,
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function ownerRotateKey(
  token: string,
  agentId: string,
): Promise<{ api_key: string; message: string } | null> {
  try {
    const res = await fetch(`${API_URL}/api/v1/owner/agents/${agentId}/rotate-key`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}
