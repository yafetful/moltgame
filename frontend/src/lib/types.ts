// Backend JSON types — matches Go structs in poker/types.go

export interface ApiGameState {
  game_id: string;
  hand_num: number;
  phase: string; // "idle"|"preflop"|"flop"|"turn"|"river"|"showdown"
  finished: boolean;
  community: string[]; // ["Ah", "Ks", "Td"] — 2-char card strings
  current_bet: number;
  small_blind: number;
  big_blind: number;
  dealer_seat: number;
  pots: { amount: number; eligible: number[] }[];
  action_on: number; // seat index or -1
  players: ApiPlayerState[];
  valid_actions?: ApiActionOption[];
  next_hand_at?: string; // ISO 8601 timestamp — present during hand break
}

export interface ApiPlayerState {
  id: string;
  name?: string;
  seat: number;
  chips: number;
  bet: number;
  total_bet: number;
  hole: string[] | null; // ["Ah", "Ks"] or null
  folded: boolean;
  all_in: boolean;
  eliminated: boolean;
  disconnected?: boolean;
}

export interface ApiActionOption {
  type: string;
  min_amount?: number;
  max_amount?: number;
  call_cost?: number;
}

export interface LiveGame {
  game_id: string;
  player_count: number;
  phase: string;
  hand_num: number;
}

export interface RecentGame {
  game_id: string;
  game_type: string;
  player_count: number;
  winner_id?: string;
  winner_name?: string;
  finished_at?: string;
}
