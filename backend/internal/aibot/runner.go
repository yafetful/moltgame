package aibot

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moltgame/backend/internal/auth"
	gameRepo "github.com/moltgame/backend/internal/game"
	"github.com/moltgame/backend/internal/models"
	natsClient "github.com/moltgame/backend/internal/nats"
	"github.com/nats-io/nats.go"
)

const (
	natsTimeout = 5 * time.Second
	entryFee    = 0 // AI games are free
)

// AgentConfig defines a single AI agent's configuration.
type AgentConfig struct {
	Name  string
	Model string // OpenRouter model ID
}

// Runner manages AI-driven poker games.
type Runner struct {
	nc         *natsClient.Client
	agentRepo  *auth.AgentRepository
	gameRepo   *gameRepo.Repository
	settlement *gameRepo.SettlementService
	orAPIKey   string
	agents     []AgentConfig

	mu          sync.Mutex
	running     bool
	currentGame string
	cancelFunc  context.CancelFunc
}

// NewRunner creates a new AI game runner.
func NewRunner(
	nc *natsClient.Client,
	agentRepo *auth.AgentRepository,
	gameRepository *gameRepo.Repository,
	settlement *gameRepo.SettlementService,
	orAPIKey string,
	agents []AgentConfig,
) *Runner {
	return &Runner{
		nc:        nc,
		agentRepo: agentRepo,
		gameRepo:  gameRepository,
		settlement: settlement,
		orAPIKey:  orAPIKey,
		agents:    agents,
	}
}

// GetBotAgentIDs returns up to n house bot agent IDs, ensuring they exist in DB.
func (r *Runner) GetBotAgentIDs(ctx context.Context, n int) ([]string, error) {
	ids, _, err := r.ensureAgents(ctx)
	if err != nil {
		return nil, err
	}
	if n > len(ids) {
		n = len(ids)
	}
	return ids[:n], nil
}

// IsBotAgent returns true if the given agent ID belongs to a house bot.
func (r *Runner) IsBotAgent(ctx context.Context, agentID string) bool {
	ids, _, err := r.ensureAgents(ctx)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == agentID {
			return true
		}
	}
	return false
}

// IsRunning returns whether an AI game is currently in progress.
func (r *Runner) IsRunning() (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running, r.currentGame
}

// StartGame triggers a new AI poker game. Returns the game ID immediately.
func (r *Runner) StartGame(ctx context.Context) (string, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return "", fmt.Errorf("AI game already in progress: %s", r.currentGame)
	}
	r.running = true
	r.mu.Unlock()

	// Ensure all AI agents exist in DB
	agentIDs, nameByID, err := r.ensureAgents(ctx)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return "", fmt.Errorf("ensure agents: %w", err)
	}

	// Create game in DB
	config, _ := json.Marshal(map[string]interface{}{
		"entry_fee": entryFee,
		"ai_game":   true,
	})
	dbGame, err := r.gameRepo.CreateGame(ctx, models.GameTypePoker, agentIDs, config)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return "", fmt.Errorf("create game: %w", err)
	}

	// Create room via NATS
	var resp natsClient.CreateRoomResponse
	err = r.nc.RequestJSON(natsClient.SubjectPokerRoomCreate, natsClient.CreateRoomRequest{
		GameID:      dbGame.ID,
		PlayerIDs:   agentIDs,
		PlayerNames: nameByID,
		Seed:        cryptoSeed(),
		EntryFee:    entryFee,
	}, &resp, natsTimeout)
	if err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return "", fmt.Errorf("create NATS room: %w", err)
	}
	if !resp.Success {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return "", fmt.Errorf("poker engine: %s", resp.Error)
	}

	// Build agent lookup: agentID → model
	modelByID := make(map[string]string)
	for i, id := range agentIDs {
		modelByID[id] = r.agents[i].Model
	}

	r.mu.Lock()
	r.currentGame = dbGame.ID
	r.mu.Unlock()

	// Launch background game loop
	gameCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancelFunc = cancel
	r.mu.Unlock()

	go r.runGame(gameCtx, dbGame.ID, agentIDs, nameByID, modelByID)

	slog.Info("AI game started", "game_id", dbGame.ID, "players", len(agentIDs))
	return dbGame.ID, nil
}

// RunBotsForGame starts a background loop to drive house bots in an existing game.
// botIDs is the subset of agent IDs in this game that are house bots.
func (r *Runner) RunBotsForGame(gameID string, botIDs []string, nameByID map[string]string) {
	if len(botIDs) == 0 {
		return
	}

	modelByID := make(map[string]string)
	for _, id := range botIDs {
		modelByID[id] = r.agents[0].Model // all bots use the same model
	}

	go r.runGame(context.Background(), gameID, botIDs, nameByID, modelByID)
}

// ensureAgents creates or looks up the 6 house AI agents.
func (r *Runner) ensureAgents(ctx context.Context) ([]string, map[string]string, error) {
	ids := make([]string, len(r.agents))
	nameByID := make(map[string]string)

	for i, ac := range r.agents {
		agent, err := r.agentRepo.GetAgentByName(ctx, ac.Name)
		if err != nil {
			if !errors.Is(err, auth.ErrAgentNotFound) {
				return nil, nil, fmt.Errorf("lookup agent %s: %w", ac.Name, err)
			}
			// Create new agent
			keyHash := hashString(fmt.Sprintf("aibot-%s-%d", ac.Name, time.Now().UnixNano()))
			agent, err = r.agentRepo.CreateAgent(ctx, ac.Name, ac.Model,
				fmt.Sprintf("AI agent powered by %s", ac.Model),
				"", keyHash, "", "")
			if err != nil {
				// Might be a race — retry lookup
				if errors.Is(err, auth.ErrNameTaken) {
					agent, err = r.agentRepo.GetAgentByName(ctx, ac.Name)
					if err != nil {
						return nil, nil, fmt.Errorf("retry lookup agent %s: %w", ac.Name, err)
					}
				} else {
					return nil, nil, fmt.Errorf("create agent %s: %w", ac.Name, err)
				}
			} else {
				slog.Info("AI agent created", "name", ac.Name, "id", agent.ID)
			}
		}
		ids[i] = agent.ID
		nameByID[agent.ID] = ac.Name
	}

	return ids, nameByID, nil
}

// runGame is the main background loop that drives all AI agents for one game.
func (r *Runner) runGame(ctx context.Context, gameID string, agentIDs []string, nameByID, modelByID map[string]string) {
	defer func() {
		r.mu.Lock()
		r.running = false
		r.currentGame = ""
		r.cancelFunc = nil
		r.mu.Unlock()
		slog.Info("AI game loop ended", "game_id", gameID)
	}()

	// Channel for turn notifications
	turnCh := make(chan natsClient.TurnNotifyEvent, 16)
	doneCh := make(chan struct{})

	// Subscribe to turn notifications
	turnSub, err := r.nc.Subscribe(natsClient.SubjectPokerTurnNotify(gameID), func(msg *nats.Msg) {
		var evt natsClient.TurnNotifyEvent
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			return
		}
		select {
		case turnCh <- evt:
		default:
			// Drop if channel full (shouldn't happen)
		}
	})
	if err != nil {
		slog.Error("failed to subscribe to turn notifications", "game_id", gameID, "error", err)
		return
	}
	defer turnSub.Unsubscribe()

	// Subscribe to game over
	gameOverSub, err := r.nc.Subscribe(natsClient.SubjectPokerGameOver(gameID), func(msg *nats.Msg) {
		select {
		case <-doneCh:
		default:
			close(doneCh)
		}
	})
	if err != nil {
		slog.Error("failed to subscribe to game over", "game_id", gameID, "error", err)
		return
	}
	defer gameOverSub.Unsubscribe()

	slog.Info("AI game loop started", "game_id", gameID)

	// Kickstart: the first turn notification is never published by room creation,
	// so we need to find the current actor and inject a synthetic turn event.
	r.kickstartFirstTurn(gameID, agentIDs, turnCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			slog.Info("AI game over detected", "game_id", gameID)
			return
		case evt := <-turnCh:
			// Check if this agent is one of ours
			model, ok := modelByID[evt.AgentID]
			if !ok {
				continue
			}
			agentName := nameByID[evt.AgentID]

			// Get game state for this agent via NATS
			state, err := r.getState(gameID, evt.AgentID)
			if err != nil {
				slog.Warn("failed to get state for AI", "game_id", gameID, "agent", agentName, "error", err)
				continue
			}

			// Check for valid_actions
			if rawActions, ok := state["valid_actions"]; !ok || rawActions == nil {
				continue
			}

			// Call AI for decision
			decision := callAI(r.orAPIKey, model, evt.AgentID, state, nameByID)

			// Include reason
			if decision.Reason != "" {
				decision.Action["reason"] = decision.Reason
			}

			// Log
			phase, _ := state["phase"].(string)
			handNum := 0
			if h, ok := state["hand_num"].(float64); ok {
				handNum = int(h)
			}
			actionType, _ := decision.Action["type"].(string)
			amountStr := ""
			if amt, ok := decision.Action["amount"]; ok {
				amountStr = fmt.Sprintf(" %v", amt)
			}
			slog.Info("AI decision",
				"game_id", gameID, "hand", handNum, "phase", phase,
				"agent", agentName, "action", actionType+amountStr,
				"reason", decision.Reason)

			// Submit action via NATS
			if err := r.submitAction(gameID, evt.AgentID, decision.Action); err != nil {
				slog.Warn("AI action submit failed", "game_id", gameID, "agent", agentName, "error", err)
			}
		}
	}
}

// kickstartFirstTurn queries each agent's state to find who has valid_actions
// and injects a synthetic turn event into turnCh.
func (r *Runner) kickstartFirstTurn(gameID string, agentIDs []string, turnCh chan natsClient.TurnNotifyEvent) {
	for _, id := range agentIDs {
		state, err := r.getState(gameID, id)
		if err != nil {
			continue
		}
		if rawActions, ok := state["valid_actions"]; ok && rawActions != nil {
			if actions, ok := rawActions.([]interface{}); ok && len(actions) > 0 {
				turnCh <- natsClient.TurnNotifyEvent{GameID: gameID, AgentID: id}
				return
			}
		}
	}
}

// getState fetches the game state for a specific agent via NATS.
func (r *Runner) getState(gameID, agentID string) (map[string]interface{}, error) {
	var resp natsClient.StateResponse
	err := r.nc.RequestJSON(
		natsClient.SubjectPokerRoomState(gameID),
		natsClient.StateRequest{AgentID: agentID},
		&resp,
		natsTimeout,
	)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("state query: %s", resp.Error)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(resp.State, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	return state, nil
}

// submitAction submits a player action via NATS.
func (r *Runner) submitAction(gameID, agentID string, action map[string]interface{}) error {
	actionJSON, err := json.Marshal(action)
	if err != nil {
		return err
	}

	var resp natsClient.ActionResponse
	err = r.nc.RequestJSON(
		natsClient.SubjectPokerRoomAction(gameID),
		natsClient.ActionRequest{
			AgentID: agentID,
			Action:  actionJSON,
		},
		&resp,
		natsTimeout,
	)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("action: %s", resp.Error)
	}
	return nil
}

// --- Helpers ---

func cryptoSeed() int64 {
	var b [8]byte
	rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]))
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
