package models

import (
	"encoding/json"
	"time"
)

type GameType string

const (
	GameTypePoker    GameType = "poker"
	GameTypeWerewolf GameType = "werewolf"
)

type GameStatus string

const (
	GameStatusWaiting  GameStatus = "waiting"
	GameStatusPlaying  GameStatus = "playing"
	GameStatusFinished GameStatus = "finished"
)

type Game struct {
	ID           string     `json:"id" db:"id"`
	Type         GameType   `json:"type" db:"type"`
	Status       GameStatus `json:"status" db:"status"`
	Config       []byte     `json:"config" db:"config"`
	PlayerCount  int        `json:"player_count" db:"player_count"`
	WinnerID     *string    `json:"winner_id,omitempty" db:"winner_id"`
	SpectatorCnt int        `json:"spectator_count" db:"spectator_count"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty" db:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty" db:"finished_at"`
}

type GamePlayer struct {
	ID          string    `json:"id" db:"id"`
	GameID      string    `json:"game_id" db:"game_id"`
	AgentID     string    `json:"agent_id" db:"agent_id"`
	SeatNumber  int       `json:"seat_number" db:"seat_number"`
	FinalRank   *int      `json:"final_rank,omitempty" db:"final_rank"`
	ChakraWon   int       `json:"chakra_won" db:"chakra_won"`
	ChakraLost  int       `json:"chakra_lost" db:"chakra_lost"`
	JoinedAt    time.Time `json:"joined_at" db:"joined_at"`
	MuBefore    float64   `json:"mu_before" db:"mu_before"`
	MuAfter     *float64  `json:"mu_after,omitempty" db:"mu_after"`
	SigmaBefore float64   `json:"sigma_before" db:"sigma_before"`
	SigmaAfter  *float64  `json:"sigma_after,omitempty" db:"sigma_after"`
}

type GameEvent struct {
	ID        string    `json:"id" db:"id"`
	GameID    string    `json:"game_id" db:"game_id"`
	SeqNum    int       `json:"seq_num" db:"seq_num"`
	EventType string    `json:"event_type" db:"event_type"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
