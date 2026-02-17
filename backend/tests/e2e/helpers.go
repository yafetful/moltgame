//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type testAgent struct {
	ID     string
	Name   string
	APIKey string
}

// createTestAgents inserts agents directly into the database via docker exec.
func createTestAgents(t *testing.T, prefix string, count int, chakra int) []testAgent {
	t.Helper()
	suffix := fmt.Sprintf("%d", rand.New(rand.NewSource(time.Now().UnixNano())).Int63()%100000)
	agents := make([]testAgent, count)

	for i := range agents {
		id := uuid.New().String()
		name := fmt.Sprintf("%s-%s-%d", prefix, suffix, i)
		key := fmt.Sprintf("moltgame_sk_e2e_%s_%d", suffix, i)
		hash := sha256.Sum256([]byte(key))

		agents[i] = testAgent{ID: id, Name: name, APIKey: key}

		q := fmt.Sprintf(
			`INSERT INTO agents (id, name, description, api_key_hash, claim_token, verification_code, status, is_claimed, chakra_balance, trueskill_mu, trueskill_sigma) VALUES ('%s', '%s', 'E2E test agent', '%s', 'e2e_%s', 'E2E%d', 'active', true, %d, 25.0, 8.333) ON CONFLICT (name) DO UPDATE SET api_key_hash = EXCLUDED.api_key_hash, chakra_balance = %d, id = EXCLUDED.id`,
			id, name, hex.EncodeToString(hash[:]), name, i, chakra, chakra,
		)
		execSQL(t, q)

		// Read back actual ID
		actualID := readSQL(t, fmt.Sprintf(`SELECT id FROM agents WHERE name = '%s'`, name))
		if actualID != "" {
			agents[i].ID = actualID
		}
	}
	return agents
}

func cleanupTestAgents(t *testing.T, agents []testAgent) {
	t.Helper()
	for _, a := range agents {
		execSQL(t, fmt.Sprintf(`DELETE FROM game_players WHERE agent_id = '%s'`, a.ID))
		execSQL(t, fmt.Sprintf(`DELETE FROM chakra_transactions WHERE agent_id = '%s'`, a.ID))
	}
	for _, a := range agents {
		execSQL(t, fmt.Sprintf(`DELETE FROM agents WHERE id = '%s'`, a.ID))
	}
}

func createGame(t *testing.T, gameType string, agents []testAgent, entryFee int) string {
	t.Helper()
	playerIDs := make([]string, len(agents))
	for i, a := range agents {
		playerIDs[i] = a.ID
	}
	body, _ := json.Marshal(map[string]interface{}{
		"type":       gameType,
		"player_ids": playerIDs,
		"entry_fee":  entryFee,
	})

	resp := apiCall(t, "POST", "/api/v1/games", body, agents[0].APIKey)
	var result struct {
		GameID string `json:"game_id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to parse create game response: %v", err)
	}
	if result.GameID == "" {
		t.Fatalf("Empty game_id in create response: %s", string(resp))
	}
	return result.GameID
}

func getState(t *testing.T, gameID string, a testAgent) map[string]interface{} {
	t.Helper()
	resp := apiCallOpt(t, "GET", fmt.Sprintf("/api/v1/games/%s/state", gameID), nil, a.APIKey, true)
	if resp == nil {
		return nil
	}
	var state map[string]interface{}
	json.Unmarshal(resp, &state)
	return state
}

func getSpectatorState(t *testing.T, gameID string) map[string]interface{} {
	t.Helper()
	resp := apiCall(t, "GET", fmt.Sprintf("/api/v1/games/%s/spectate", gameID), nil, "")
	var state map[string]interface{}
	json.Unmarshal(resp, &state)
	return state
}

func getEvents(t *testing.T, gameID string) []map[string]interface{} {
	t.Helper()
	resp := apiCall(t, "GET", fmt.Sprintf("/api/v1/games/%s/events", gameID), nil, "")
	var events []map[string]interface{}
	json.Unmarshal(resp, &events)
	return events
}

func submitAction(t *testing.T, gameID string, a testAgent, action map[string]interface{}) map[string]interface{} {
	t.Helper()
	actionJSON, _ := json.Marshal(action)
	body, _ := json.Marshal(map[string]json.RawMessage{"action": actionJSON})
	resp := apiCallOpt(t, "POST", fmt.Sprintf("/api/v1/games/%s/action", gameID), body, a.APIKey, true)
	if resp == nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(resp, &result)
	return result
}

func apiCall(t *testing.T, method, path string, body []byte, apiKey string) []byte {
	t.Helper()
	return apiCallOpt(t, method, path, body, apiKey, false)
}

func apiCallOpt(t *testing.T, method, path string, body []byte, apiKey string, allowError bool) []byte {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, _ := http.NewRequest(method, apiBase+path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if allowError {
			return nil
		}
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if allowError {
			return nil
		}
		t.Fatalf("API %s %s returned %d: %s", method, path, resp.StatusCode, string(data[:min(len(data), 300)]))
	}
	return data
}

func execSQL(t *testing.T, query string) {
	t.Helper()
	cmd := exec.Command("docker", "exec", "moltgame-postgres", "psql", "-U", "moltgame", "-d", "moltgame", "-c", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("SQL error: %v\n%s", err, string(out))
	}
}

func readSQL(t *testing.T, query string) string {
	t.Helper()
	cmd := exec.Command("docker", "exec", "moltgame-postgres", "psql", "-U", "moltgame", "-d", "moltgame", "-t", "-A", "-c", query)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
