// simulate: Creates test agents and plays a full game via HTTP API.
// Usage: go run ./cmd/simulate [--game=poker|werewolf]
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

const apiBase = "http://localhost:8080"

type agent struct {
	ID     string
	Name   string
	APIKey string
}

func main() {
	gameType := flag.String("game", "poker", "Game type: poker or werewolf")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[sim] ")

	switch *gameType {
	case "poker":
		runPoker()
	case "werewolf":
		runWerewolf()
	default:
		log.Fatalf("Unknown game type: %s (use poker or werewolf)", *gameType)
	}
}

func runPoker() {
	agents := createAgents(6)

	playerIDs := make([]string, 6)
	for i, a := range agents {
		playerIDs[i] = a.ID
	}
	body, _ := json.Marshal(map[string]interface{}{
		"type":       "poker",
		"player_ids": playerIDs,
		"entry_fee":  0,
	})

	resp := apiCall("POST", "/api/v1/games", body, agents[0].APIKey)
	if resp == nil {
		log.Fatal("Failed to create game")
	}
	var createResp struct {
		GameID string `json:"game_id"`
	}
	json.Unmarshal(resp, &createResp)
	gameID := createResp.GameID
	log.Printf("Poker game created: %s", gameID)

	fmt.Println()
	fmt.Printf(">>> SPECTATE: http://localhost:3000/zh/game/%s\n", gameID)
	fmt.Println(">>> Game starts in 3 seconds...")
	fmt.Println()
	time.Sleep(3 * time.Second)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for round := 0; round < 5000; round++ {
		acted := false
		for _, a := range agents {
			state := getState(gameID, a)
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

			actionJSON, _ := json.Marshal(chosenAction)
			result := submitAction(gameID, a, actionJSON)
			if result == nil {
				continue
			}

			log.Printf("%s → %v", a.Name, chosenAction["type"])
			acted = true

			if gameOver, ok := result["game_over"].(bool); ok && gameOver {
				log.Println("========== GAME OVER ==========")
				fmt.Printf("\n>>> REPLAY: http://localhost:3000/zh/game/%s/replay\n\n", gameID)
				os.Exit(0)
			}
			break
		}

		if !acted {
			time.Sleep(20 * time.Millisecond)
		} else {
			time.Sleep(150 * time.Millisecond)
		}
	}
	log.Println("Max rounds reached without game over")
}

func runWerewolf() {
	agents := createAgents(5)

	playerIDs := make([]string, 5)
	for i, a := range agents {
		playerIDs[i] = a.ID
	}
	body, _ := json.Marshal(map[string]interface{}{
		"type":       "werewolf",
		"player_ids": playerIDs,
		"entry_fee":  0,
	})

	resp := apiCall("POST", "/api/v1/games", body, agents[0].APIKey)
	if resp == nil {
		log.Fatal("Failed to create game")
	}
	var createResp struct {
		GameID string `json:"game_id"`
	}
	json.Unmarshal(resp, &createResp)
	gameID := createResp.GameID
	log.Printf("Werewolf game created: %s", gameID)

	fmt.Println()
	fmt.Printf(">>> SPECTATE: http://localhost:3000/zh/game/%s\n", gameID)
	fmt.Println(">>> Game starts in 3 seconds...")
	fmt.Println()
	time.Sleep(3 * time.Second)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	messages := []string{
		"I think we should be careful.",
		"Something feels off about this group.",
		"I have a hunch about someone...",
		"Let's discuss who acted suspicious last night.",
		"I trust my instincts on this one.",
	}

	for round := 0; round < 500; round++ {
		acted := false
		for _, a := range agents {
			state := getState(gameID, a)
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

			actionJSON, _ := json.Marshal(action)
			result := submitAction(gameID, a, actionJSON)
			if result == nil {
				continue
			}

			log.Printf("%s → %s", a.Name, actionType)
			acted = true

			if gameOver, ok := result["game_over"].(bool); ok && gameOver {
				log.Println("========== GAME OVER ==========")
				fmt.Printf("\n>>> REPLAY: http://localhost:3000/zh/game/%s/replay\n\n", gameID)
				os.Exit(0)
			}
			break
		}

		if !acted {
			time.Sleep(20 * time.Millisecond)
		} else {
			time.Sleep(150 * time.Millisecond)
		}
	}
	log.Println("Max rounds reached without game over")
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

func createAgents(count int) []agent {
	agents := make([]agent, count)
	for i := range agents {
		id := uuid.New().String()
		key := fmt.Sprintf("moltgame_sk_sim_%d_%d", i, time.Now().UnixNano())
		hash := sha256.Sum256([]byte(key))
		agents[i] = agent{
			ID:     id,
			Name:   fmt.Sprintf("sim-bot-%d", i),
			APIKey: key,
		}
		claimToken := fmt.Sprintf("sim_claim_%d", i)
		verifyCode := fmt.Sprintf("SIM%d", i)
		q := fmt.Sprintf(
			`INSERT INTO agents (id, name, description, api_key_hash, claim_token, verification_code, status, is_claimed, chakra_balance, trueskill_mu, trueskill_sigma) VALUES ('%s', '%s', 'Simulation bot %d', '%s', '%s', '%s', 'active', true, 2000, 25.0, 8.333) ON CONFLICT (name) DO UPDATE SET api_key_hash = EXCLUDED.api_key_hash, chakra_balance = 2000`,
			id, agents[i].Name, i, hex.EncodeToString(hash[:]), claimToken, verifyCode,
		)
		idQ := fmt.Sprintf(`SELECT id FROM agents WHERE name = '%s'`, agents[i].Name)
		execSQL(q)
		actualID := readSQL(idQ)
		if actualID != "" {
			agents[i].ID = actualID
		}
		log.Printf("Agent %d: %s (id=%s)", i, agents[i].Name, agents[i].ID[:8])
	}
	return agents
}

func getState(gameID string, a agent) map[string]interface{} {
	resp := apiCall("GET", fmt.Sprintf("/api/v1/games/%s/state", gameID), nil, a.APIKey)
	if resp == nil {
		return nil
	}
	var state map[string]interface{}
	json.Unmarshal(resp, &state)
	return state
}

func submitAction(gameID string, a agent, action []byte) map[string]interface{} {
	body, _ := json.Marshal(map[string]json.RawMessage{"action": action})
	resp := apiCall("POST", fmt.Sprintf("/api/v1/games/%s/action", gameID), body, a.APIKey)
	if resp == nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(resp, &result)
	return result
}

func apiCall(method, path string, body []byte, apiKey string) []byte {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, _ := http.NewRequest(method, apiBase+path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode != 403 && resp.StatusCode != 404 {
			log.Printf("API %d %s %s: %s", resp.StatusCode, method, path, string(data[:min(len(data), 200)]))
		}
		return nil
	}
	return data
}

func execSQL(query string) {
	cmd := exec.Command("docker", "exec", "moltgame-postgres", "psql", "-U", "moltgame", "-d", "moltgame", "-c", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("SQL error: %v\n%s", err, string(out))
	}
}

func readSQL(query string) string {
	cmd := exec.Command("docker", "exec", "moltgame-postgres", "psql", "-U", "moltgame", "-d", "moltgame", "-t", "-A", "-c", query)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
