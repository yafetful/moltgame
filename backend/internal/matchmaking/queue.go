package matchmaking

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/moltgame/backend/internal/models"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/moltgame/backend/internal/trueskill"
)

// QueueEntry represents an agent waiting for a match.
type QueueEntry struct {
	AgentID  string
	Mu       float64
	Sigma    float64
	JoinedAt time.Time
}

// GameConfig defines matchmaking parameters for a game type.
type GameConfig struct {
	GameType    models.GameType
	MinPlayers  int
	MaxPlayers  int
	EntryFee    int
}

var DefaultConfigs = map[models.GameType]GameConfig{
	models.GameTypePoker: {
		GameType:   models.GameTypePoker,
		MinPlayers: 6,
		MaxPlayers: 6,
		EntryFee:   100,
	},
	models.GameTypeWerewolf: {
		GameType:   models.GameTypeWerewolf,
		MinPlayers: 5,
		MaxPlayers: 5,
		EntryFee:   30,
	},
}

// BotProvider supplies house AI bot agent IDs for backfill.
type BotProvider interface {
	// GetBotAgentIDs returns up to n house bot agent IDs (already in DB).
	GetBotAgentIDs(ctx context.Context, n int) ([]string, error)
	// IsBotAgent returns true if the given agent ID is a house bot.
	IsBotAgent(ctx context.Context, agentID string) bool
}

// BusyBotChecker checks whether a bot is currently in an active game.
type BusyBotChecker interface {
	IsAgentInActiveGame(ctx context.Context, agentID string) bool
}

// Service manages matchmaking queues.
type Service struct {
	mu              sync.Mutex
	queues          map[models.GameType][]*QueueEntry
	nats            *natsClient.Client
	onMatch         func(ctx context.Context, gameType models.GameType, players []*QueueEntry) error
	botProvider     BotProvider
	busyBotChecker  BusyBotChecker
}

// NewService creates a new matchmaking service.
func NewService(nc *natsClient.Client, onMatch func(ctx context.Context, gameType models.GameType, players []*QueueEntry) error) *Service {
	return &Service{
		queues:  make(map[models.GameType][]*QueueEntry),
		nats:    nc,
		onMatch: onMatch,
	}
}

// SetBotProvider sets the provider used for AI bot backfill.
func (s *Service) SetBotProvider(bp BotProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.botProvider = bp
}

// SetBusyBotChecker sets the checker used to determine if a bot is in an active game.
func (s *Service) SetBusyBotChecker(c BusyBotChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busyBotChecker = c
}

// Join adds an agent to the matchmaking queue.
func (s *Service) Join(gameType models.GameType, agentID string, mu, sigma float64) error {
	cfg, ok := DefaultConfigs[gameType]
	if !ok {
		return fmt.Errorf("unsupported game type: %s", gameType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already in queue
	for _, entry := range s.queues[gameType] {
		if entry.AgentID == agentID {
			return fmt.Errorf("already in queue")
		}
	}

	entry := &QueueEntry{
		AgentID:  agentID,
		Mu:       mu,
		Sigma:    sigma,
		JoinedAt: time.Now(),
	}

	s.queues[gameType] = append(s.queues[gameType], entry)
	slog.Info("agent joined queue", "agent_id", agentID, "game_type", gameType, "queue_size", len(s.queues[gameType]))

	// Try immediate match
	s.tryMatch(gameType, cfg)

	return nil
}

// Leave removes an agent from the matchmaking queue.
func (s *Service) Leave(gameType models.GameType, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue := s.queues[gameType]
	for i, entry := range queue {
		if entry.AgentID == agentID {
			s.queues[gameType] = append(queue[:i], queue[i+1:]...)
			slog.Info("agent left queue", "agent_id", agentID, "game_type", gameType)
			return nil
		}
	}
	return fmt.Errorf("not in queue")
}

// LeaveAll removes an agent from all queues.
func (s *Service) LeaveAll(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for gt, queue := range s.queues {
		for i, entry := range queue {
			if entry.AgentID == agentID {
				s.queues[gt] = append(queue[:i], queue[i+1:]...)
				break
			}
		}
	}
}

// QueueStatus returns the current queue sizes.
func (s *Service) QueueStatus() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := make(map[string]int)
	for gt, queue := range s.queues {
		status[string(gt)] = len(queue)
	}
	return status
}

// backfillWait is how long a real player must wait before AI bots are added.
const backfillWait = 30 * time.Second

// RunMatchLoop periodically attempts to form matches.
func (s *Service) RunMatchLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for gameType, cfg := range DefaultConfigs {
				s.tryMatch(gameType, cfg)
				// If there are real players waiting 30s+ but not enough, try backfill
				s.tryBackfill(ctx, gameType, cfg)
			}
			s.mu.Unlock()
		}
	}
}

// tryMatch attempts to form a match from the queue. Must be called with mu held.
func (s *Service) tryMatch(gameType models.GameType, cfg GameConfig) {
	queue := s.queues[gameType]
	if len(queue) < cfg.MinPlayers {
		return
	}

	// Try to find a compatible group using relaxed TrueSkill matching
	matched := s.findCompatibleGroup(queue, cfg)
	if matched == nil {
		return
	}

	// Remove matched players from queue
	matchedSet := make(map[string]bool)
	for _, entry := range matched {
		matchedSet[entry.AgentID] = true
	}
	remaining := make([]*QueueEntry, 0, len(queue)-len(matched))
	for _, entry := range queue {
		if !matchedSet[entry.AgentID] {
			remaining = append(remaining, entry)
		}
	}
	s.queues[gameType] = remaining

	// Execute match creation asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.onMatch(ctx, gameType, matched); err != nil {
			slog.Error("match creation failed", "error", err, "game_type", gameType)
			// Put players back in queue
			s.mu.Lock()
			s.queues[gameType] = append(s.queues[gameType], matched...)
			s.mu.Unlock()
			return
		}

		slog.Info("match created", "game_type", gameType, "players", len(matched))
	}()
}

// findCompatibleGroup finds a group of players with compatible TrueSkill ratings.
// Uses relaxed matching: the longer a player waits, the wider the acceptable range.
func (s *Service) findCompatibleGroup(queue []*QueueEntry, cfg GameConfig) []*QueueEntry {
	now := time.Now()
	n := cfg.MinPlayers

	if len(queue) < n {
		return nil
	}

	// For each potential "anchor" player, try to find n-1 compatible players
	for i, anchor := range queue {
		waitTime := now.Sub(anchor.JoinedAt)
		maxRange := skillRange(waitTime, anchor.Sigma)

		compatible := []*QueueEntry{anchor}
		for j, candidate := range queue {
			if i == j {
				continue
			}

			candidateWait := now.Sub(candidate.JoinedAt)
			candidateRange := skillRange(candidateWait, candidate.Sigma)

			// Use the wider of the two ranges
			effectiveRange := math.Max(maxRange, candidateRange)

			// Check if skills are within range
			diff := math.Abs(anchor.Mu - candidate.Mu)
			if diff <= effectiveRange {
				compatible = append(compatible, candidate)
			}

			if len(compatible) >= n {
				return compatible[:n]
			}
		}
	}

	return nil
}

// skillRange returns the acceptable skill range based on wait time.
// 0-15s: ±1σ, 15-30s: ±2σ, 30-60s: ±3σ, 60s+: unlimited
func skillRange(waitTime time.Duration, sigma float64) float64 {
	switch {
	case waitTime < 15*time.Second:
		return 1 * sigma
	case waitTime < 30*time.Second:
		return 2 * sigma
	case waitTime < 60*time.Second:
		return 3 * sigma
	default:
		return trueskill.DefaultMu * 10 // effectively unlimited
	}
}

// tryBackfill fills the queue with AI bots when real players have waited long enough.
// Must be called with mu held.
func (s *Service) tryBackfill(ctx context.Context, gameType models.GameType, cfg GameConfig) {
	if s.botProvider == nil {
		return
	}

	queue := s.queues[gameType]
	if len(queue) == 0 || len(queue) >= cfg.MinPlayers {
		return // no real players or already enough
	}

	// Check if the oldest real player has waited long enough
	now := time.Now()
	oldest := queue[0]
	if now.Sub(oldest.JoinedAt) < backfillWait {
		return
	}

	// Need this many bots
	needed := cfg.MinPlayers - len(queue)
	existingIDs := make(map[string]bool)
	for _, e := range queue {
		existingIDs[e.AgentID] = true
	}

	// Get all bot IDs (request more than needed so we have alternatives if some are busy)
	botIDs, err := s.botProvider.GetBotAgentIDs(ctx, needed+6)
	if err != nil {
		slog.Error("failed to get bot agent IDs for backfill", "error", err)
		return
	}

	// Add bot entries to queue, skipping bots already in queue or in active games
	added := 0
	for _, id := range botIDs {
		if existingIDs[id] {
			continue
		}
		if s.busyBotChecker != nil && s.busyBotChecker.IsAgentInActiveGame(ctx, id) {
			continue
		}
		queue = append(queue, &QueueEntry{
			AgentID:  id,
			Mu:       trueskill.DefaultMu,
			Sigma:    trueskill.DefaultSigma,
			JoinedAt: now,
		})
		added++
		if added >= needed {
			break
		}
	}
	s.queues[gameType] = queue

	if added > 0 {
		slog.Info("backfilled matchmaking with AI bots", "game_type", gameType, "bots_added", added, "queue_size", len(queue))
		s.tryMatch(gameType, cfg)
	}
}

// PublishMatchFound publishes a match_found notification via NATS.
func (s *Service) PublishMatchFound(gameID string, gameType models.GameType, playerIDs []string) error {
	if s.nats == nil {
		return nil
	}
	msg := natsClient.MatchFoundMsg{
		GameID:    gameID,
		GameType:  string(gameType),
		PlayerIDs: playerIDs,
	}
	data, _ := json.Marshal(msg)
	return s.nats.Conn().Publish(natsClient.SubjectMatchmaking(string(gameType)), data)
}
