package game

import (
	"context"
	"fmt"

	"github.com/moltgame/backend/internal/chakra"
	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/internal/trueskill"
)

// SettlementService handles end-of-game settlement: Chakra distribution + rating updates.
type SettlementService struct {
	gameRepo   *Repository
	chakraRepo *chakra.Repository
}

// NewSettlementService creates a new settlement service.
func NewSettlementService(gameRepo *Repository, chakraRepo *chakra.Repository) *SettlementService {
	return &SettlementService{
		gameRepo:   gameRepo,
		chakraRepo: chakraRepo,
	}
}

// PlayerResult describes a player's outcome in a finished game.
type PlayerResult struct {
	AgentID string
	Rank    int // 1-based rank (1 = winner)
}

// SettleConfig is the configuration for settling a game.
type SettleConfig struct {
	GameID    string
	GameType  models.GameType
	EntryFee  int // Chakra per player
	RakeRate  float64 // 0.0 to 1.0 (e.g., 0.05 = 5% rake)
	Results   []PlayerResult
	WinnerID  *string // for games with a single winner (optional)
}

// Settle processes end-of-game settlement.
func (s *SettlementService) Settle(ctx context.Context, cfg SettleConfig) error {
	n := len(cfg.Results)
	if n < 2 {
		return fmt.Errorf("need at least 2 players for settlement")
	}

	totalPool := cfg.EntryFee * n
	rakeAmount := int(float64(totalPool) * cfg.RakeRate)
	prizePool := totalPool - rakeAmount

	// Calculate prize distribution
	prizes := distributePrizes(prizePool, n)

	// Get current ratings for all players
	gamePlayers, err := s.gameRepo.GetGamePlayers(ctx, cfg.GameID)
	if err != nil {
		return fmt.Errorf("get game players: %w", err)
	}

	// Build TrueSkill ranked players
	rankedPlayers := make([]trueskill.RankedPlayer, n)
	for i, res := range cfg.Results {
		var mu, sigma float64
		for _, gp := range gamePlayers {
			if gp.AgentID == res.AgentID {
				mu = gp.MuBefore
				sigma = gp.SigmaBefore
				break
			}
		}
		rankedPlayers[i] = trueskill.RankedPlayer{
			ID:     res.AgentID,
			Rating: trueskill.Rating{Mu: mu, Sigma: sigma},
			Rank:   res.Rank,
		}
	}

	// Compute new ratings
	updatedRatings := trueskill.UpdateRatings(rankedPlayers)

	// Apply results for each player
	for i, res := range cfg.Results {
		agentID := res.AgentID
		prize := prizes[i]
		newRating := updatedRatings[i].Rating

		// Credit prize (if any)
		if prize > 0 {
			gameID := cfg.GameID
			note := fmt.Sprintf("rank #%d prize", res.Rank)
			if err := s.chakraRepo.Credit(ctx, agentID, prize, models.ChakraTypePrize, &gameID, note); err != nil {
				return fmt.Errorf("credit prize to %s: %w", agentID, err)
			}
		}

		// Update TrueSkill rating in agents table
		if err := s.gameRepo.UpdateAgentRating(ctx, agentID, newRating.Mu, newRating.Sigma); err != nil {
			return fmt.Errorf("update rating for %s: %w", agentID, err)
		}

		// Update game_players record
		if err := s.gameRepo.UpdatePlayerResult(ctx, cfg.GameID, agentID,
			res.Rank, prize, cfg.EntryFee, newRating.Mu, newRating.Sigma); err != nil {
			return fmt.Errorf("update player result for %s: %w", agentID, err)
		}
	}

	// Mark game as finished
	if err := s.gameRepo.FinishGame(ctx, cfg.GameID, cfg.WinnerID); err != nil {
		return fmt.Errorf("finish game: %w", err)
	}

	return nil
}

// distributePrizes returns prize amounts for each rank position.
// Top-heavy distribution: 1st gets ~60%, 2nd gets ~25%, 3rd gets ~15%.
// For 2-player games: winner takes all.
func distributePrizes(prizePool, numPlayers int) []int {
	prizes := make([]int, numPlayers)

	switch {
	case numPlayers == 2:
		// Winner takes all
		prizes[0] = prizePool

	case numPlayers <= 4:
		// Top 2 get paid: 65% / 35%
		prizes[0] = prizePool * 65 / 100
		prizes[1] = prizePool - prizes[0]

	case numPlayers <= 6:
		// Top 3 get paid: 55% / 30% / 15%
		prizes[0] = prizePool * 55 / 100
		prizes[1] = prizePool * 30 / 100
		prizes[2] = prizePool - prizes[0] - prizes[1]

	default:
		// Top 3 get paid: 50% / 30% / 20%
		prizes[0] = prizePool * 50 / 100
		prizes[1] = prizePool * 30 / 100
		prizes[2] = prizePool - prizes[0] - prizes[1]
	}

	return prizes
}

// CollectEntryFees deducts entry fees from all players before the game starts.
func (s *SettlementService) CollectEntryFees(ctx context.Context, gameID string, playerIDs []string, entryFee int) error {
	for _, agentID := range playerIDs {
		note := "game entry fee"
		if err := s.chakraRepo.Debit(ctx, agentID, entryFee, models.ChakraTypeEntryFee, &gameID, note); err != nil {
			return fmt.Errorf("debit entry fee from %s: %w", agentID, err)
		}
	}
	return nil
}
