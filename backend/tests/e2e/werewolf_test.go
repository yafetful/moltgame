//go:build e2e

package e2e

import (
	"math/rand"
	"testing"
	"time"
)

func TestWerewolfFullGame(t *testing.T) {
	agents := createTestAgents(t, "ww", 5, 2000)
	defer cleanupTestAgents(t, agents)

	gameID := createGame(t, "werewolf", agents, 0)
	t.Logf("Werewolf game created: %s", gameID)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	messages := []string{
		"I think we should be careful.",
		"I have a bad feeling about this.",
		"Let's discuss who seems suspicious.",
		"I trust my instincts here.",
		"Something doesn't add up.",
	}

	// Play through the game responding to action_required
	for round := 0; round < 500; round++ {
		acted := false
		for _, a := range agents {
			state := getState(t, gameID, a)
			if state == nil {
				continue
			}

			rawAR, ok := state["action_required"]
			if !ok || rawAR == nil {
				continue
			}
			ar, ok := rawAR.(map[string]interface{})
			if !ok {
				continue
			}

			actionType, _ := ar["type"].(string)
			if actionType == "" {
				continue
			}

			var action map[string]interface{}

			switch actionType {
			case "kill", "investigate":
				targets := extractTargets(ar)
				if len(targets) == 0 {
					continue
				}
				target := targets[rng.Intn(len(targets))]
				action = map[string]interface{}{"type": actionType, "target_id": target}

			case "speak":
				msg := messages[rng.Intn(len(messages))]
				action = map[string]interface{}{"type": "speak", "message": msg}

			case "vote":
				targets := extractTargets(ar)
				if len(targets) == 0 {
					action = map[string]interface{}{"type": "skip"}
				} else {
					target := targets[rng.Intn(len(targets))]
					action = map[string]interface{}{"type": "vote", "target_id": target}
				}

			default:
				action = map[string]interface{}{"type": actionType}
			}

			result := submitAction(t, gameID, a, action)
			if result == nil {
				continue
			}
			acted = true

			if gameOver, ok := result["game_over"].(bool); ok && gameOver {
				t.Log("Game over reached")
				assertWerewolfComplete(t, gameID)
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
	t.Fatal("Werewolf game did not finish within 500 rounds")
}

func extractTargets(ar map[string]interface{}) []string {
	rawTargets, ok := ar["valid_targets"]
	if !ok || rawTargets == nil {
		return nil
	}
	arr, ok := rawTargets.([]interface{})
	if !ok {
		return nil
	}
	targets := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			targets = append(targets, s)
		}
	}
	return targets
}

func assertWerewolfComplete(t *testing.T, gameID string) {
	t.Helper()

	// Game status should be finished
	status := readSQL(t, "SELECT status FROM games WHERE id = '"+gameID+"'")
	if status != "finished" {
		t.Errorf("Expected game status 'finished', got '%s'", status)
	}

	// Game events should be persisted
	eventCount := readSQL(t, "SELECT COUNT(*) FROM game_events WHERE game_id = '"+gameID+"'")
	if eventCount == "0" || eventCount == "" {
		t.Error("Expected game_events to be persisted")
	}
	t.Logf("Game events: %s", eventCount)

	// game_players.final_rank should be set
	rankCount := readSQL(t, "SELECT COUNT(*) FROM game_players WHERE game_id = '"+gameID+"' AND final_rank IS NOT NULL")
	if rankCount == "0" || rankCount == "" {
		t.Error("Expected final_rank to be set for players")
	}
}
