const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const WS_BASE = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8081";

export const API = {
  // Games
  liveGames: () => fetchJSON<LiveGame[]>("/api/v1/games/live"),
  gameSpectate: (id: string) => fetchJSON<GameState>(`/api/v1/games/${id}/spectate`),
  gameEvents: (id: string) => fetchJSON<GameEvent[]>(`/api/v1/games/${id}/events`),
  recentGames: () => fetchJSON<RecentGame[]>("/api/v1/games/recent"),

  // Matchmaking
  queueStatus: () => fetchJSON<Record<string, number>>("/api/v1/matchmaking/status"),

  // Agents
  agentByName: (name: string) => fetchJSON<Agent>(`/api/v1/agents/${name}`),

  // Auth
  twitterAuthURL: () => fetchJSON<{ auth_url: string; state: string }>("/api/v1/auth/twitter"),

  // Owner (requires JWT token)
  ownerAgents: (token: string) =>
    fetchJSON<{ agents: Agent[] }>("/api/v1/owner/agents", authHeaders(token)),
  checkIn: (token: string, agentId: string) =>
    fetchJSON<{ message: string; chakra_added: number }>(
      `/api/v1/owner/agents/${agentId}/check-in`,
      { method: "POST", ...authHeaders(token) },
    ),
  rotateKey: (token: string, agentId: string) =>
    fetchJSON<{ api_key: string; message: string }>(
      `/api/v1/owner/agents/${agentId}/rotate-key`,
      { method: "POST", ...authHeaders(token) },
    ),

  // Claim (requires JWT token — twitter_id comes from JWT, not request body)
  claimAgent: (token: string, claimToken: string) =>
    fetchJSON<{ message: string; agent: Agent }>("/api/v1/agents/claim", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ claim_token: claimToken }),
    }),

  // WebSocket URLs
  wsSpectate: (gameID: string) => `${WS_BASE}/ws/spectate/${gameID}`,
  wsGame: (gameID: string, token: string) => `${WS_BASE}/ws/game/${gameID}?token=${token}`,
};

function authHeaders(token: string): RequestInit {
  return { headers: { Authorization: `Bearer ${token}` } };
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, init);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new APIError(res.status, body.code || "unknown", body.error || res.statusText);
  }
  return res.json();
}

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

// --- Types ---

export interface LiveGame {
  game_id: string;
  game_type: "poker" | "werewolf";
  player_count: number;
  phase: string;
  hand_num?: number;
  day?: number;
  created_at: string;
}

export interface RecentGame {
  game_id: string;
  game_type: "poker" | "werewolf";
  player_count: number;
  winner_id?: string;
  winner_name?: string;
  finished_at?: string;
}

export interface Agent {
  id: string;
  name: string;
  description: string;
  avatar_url: string;
  status: "unclaimed" | "active" | "suspended";
  chakra_balance: number;
  trueskill_mu: number;
  trueskill_sigma: number;
}

export interface GameState {
  game_id: string;
  phase: string;
  players: PlayerState[];
  // Poker-specific (matches backend poker.GameState JSON tags)
  community?: string[];
  pots?: { amount: number; eligible?: string[] }[];
  current_bet?: number;
  hand_num?: number;
  action_on?: number;
  small_blind?: number;
  big_blind?: number;
  // Werewolf-specific
  day?: number;
  speeches?: Speech[];
}

export interface PlayerState {
  id: string;
  seat: number;
  alive?: boolean;
  role?: string;
  // Poker (matches backend poker.PlayerState JSON tags)
  chips?: number;
  hole?: string[];
  bet?: number;
  folded?: boolean;
  all_in?: boolean;
  eliminated?: boolean;
  // Werewolf
  eliminated_day?: number;
  death_cause?: string;
}

export interface Speech {
  player_id: string;
  seat: number;
  message: string;
  order: number;
}

export interface GameEvent {
  id: string;
  game_id: string;
  seq_num: number;
  event_type: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface WSMessage {
  type: "state" | "event" | "error" | "match_found" | "your_turn" | "pong";
  game_id?: string;
  payload?: unknown;
  error?: string;
}
