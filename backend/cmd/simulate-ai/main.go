// simulate-ai: 6 AI agents (different OpenRouter models) play a full poker tournament.
// Each agent runs as an independent goroutine, polling game state via REST API.
// Reads OPENROUTER_API_KEY and MODEL_ID_1..6 from .env.
// Usage: go run ./cmd/simulate-ai
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

const (
	apiBase       = "http://localhost:8080"
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	aiTimeout     = 30 * time.Second
	waitTimeout   = 30 // seconds, for long-polling /agent/wait
)

type aiAgent struct {
	ID     string
	Name   string
	APIKey string
	Model  string // OpenRouter model ID
}

// OpenRouter structured output schema for poker actions
var pokerActionSchema = map[string]interface{}{
	"type": "json_schema",
	"json_schema": map[string]interface{}{
		"name":   "poker_action",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "The poker action to take. Must be one of: fold, check, call, raise, allin",
				},
				"amount": map[string]interface{}{
					"type":        "number",
					"description": "Bet amount, only required for raise action. Must be between min and max allowed.",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Very short reason, max 10 words.",
				},
			},
			"required":             []string{"action", "amount", "reason"},
			"additionalProperties": false,
		},
	},
}

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[ai-sim] ")

	// Load .env from multiple possible locations
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	orKey := os.Getenv("OPENROUTER_API_KEY")
	if orKey == "" {
		log.Fatal("OPENROUTER_API_KEY not set in .env")
	}

	// Define 6 agents with their models
	agentDefs := []struct {
		envKey string
		name   string
	}{
		{"MODEL_ID_1", "seed-16"},
		{"MODEL_ID_2", "gemini-flash"},
		{"MODEL_ID_3", "gpt-52-chat"},
		{"MODEL_ID_4", "deepseek-v3"},
		{"MODEL_ID_5", "grok-fast"},
		{"MODEL_ID_6", "claude-sonnet"},
	}

	agents := make([]aiAgent, 6)
	for i, def := range agentDefs {
		modelID := os.Getenv(def.envKey)
		if modelID == "" {
			log.Fatalf("%s not set in .env", def.envKey)
		}
		agents[i] = aiAgent{Name: def.name, Model: modelID}
	}

	// Step 1: Clear database
	log.Println("Clearing database...")
	execSQL("TRUNCATE chakra_transactions, game_events, game_players, games, agents CASCADE;")
	log.Println("Database cleared")

	// Step 2: Register agents via API
	log.Println("Registering agents via API...")
	for i := range agents {
		registerAgentViaAPI(&agents[i])
	}
	fmt.Println()

	// Build name lookup
	nameByID := map[string]string{}
	for _, a := range agents {
		nameByID[a.ID] = a.Name
	}

	// Step 3: Create game via API
	playerIDs := make([]string, 6)
	for i, a := range agents {
		playerIDs[i] = a.ID
	}
	body, _ := json.Marshal(map[string]interface{}{
		"type":       "poker",
		"player_ids": playerIDs,
		"entry_fee":  100,
	})
	resp := apiCall("POST", "/api/v1/games", body, agents[0].APIKey)
	if resp == nil {
		log.Fatal("Failed to create game — is api-gateway running?")
	}
	var createResp struct {
		GameID string `json:"game_id"`
	}
	json.Unmarshal(resp, &createResp)
	gameID := createResp.GameID
	log.Printf("Game created: %s", gameID)

	fmt.Println()
	fmt.Printf(">>> SPECTATE: http://localhost:3000/zh/game/%s\n", gameID)
	fmt.Println(">>> Game starts in 3 seconds...")
	fmt.Println()
	time.Sleep(3 * time.Second)

	// Step 4: Launch 6 independent agent goroutines
	var wg sync.WaitGroup
	done := make(chan struct{}) // closed when game is over

	for i := range agents {
		wg.Add(1)
		go func(a aiAgent) {
			defer wg.Done()
			runAgent(a, gameID, orKey, nameByID, done)
		}(agents[i])
	}

	// Wait for game to finish (any goroutine detects game_over and closes done)
	<-done
	time.Sleep(2 * time.Second) // let other goroutines notice

	fmt.Println()
	log.Println("========== GAME OVER ==========")
	fmt.Printf("\n>>> REPLAY: http://localhost:3000/zh/game/%s/replay\n\n", gameID)
	printFinalResults(gameID)
}

// runAgent is the main loop for a single AI agent goroutine.
// Uses long-polling via GET /api/v1/agent/wait to efficiently wait for turns.
func runAgent(a aiAgent, gameID, orKey string, nameByID map[string]string, done chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
		}

		// Long-poll: wait for our turn or game over
		event, state := waitForTurn(a, done)

		switch event {
		case "game_over":
			closeOnce(done)
			return
		case "your_turn":
			// proceed below
		default:
			// timeout (204) or error — retry
			continue
		}

		if state == nil {
			continue
		}

		// Verify state has valid_actions (double-check)
		rawActions, ok := state["valid_actions"]
		if !ok || rawActions == nil {
			continue
		}
		actions, ok := rawActions.([]interface{})
		if !ok || len(actions) == 0 {
			continue
		}
		_ = actions

		// It's our turn — call AI
		decision := callAI(orKey, a, state, nameByID)

		// Include reason in the action payload
		if decision.Reason != "" {
			decision.Action["reason"] = decision.Reason
		}

		// Log decision before submit
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

		// Submit action
		actionJSON, _ := json.Marshal(decision.Action)
		result := submitAction(gameID, a, actionJSON)
		if result == nil {
			// Submit failed (likely "not your turn" — turn moved while AI was thinking).
			// Just go back to waitForTurn instead of trying stale fallback.
			log.Printf("  [%s] submit failed, returning to wait", a.Name)
			continue
		}

		fmt.Printf("[Hand %d | %-8s] %-15s → %s%s\n", handNum, phase, a.Name, actionType, amountStr)
		if decision.Reason != "" {
			fmt.Printf("  reason: %q\n", decision.Reason)
		}

		// Check game over
		if gameOver, ok := result["game_over"].(bool); ok && gameOver {
			closeOnce(done)
			return
		}
	}
}

// waitForTurn calls GET /api/v1/agent/wait for long-polling turn notification.
// Returns event type ("your_turn", "game_over", or "") and parsed state (if your_turn).
func waitForTurn(a aiAgent, done chan struct{}) (string, map[string]interface{}) {
	url := fmt.Sprintf("%s/api/v1/agent/wait?timeout=%d", apiBase, waitTimeout)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	client := &http.Client{Timeout: time.Duration(waitTimeout+5) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Check if game ended while we were waiting
		select {
		case <-done:
			return "game_over", nil
		default:
		}
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return "", nil // timeout, retry
	}

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	var waitResp struct {
		Event  string          `json:"event"`
		GameID string          `json:"game_id"`
		State  json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(data, &waitResp); err != nil {
		return "", nil
	}

	if waitResp.Event == "game_over" {
		return "game_over", nil
	}

	if waitResp.Event == "your_turn" && waitResp.State != nil {
		var state map[string]interface{}
		if err := json.Unmarshal(waitResp.State, &state); err != nil {
			return "your_turn", nil
		}
		return "your_turn", state
	}

	return waitResp.Event, nil
}

var doneOnce sync.Once

func closeOnce(ch chan struct{}) {
	doneOnce.Do(func() { close(ch) })
}

// --- AI Decision ---

type aiDecision struct {
	Action map[string]interface{}
	Reason string
}

const systemPrompt = `You are playing Texas Hold'em poker tournament. Analyze the game state and choose your action.

Rules:
- "action" must be exactly one of the valid action types listed below
- "amount" is the bet size for "raise" only (between min and max). For other actions set amount to 0
- "reason" max 10 words
- Consider pot odds, position, hand strength, and opponent behavior`

func callAI(orKey string, a aiAgent, state map[string]interface{}, nameByID map[string]string) aiDecision {
	userPrompt := formatStatePrompt(a, state, nameByID)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": a.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":     0.7,
		"max_tokens":      500,
		"response_format": pokerActionSchema,
	})

	req, _ := http.NewRequest("POST", openRouterURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+orKey)

	client := &http.Client{Timeout: aiTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  [%s] OpenRouter request failed: %v", a.Name, err)
		return fallbackDecision(state)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("  [%s] OpenRouter %d: %s", a.Name, resp.StatusCode, string(data[:min(len(data), 200)]))
		return fallbackDecision(state)
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &orResp); err != nil || len(orResp.Choices) == 0 {
		log.Printf("  [%s] failed to parse OpenRouter response", a.Name)
		return fallbackDecision(state)
	}

	content := strings.TrimSpace(orResp.Choices[0].Message.Content)
	if content == "" {
		log.Printf("  [%s] OpenRouter returned empty content", a.Name)
		return fallbackDecision(state)
	}
	return parseAIResponse(content, state, a.Name)
}

func parseAIResponse(content string, state map[string]interface{}, agentName string) aiDecision {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// Try extracting JSON from markdown code block
		re := regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)\\s*```")
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			if err2 := json.Unmarshal([]byte(matches[1]), &parsed); err2 != nil {
				log.Printf("  [%s] AI response not valid JSON: %.100s", agentName, content)
				return fallbackDecision(state)
			}
		} else {
			log.Printf("  [%s] AI response not valid JSON: %.100s", agentName, content)
			return fallbackDecision(state)
		}
	}

	actionType, _ := parsed["action"].(string)
	reason, _ := parsed["reason"].(string)

	if actionType == "" {
		log.Printf("  [%s] AI returned no action type", agentName)
		return fallbackDecision(state)
	}

	// Validate action
	validActions := extractValidActions(state)
	valid := false
	for _, va := range validActions {
		if va["type"] == actionType {
			valid = true
			break
		}
	}
	if !valid {
		log.Printf("  [%s] AI chose invalid action %q, falling back", agentName, actionType)
		return fallbackDecision(state)
	}

	action := map[string]interface{}{"type": actionType}
	if actionType == "raise" {
		if amt, ok := parsed["amount"].(float64); ok {
			for _, va := range validActions {
				if va["type"] == "raise" {
					minAmt, _ := va["min_amount"].(float64)
					maxAmt, _ := va["max_amount"].(float64)
					if amt < minAmt {
						amt = minAmt
					}
					if amt > maxAmt {
						amt = maxAmt
					}
					break
				}
			}
			action["amount"] = int(amt)
		} else {
			for _, va := range validActions {
				if va["type"] == "raise" {
					action["amount"] = int(va["min_amount"].(float64))
					break
				}
			}
		}
	}

	return aiDecision{Action: action, Reason: reason}
}

func fallbackDecision(state map[string]interface{}) aiDecision {
	actions := extractValidActions(state)
	fallback := buildFallbackFromParsed(actions)
	return aiDecision{Action: fallback, Reason: "(AI fallback)"}
}

func buildFallback(actions []interface{}) map[string]interface{} {
	for _, raw := range actions {
		act := raw.(map[string]interface{})
		if act["type"] == "check" {
			return map[string]interface{}{"type": "check"}
		}
	}
	for _, raw := range actions {
		act := raw.(map[string]interface{})
		if act["type"] == "call" {
			return map[string]interface{}{"type": "call"}
		}
	}
	return map[string]interface{}{"type": "fold"}
}

func buildFallbackFromParsed(actions []map[string]interface{}) map[string]interface{} {
	for _, act := range actions {
		if act["type"] == "check" {
			return map[string]interface{}{"type": "check"}
		}
	}
	for _, act := range actions {
		if act["type"] == "call" {
			return map[string]interface{}{"type": "call"}
		}
	}
	return map[string]interface{}{"type": "fold"}
}

func extractValidActions(state map[string]interface{}) []map[string]interface{} {
	rawActions, ok := state["valid_actions"]
	if !ok || rawActions == nil {
		return nil
	}
	arr, ok := rawActions.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(arr))
	for _, raw := range arr {
		if m, ok := raw.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// --- Prompt Formatting ---

func formatStatePrompt(a aiAgent, state map[string]interface{}, nameByID map[string]string) string {
	var sb strings.Builder

	phase, _ := state["phase"].(string)
	handNum := 0
	if h, ok := state["hand_num"].(float64); ok {
		handNum = int(h)
	}
	sb.WriteString(fmt.Sprintf("Hand #%d, Phase: %s\n", handNum, phase))

	// Hole cards
	if players, ok := state["players"].([]interface{}); ok {
		for _, raw := range players {
			p := raw.(map[string]interface{})
			if p["id"] == a.ID {
				if hole, ok := p["hole"].([]interface{}); ok && len(hole) > 0 {
					cards := make([]string, len(hole))
					for i, c := range hole {
						cards[i] = fmt.Sprintf("%v", c)
					}
					sb.WriteString(fmt.Sprintf("Your cards: %s\n", strings.Join(cards, ", ")))
				}
				break
			}
		}
	}

	// Community cards
	if community, ok := state["community"].([]interface{}); ok && len(community) > 0 {
		cards := make([]string, len(community))
		for i, c := range community {
			cards[i] = fmt.Sprintf("%v", c)
		}
		sb.WriteString(fmt.Sprintf("Community: %s\n", strings.Join(cards, ", ")))
	}

	// Pot
	if pots, ok := state["pots"].([]interface{}); ok {
		totalPot := 0
		for _, raw := range pots {
			pot := raw.(map[string]interface{})
			if amt, ok := pot["amount"].(float64); ok {
				totalPot += int(amt)
			}
		}
		sb.WriteString(fmt.Sprintf("Pot: %d\n", totalPot))
	}

	// Blinds
	if smallBlind, ok := state["small_blind"].(float64); ok {
		bigBlind, _ := state["big_blind"].(float64)
		sb.WriteString(fmt.Sprintf("Blinds: %d/%d\n", int(smallBlind), int(bigBlind)))
	}

	// Current bet
	if cb, ok := state["current_bet"].(float64); ok {
		sb.WriteString(fmt.Sprintf("Current bet: %d\n", int(cb)))
	}

	// Players
	sb.WriteString("\nPlayers:\n")
	if players, ok := state["players"].([]interface{}); ok {
		for _, raw := range players {
			p := raw.(map[string]interface{})
			id, _ := p["id"].(string)
			name := nameByID[id]
			if name == "" {
				name = id[:8]
			}
			chips := 0
			if c, ok := p["chips"].(float64); ok {
				chips = int(c)
			}
			bet := 0
			if b, ok := p["bet"].(float64); ok {
				bet = int(b)
			}
			folded, _ := p["folded"].(bool)
			allIn, _ := p["all_in"].(bool)
			eliminated, _ := p["eliminated"].(bool)
			disconnected, _ := p["disconnected"].(bool)

			marker := ""
			if id == a.ID {
				marker = " (YOU)"
			}
			status := ""
			if eliminated {
				status = " [eliminated]"
			} else if disconnected {
				status = " [disconnected]"
			} else if folded {
				status = " [folded]"
			} else if allIn {
				status = " [all-in]"
			}

			sb.WriteString(fmt.Sprintf("- %s%s: %d chips, bet %d%s\n", name, marker, chips, bet, status))
		}
	}

	// Valid actions
	sb.WriteString("\nValid actions:\n")
	if actions, ok := state["valid_actions"].([]interface{}); ok {
		for _, raw := range actions {
			act := raw.(map[string]interface{})
			actType, _ := act["type"].(string)
			switch actType {
			case "fold":
				sb.WriteString("- fold\n")
			case "check":
				sb.WriteString("- check\n")
			case "call":
				cost := 0
				if c, ok := act["call_cost"].(float64); ok {
					cost = int(c)
				}
				sb.WriteString(fmt.Sprintf("- call (cost: %d)\n", cost))
			case "raise":
				minAmt, maxAmt := 0, 0
				if m, ok := act["min_amount"].(float64); ok {
					minAmt = int(m)
				}
				if m, ok := act["max_amount"].(float64); ok {
					maxAmt = int(m)
				}
				sb.WriteString(fmt.Sprintf("- raise (min: %d, max: %d)\n", minAmt, maxAmt))
			case "allin":
				amt := 0
				if m, ok := act["min_amount"].(float64); ok {
					amt = int(m)
				}
				sb.WriteString(fmt.Sprintf("- allin (amount: %d)\n", amt))
			}
		}
	}

	return sb.String()
}

// --- Agent Registration via API ---

func registerAgentViaAPI(a *aiAgent) {
	body, _ := json.Marshal(map[string]string{
		"name":        a.Name,
		"description": fmt.Sprintf("AI agent powered by %s", a.Model),
	})

	resp := apiCallNoAuth("POST", "/api/v1/agents/register", body)
	if resp == nil {
		log.Fatalf("Failed to register agent %s — is api-gateway running?", a.Name)
	}

	var regResp struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(resp, &regResp); err != nil {
		log.Fatalf("Failed to parse register response for %s: %v", a.Name, err)
	}

	a.ID = regResp.ID
	a.APIKey = regResp.APIKey
	log.Printf("Agent: %-15s  model: %-35s  id: %s", a.Name, a.Model, a.ID[:8])
}

// --- API Helpers ---

func getState(gameID string, a aiAgent) map[string]interface{} {
	resp := apiCall("GET", fmt.Sprintf("/api/v1/games/%s/state", gameID), nil, a.APIKey)
	if resp == nil {
		return nil
	}
	var state map[string]interface{}
	json.Unmarshal(resp, &state)
	return state
}

func submitAction(gameID string, a aiAgent, action []byte) map[string]interface{} {
	body, _ := json.Marshal(map[string]json.RawMessage{"action": action})
	resp := apiCall("POST", fmt.Sprintf("/api/v1/games/%s/action", gameID), body, a.APIKey)
	if resp == nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(resp, &result)
	return result
}

func checkGameFinished(gameID string) bool {
	status := readSQL(fmt.Sprintf("SELECT status FROM games WHERE id = '%s'", gameID))
	return status == "finished"
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
		if resp.StatusCode != 403 && resp.StatusCode != 404 && resp.StatusCode != 503 {
			log.Printf("API %d %s %s: %s", resp.StatusCode, method, path, string(data[:min(len(data), 200)]))
		}
		return nil
	}
	return data
}

func apiCallNoAuth(method, path string, body []byte) []byte {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, _ := http.NewRequest(method, apiBase+path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Printf("API %d %s %s: %s", resp.StatusCode, method, path, string(data[:min(len(data), 200)]))
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

func printFinalResults(gameID string) {
	log.Println("Final Results:")
	query := fmt.Sprintf(
		`SELECT a.name, gp.final_rank, gp.chakra_won,
		        round(gp.mu_after::numeric, 1) as mu
		 FROM game_players gp
		 JOIN agents a ON a.id = gp.agent_id
		 WHERE gp.game_id = '%s'
		 ORDER BY gp.final_rank NULLS LAST`, gameID)
	result := readSQL(query)
	if result != "" {
		for _, line := range strings.Split(result, "\n") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				fmt.Printf("  #%s  %-15s  Chakra: %s  μ: %s\n",
					strings.TrimSpace(parts[1]),
					strings.TrimSpace(parts[0]),
					strings.TrimSpace(parts[2]),
					strings.TrimSpace(parts[3]))
			}
		}
	}

	// Event stats
	evtQuery := fmt.Sprintf(
		`SELECT count(*) FILTER (WHERE event_type = 'hand_start') as hands,
		        count(*) as total_events
		 FROM game_events WHERE game_id = '%s'`, gameID)
	evtResult := readSQL(evtQuery)
	if evtResult != "" {
		parts := strings.Split(evtResult, "|")
		if len(parts) >= 2 {
			fmt.Printf("\n  Hands played: %s  |  Total events: %s\n",
				strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}
