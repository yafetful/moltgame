// +build ignore

// Test candidate models for structured output compatibility.
// Run: go run ./cmd/test-structured/candidates_test.go
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

const orURL = "https://openrouter.ai/api/v1/chat/completions"

var schema = map[string]interface{}{
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
					"description": "Brief one-sentence explanation of your decision.",
				},
			},
			"required":             []string{"action", "amount", "reason"},
			"additionalProperties": false,
		},
	},
}

const sysPrompt = `You are playing Texas Hold'em poker. Respond with your action.
- "action" must be one of: fold, check, call, raise, allin
- "amount" is for raise only, 0 for other actions
- "reason" is a brief explanation`

const userPrompt = `Hand #5, Phase: flop
Your cards: Ac, Jd
Community: As, Kh, 7d
Pot: 240, Blinds: 40/80, Current bet: 80

Valid actions:
- fold
- call (cost: 40)
- raise (min: 160, max: 1380)`

func main() {
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	orKey := os.Getenv("OPENROUTER_API_KEY")
	if orKey == "" {
		log.Fatal("OPENROUTER_API_KEY not set")
	}

	candidates := []string{
		// Already confirmed working
		// "deepseek/deepseek-v3.2",
		// "x-ai/grok-4.1-fast",
		// "anthropic/claude-sonnet-4.6",

		// Candidates to replace failing models
		"openai/gpt-5.1",
		"openai/gpt-5.2-chat",
		"google/gemini-3-pro-preview",
		"google/gemini-3-flash-preview",
		"qwen/qwen3.5-397b-a17b",
		"qwen/qwen3.5-27b",
		"meta-llama/llama-4-maverick",
		"meta-llama/llama-4-scout",
		"deepseek/deepseek-v3.2-speciale",
	}

	fmt.Println("=== Candidate Model Test ===")
	fmt.Println()

	var wg sync.WaitGroup
	type result struct {
		model   string
		ok      bool
		action  string
		latency time.Duration
		errMsg  string
		raw     string
	}
	results := make([]result, len(candidates))

	for i, m := range candidates {
		wg.Add(1)
		go func(idx int, model string) {
			defer wg.Done()
			start := time.Now()

			reqBody, _ := json.Marshal(map[string]interface{}{
				"model": model,
				"messages": []map[string]string{
					{"role": "system", "content": sysPrompt},
					{"role": "user", "content": userPrompt},
				},
				"temperature":     0.7,
				"max_tokens":      500,
				"response_format": schema,
			})

			req, _ := http.NewRequest("POST", orURL, bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+orKey)

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			latency := time.Since(start)

			if err != nil {
				results[idx] = result{model: model, errMsg: err.Error(), latency: latency}
				return
			}
			defer resp.Body.Close()

			data, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				results[idx] = result{model: model, errMsg: fmt.Sprintf("HTTP %d", resp.StatusCode), raw: string(data[:min(len(data), 150)]), latency: latency}
				return
			}

			var orResp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(data, &orResp); err != nil || len(orResp.Choices) == 0 {
				results[idx] = result{model: model, errMsg: "bad response structure", raw: string(data[:min(len(data), 150)]), latency: latency}
				return
			}

			content := strings.TrimSpace(orResp.Choices[0].Message.Content)
			if content == "" {
				results[idx] = result{model: model, errMsg: "empty content", latency: latency}
				return
			}

			var parsed struct {
				Action string `json:"action"`
			}
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				results[idx] = result{model: model, errMsg: "JSON parse failed", raw: content[:min(len(content), 150)], latency: latency}
				return
			}

			results[idx] = result{model: model, ok: true, action: parsed.Action, latency: latency}
		}(i, m)
	}

	wg.Wait()

	for _, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-40s  action=%-6s  latency=%v\n", status, r.model, r.action, r.latency.Round(time.Millisecond))
		if !r.ok {
			fmt.Printf("       error: %s\n", r.errMsg)
			if r.raw != "" {
				fmt.Printf("       raw: %.120s\n", r.raw)
			}
		}
	}
}
