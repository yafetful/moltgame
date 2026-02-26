// test-structured: Test that all 6 OpenRouter models return valid structured JSON.
// Sends the same poker game context to each model and validates the response.
// Usage: go run ./cmd/test-structured
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// Same schema as simulate-ai
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

const systemPrompt = `You are playing Texas Hold'em poker tournament. Analyze the game state and choose your action.

Rules:
- "action" must be exactly one of the valid action types listed below
- "amount" is the bet size for "raise" only (between min and max). For other actions set amount to 0
- "reason" max 10 words
- Consider pot odds, position, hand strength, and opponent behavior`

const testPrompt = `Hand #5, Phase: flop
Your cards: Ac, Jd
Community: As, Kh, 7d
Pot: 240
Blinds: 40/80
Current bet: 80

Players:
- deepseek-v3: 1200 chips, bet 80
- grok-fast: 900 chips [folded]
- qwen-122b (YOU): 1380 chips, bet 40
- gemini-pro: 1100 chips [folded]
- gpt-52: 1420 chips [folded]
- claude-sonnet: 1000 chips, bet 40

Valid actions:
- fold
- call (cost: 40)
- raise (min: 160, max: 1380)`

type testResult struct {
	Model   string
	Name    string
	OK      bool
	Action  string
	Amount  float64
	Reason  string
	RawResp string
	Error   string
	Latency time.Duration
}

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[test] ")

	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	orKey := os.Getenv("OPENROUTER_API_KEY")
	if orKey == "" {
		log.Fatal("OPENROUTER_API_KEY not set in .env")
	}

	models := []struct {
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

	fmt.Println("=== OpenRouter Structured Output Test ===")
	fmt.Println("Testing 6 models × 3 rounds each")
	fmt.Println()

	rounds := 3
	allResults := make([][]testResult, len(models))
	for i := range allResults {
		allResults[i] = make([]testResult, rounds)
	}

	for round := 0; round < rounds; round++ {
		fmt.Printf("--- Round %d ---\n", round+1)

		var wg sync.WaitGroup
		results := make([]testResult, len(models))

		for i, m := range models {
			modelID := os.Getenv(m.envKey)
			if modelID == "" {
				log.Fatalf("%s not set in .env", m.envKey)
			}

			wg.Add(1)
			go func(idx int, model, name string) {
				defer wg.Done()
				results[idx] = testModel(orKey, model, name)
			}(i, modelID, m.name)
		}

		wg.Wait()

		for i, r := range results {
			allResults[i][round] = r
			status := "PASS"
			if !r.OK {
				status = "FAIL"
			}
			fmt.Printf("  [%s] %-15s  %5s  action=%-6s  amount=%.0f  latency=%v\n",
				status, r.Name, r.Model[:min(len(r.Model), 20)], r.Action, r.Amount, r.Latency.Round(time.Millisecond))
			if !r.OK {
				fmt.Printf("         error: %s\n", r.Error)
				if r.RawResp != "" {
					fmt.Printf("         raw: %.120s\n", r.RawResp)
				}
			} else if r.Reason != "" {
				fmt.Printf("         reason: %q\n", r.Reason)
			}
		}
		fmt.Println()

		if round < rounds-1 {
			time.Sleep(1 * time.Second) // brief pause between rounds
		}
	}

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Printf("%-15s  Pass/Total  Avg Latency\n", "Model")
	fmt.Println(strings.Repeat("-", 50))

	totalPass, totalTests := 0, 0
	for i, m := range models {
		pass := 0
		var totalLatency time.Duration
		for _, r := range allResults[i] {
			if r.OK {
				pass++
			}
			totalLatency += r.Latency
		}
		avgLatency := totalLatency / time.Duration(rounds)
		fmt.Printf("%-15s  %d/%d         %v\n", m.name, pass, rounds, avgLatency.Round(time.Millisecond))
		totalPass += pass
		totalTests += rounds
	}
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%-15s  %d/%d\n", "TOTAL", totalPass, totalTests)

	if totalPass == totalTests {
		fmt.Println("\nAll models passed structured output test!")
	} else {
		fmt.Printf("\n%d/%d tests failed. Consider replacing failing models.\n", totalTests-totalPass, totalTests)
		os.Exit(1)
	}
}

func testModel(orKey, model, name string) testResult {
	start := time.Now()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": testPrompt},
		},
		"temperature":     0.7,
		"max_tokens":      500,
		"response_format": pokerActionSchema,
	})

	req, _ := http.NewRequest("POST", openRouterURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+orKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return testResult{Model: model, Name: name, Error: fmt.Sprintf("request failed: %v", err), Latency: latency}
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return testResult{
			Model:   model,
			Name:    name,
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
			RawResp: string(data[:min(len(data), 200)]),
			Latency: latency,
		}
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &orResp); err != nil || len(orResp.Choices) == 0 {
		return testResult{
			Model:   model,
			Name:    name,
			Error:   "failed to parse OpenRouter response structure",
			RawResp: string(data[:min(len(data), 200)]),
			Latency: latency,
		}
	}

	content := strings.TrimSpace(orResp.Choices[0].Message.Content)
	if content == "" {
		return testResult{
			Model:   model,
			Name:    name,
			Error:   "empty content",
			Latency: latency,
		}
	}

	var parsed struct {
		Action string  `json:"action"`
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return testResult{
			Model:   model,
			Name:    name,
			Error:   fmt.Sprintf("JSON parse failed: %v", err),
			RawResp: content[:min(len(content), 200)],
			Latency: latency,
		}
	}

	// Validate action
	validActions := []string{"fold", "check", "call", "raise", "allin"}
	actionValid := false
	for _, va := range validActions {
		if parsed.Action == va {
			actionValid = true
			break
		}
	}
	if !actionValid {
		return testResult{
			Model:   model,
			Name:    name,
			Error:   fmt.Sprintf("invalid action: %q", parsed.Action),
			RawResp: content,
			Latency: latency,
		}
	}

	return testResult{
		Model:   model,
		Name:    name,
		OK:      true,
		Action:  parsed.Action,
		Amount:  parsed.Amount,
		Reason:  parsed.Reason,
		RawResp: content,
		Latency: latency,
	}
}
