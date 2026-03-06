package game

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moltgame/backend/internal/models"
)

// Repository handles game persistence.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new game repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateGame inserts a new game record and its players.
func (r *Repository) CreateGame(ctx context.Context, gameType models.GameType, playerIDs []string, config json.RawMessage) (*models.Game, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var game models.Game
	err = tx.QueryRow(ctx,
		`INSERT INTO games (type, status, config, player_count)
		 VALUES ($1, 'playing', $2, $3)
		 RETURNING id, type, status, config, player_count, spectator_count, created_at, started_at`,
		gameType, config, len(playerIDs),
	).Scan(&game.ID, &game.Type, &game.Status, &game.Config,
		&game.PlayerCount, &game.SpectatorCnt, &game.CreatedAt, &game.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("insert game: %w", err)
	}

	// Insert players with their current TrueSkill ratings
	for seat, agentID := range playerIDs {
		var mu, sigma float64
		err = tx.QueryRow(ctx,
			`SELECT trueskill_mu, trueskill_sigma FROM agents WHERE id = $1`,
			agentID,
		).Scan(&mu, &sigma)
		if err != nil {
			return nil, fmt.Errorf("get agent %s rating: %w", agentID, err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO game_players (game_id, agent_id, seat_number, mu_before, sigma_before)
			 VALUES ($1, $2, $3, $4, $5)`,
			game.ID, agentID, seat, mu, sigma,
		)
		if err != nil {
			return nil, fmt.Errorf("insert game_player %s: %w", agentID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &game, nil
}

// FinishGame marks a game as finished and records final results.
func (r *Repository) FinishGame(ctx context.Context, gameID string, winnerID *string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE games SET status = 'finished', winner_id = $2, finished_at = $3 WHERE id = $1`,
		gameID, winnerID, now,
	)
	return err
}

// UpdatePlayerResult updates a player's final rank and rating changes.
func (r *Repository) UpdatePlayerResult(ctx context.Context, gameID, agentID string, finalRank int, chakraWon, chakraLost int, muAfter, sigmaAfter float64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE game_players
		 SET final_rank = $3, chakra_won = $4, chakra_lost = $5, mu_after = $6, sigma_after = $7
		 WHERE game_id = $1 AND agent_id = $2`,
		gameID, agentID, finalRank, chakraWon, chakraLost, muAfter, sigmaAfter,
	)
	return err
}

// RecordEvents batch-inserts game events for Event Sourcing.
func (r *Repository) RecordEvents(ctx context.Context, gameID string, startSeq int, events []models.GameEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for i, evt := range events {
		batch.Queue(
			`INSERT INTO game_events (game_id, seq_num, event_type, payload, created_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			gameID, startSeq+i, evt.EventType, evt.Payload, evt.CreatedAt,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range events {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}
	return nil
}

// GetGame retrieves a game by ID.
func (r *Repository) GetGame(ctx context.Context, gameID string) (*models.Game, error) {
	var game models.Game
	err := r.db.QueryRow(ctx,
		`SELECT id, type, status, config, player_count, winner_id, spectator_count,
		        created_at, started_at, finished_at
		 FROM games WHERE id = $1`,
		gameID,
	).Scan(&game.ID, &game.Type, &game.Status, &game.Config,
		&game.PlayerCount, &game.WinnerID, &game.SpectatorCnt,
		&game.CreatedAt, &game.StartedAt, &game.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

// GetGamePlayers retrieves all players in a game.
func (r *Repository) GetGamePlayers(ctx context.Context, gameID string) ([]models.GamePlayer, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, game_id, agent_id, seat_number, final_rank, chakra_won, chakra_lost,
		        joined_at, mu_before, mu_after, sigma_before, sigma_after
		 FROM game_players WHERE game_id = $1 ORDER BY seat_number`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.GamePlayer
	for rows.Next() {
		var p models.GamePlayer
		if err := rows.Scan(&p.ID, &p.GameID, &p.AgentID, &p.SeatNumber,
			&p.FinalRank, &p.ChakraWon, &p.ChakraLost,
			&p.JoinedAt, &p.MuBefore, &p.MuAfter, &p.SigmaBefore, &p.SigmaAfter); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// GetGameEvents retrieves events for replay.
func (r *Repository) GetGameEvents(ctx context.Context, gameID string) ([]models.GameEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, game_id, seq_num, event_type, payload, created_at
		 FROM game_events WHERE game_id = $1 ORDER BY seq_num`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.GameEvent
	for rows.Next() {
		var e models.GameEvent
		if err := rows.Scan(&e.ID, &e.GameID, &e.SeqNum, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListLiveGames returns currently playing games.
func (r *Repository) ListLiveGames(ctx context.Context, limit int) ([]models.Game, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, type, status, config, player_count, winner_id, spectator_count,
		        created_at, started_at, finished_at
		 FROM games WHERE status = 'playing' ORDER BY created_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var g models.Game
		if err := rows.Scan(&g.ID, &g.Type, &g.Status, &g.Config,
			&g.PlayerCount, &g.WinnerID, &g.SpectatorCnt,
			&g.CreatedAt, &g.StartedAt, &g.FinishedAt); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// RecentGame is a lightweight struct for the recent games listing.
type RecentGame struct {
	ID          string          `json:"game_id"`
	Type        models.GameType `json:"game_type"`
	PlayerCount int             `json:"player_count"`
	WinnerID    *string         `json:"winner_id,omitempty"`
	WinnerName  *string         `json:"winner_name,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

// ListRecentGames returns recently finished games.
func (r *Repository) ListRecentGames(ctx context.Context, limit int) ([]RecentGame, error) {
	rows, err := r.db.Query(ctx,
		`SELECT g.id, g.type, g.player_count, g.winner_id, g.finished_at,
		        a.name AS winner_name
		 FROM games g
		 LEFT JOIN agents a ON a.id = g.winner_id
		 WHERE g.status = 'finished'
		 ORDER BY g.finished_at DESC NULLS LAST
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []RecentGame
	for rows.Next() {
		var g RecentGame
		if err := rows.Scan(&g.ID, &g.Type, &g.PlayerCount,
			&g.WinnerID, &g.FinishedAt, &g.WinnerName); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// FindActiveGameForAgent returns the game ID of an active game the agent is in.
// Returns empty string if the agent is not in any active game.
func (r *Repository) FindActiveGameForAgent(ctx context.Context, agentID string) (string, error) {
	var gameID string
	err := r.db.QueryRow(ctx,
		`SELECT g.id FROM games g
		 JOIN game_players gp ON g.id = gp.game_id
		 WHERE gp.agent_id = $1 AND g.status = 'playing'
		 LIMIT 1`,
		agentID,
	).Scan(&gameID)
	if err != nil {
		return "", err
	}
	return gameID, nil
}

// FindRecentlyFinishedGameForAgent returns a game that finished within the last
// 60 seconds for this agent. Used to catch game_over events that an agent might
// have missed due to polling gaps.
func (r *Repository) FindRecentlyFinishedGameForAgent(ctx context.Context, agentID string) (string, int, error) {
	var gameID string
	var rank int
	err := r.db.QueryRow(ctx,
		`SELECT g.id, gp.final_rank FROM games g
		 JOIN game_players gp ON g.id = gp.game_id
		 WHERE gp.agent_id = $1 AND g.status = 'finished'
		   AND g.finished_at > NOW() - INTERVAL '60 seconds'
		 ORDER BY g.finished_at DESC
		 LIMIT 1`,
		agentID,
	).Scan(&gameID, &rank)
	if err != nil {
		return "", 0, err
	}
	return gameID, rank, nil
}

// AgentGameHistory is a lightweight struct for an agent's game history.
type AgentGameHistory struct {
	GameID     string          `json:"game_id"`
	GameType   models.GameType `json:"game_type"`
	FinalRank  *int            `json:"final_rank,omitempty"`
	ChakraWon  int             `json:"chakra_won"`
	ChakraLost int             `json:"chakra_lost"`
	Players    int             `json:"players"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

// GetAgentHistory returns an agent's recent game history.
func (r *Repository) GetAgentHistory(ctx context.Context, agentID string, limit int) ([]AgentGameHistory, error) {
	rows, err := r.db.Query(ctx,
		`SELECT g.id, g.type, g.player_count, gp.final_rank, gp.chakra_won, gp.chakra_lost, g.finished_at
		 FROM game_players gp
		 JOIN games g ON g.id = gp.game_id
		 WHERE gp.agent_id = $1 AND g.status = 'finished'
		 ORDER BY g.finished_at DESC NULLS LAST
		 LIMIT $2`,
		agentID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []AgentGameHistory
	for rows.Next() {
		var h AgentGameHistory
		if err := rows.Scan(&h.GameID, &h.GameType, &h.Players,
			&h.FinalRank, &h.ChakraWon, &h.ChakraLost, &h.FinishedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

// UpdateAgentRating updates an agent's TrueSkill rating.
func (r *Repository) UpdateAgentRating(ctx context.Context, agentID string, mu, sigma float64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE agents SET trueskill_mu = $2, trueskill_sigma = $3 WHERE id = $1`,
		agentID, mu, sigma,
	)
	return err
}
