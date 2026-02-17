//go:build e2e

package e2e

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSpectatorPoker(t *testing.T) {
	agents := createTestAgents(t, "spec-pk", 6, 2000)
	defer cleanupTestAgents(t, agents)

	gameID := createGame(t, "poker", agents, 0)

	// Wait briefly for the game to initialize
	time.Sleep(200 * time.Millisecond)

	state := getSpectatorState(t, gameID)
	if state == nil {
		t.Fatal("Expected spectator state, got nil")
	}

	// Should have players
	players, ok := state["players"].([]interface{})
	if !ok || len(players) != 6 {
		t.Errorf("Expected 6 players in spectator view, got %v", len(players))
	}

	// God view: hole cards should be visible for dealt players
	// Community should exist (even if empty initially)
	if _, ok := state["community"]; !ok {
		t.Error("Expected 'community' field in spectator state")
	}

	t.Logf("Spectator poker state: phase=%v, players=%d", state["phase"], len(players))
}

func TestSpectatorWerewolf(t *testing.T) {
	agents := createTestAgents(t, "spec-ww", 5, 2000)
	defer cleanupTestAgents(t, agents)

	gameID := createGame(t, "werewolf", agents, 0)

	time.Sleep(200 * time.Millisecond)

	state := getSpectatorState(t, gameID)
	if state == nil {
		t.Fatal("Expected spectator state, got nil")
	}

	// Should have 5 players
	players, ok := state["players"].([]interface{})
	if !ok || len(players) != 5 {
		t.Errorf("Expected 5 players in spectator view, got %v", len(players))
	}

	// God view: roles should be visible
	for _, raw := range players {
		p, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := p["role"].(string)
		if role == "" {
			t.Error("Expected role to be visible in god view spectator state")
			break
		}
	}

	t.Logf("Spectator werewolf state: phase=%v, players=%d", state["phase"], len(players))
}

func TestLiveGamesListing(t *testing.T) {
	agents := createTestAgents(t, "live", 6, 2000)
	defer cleanupTestAgents(t, agents)

	gameID := createGame(t, "poker", agents, 0)

	time.Sleep(200 * time.Millisecond)

	resp := apiCall(t, "GET", "/api/v1/games/live", nil, "")
	var games []map[string]interface{}
	if err := json.Unmarshal(resp, &games); err != nil {
		t.Fatalf("Failed to parse live games: %v", err)
	}

	found := false
	for _, g := range games {
		if g["game_id"] == gameID {
			found = true
			if g["game_type"] != "poker" {
				t.Errorf("Expected game_type 'poker', got '%v'", g["game_type"])
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected game %s to appear in /games/live", gameID)
	}
}
