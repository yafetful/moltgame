//go:build e2e

package e2e

import (
	"math/rand"
	"testing"
	"time"
)

func TestPokerFullGame(t *testing.T) {
	agents := createTestAgents(t, "poker", 6, 2000)
	defer cleanupTestAgents(t, agents)

	gameID := createGame(t, "poker", agents, 0)
	t.Logf("Poker game created: %s", gameID)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Play aggressively to finish quickly
	for round := 0; round < 5000; round++ {
		acted := false
		for _, a := range agents {
			state := getState(t, gameID, a)
			if state == nil {
				continue
			}

			rawActions, ok := state["valid_actions"]
			if !ok || rawActions == nil {
				continue
			}
			actions, ok := rawActions.([]interface{})
			if !ok || len(actions) == 0 {
				continue
			}

			// Aggressive bot: raise often, sometimes call/fold
			var chosenAction map[string]interface{}
			roll := rng.Float64()

			hasCall, hasCheck, hasRaise := false, false, false
			var raiseMin, raiseMax int

			for _, raw := range actions {
				act := raw.(map[string]interface{})
				switch act["type"].(string) {
				case "call":
					hasCall = true
				case "check":
					hasCheck = true
				case "raise":
					hasRaise = true
					raiseMin = int(act["min_amount"].(float64))
					raiseMax = int(act["max_amount"].(float64))
				}
			}

			switch {
			case hasRaise && roll > 0.5:
				amt := raiseMax
				if rng.Float64() > 0.3 {
					amt = raiseMin + (raiseMax-raiseMin)/2
				}
				chosenAction = map[string]interface{}{"type": "raise", "amount": amt}
			case hasCheck:
				chosenAction = map[string]interface{}{"type": "check"}
			case roll < 0.2 && !hasCheck:
				chosenAction = map[string]interface{}{"type": "fold"}
			case hasCall:
				chosenAction = map[string]interface{}{"type": "call"}
			default:
				first := actions[0].(map[string]interface{})
				chosenAction = map[string]interface{}{"type": first["type"]}
			}

			result := submitAction(t, gameID, a, chosenAction)
			if result == nil {
				continue
			}
			acted = true

			if gameOver, ok := result["game_over"].(bool); ok && gameOver {
				t.Log("Game over reached")
				assertPokerComplete(t, gameID, agents)
				return
			}
			break
		}

		if !acted {
			time.Sleep(20 * time.Millisecond)
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	t.Fatal("Poker game did not finish within 5000 rounds")
}

func assertPokerComplete(t *testing.T, gameID string, agents []testAgent) {
	t.Helper()

	// Game status should be finished
	status := readSQL(t, "SELECT status FROM games WHERE id = '"+gameID+"'")
	if status != "finished" {
		t.Errorf("Expected game status 'finished', got '%s'", status)
	}

	// Winner should be set
	winnerID := readSQL(t, "SELECT winner_id FROM games WHERE id = '"+gameID+"'")
	if winnerID == "" {
		t.Error("Expected winner_id to be set")
	}

	// Game events should be persisted
	eventCount := readSQL(t, "SELECT COUNT(*) FROM game_events WHERE game_id = '"+gameID+"'")
	if eventCount == "0" || eventCount == "" {
		t.Error("Expected game_events to be persisted")
	}
	t.Logf("Game events: %s", eventCount)

	// game_players.final_rank should be set for at least the winner
	rankCount := readSQL(t, "SELECT COUNT(*) FROM game_players WHERE game_id = '"+gameID+"' AND final_rank IS NOT NULL")
	if rankCount == "0" || rankCount == "" {
		t.Error("Expected final_rank to be set for players")
	}

	// TrueSkill should be updated
	muAfter := readSQL(t, "SELECT COUNT(*) FROM game_players WHERE game_id = '"+gameID+"' AND mu_after IS NOT NULL")
	if muAfter == "0" || muAfter == "" {
		t.Error("Expected mu_after (TrueSkill) to be updated")
	}

	// GET /events should return replay data
	events := getEvents(t, gameID)
	if len(events) == 0 {
		t.Error("Expected /events endpoint to return replay data")
	}
	t.Logf("Replay events via API: %d", len(events))
}
