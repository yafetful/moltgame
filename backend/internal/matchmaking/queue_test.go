package matchmaking

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moltgame/backend/internal/models"
	"github.com/moltgame/backend/internal/trueskill"
)

func TestJoinAndLeave(t *testing.T) {
	svc := NewService(nil, func(ctx context.Context, gt models.GameType, players []*QueueEntry) error {
		return nil
	})

	err := svc.Join(models.GameTypePoker, "agent-1", trueskill.DefaultMu, trueskill.DefaultSigma)
	if err != nil {
		t.Fatalf("join error: %v", err)
	}

	status := svc.QueueStatus()
	if status["poker"] != 1 {
		t.Errorf("queue size = %d, want 1", status["poker"])
	}

	// Duplicate join
	err = svc.Join(models.GameTypePoker, "agent-1", trueskill.DefaultMu, trueskill.DefaultSigma)
	if err == nil {
		t.Error("expected error for duplicate join")
	}

	// Leave
	err = svc.Leave(models.GameTypePoker, "agent-1")
	if err != nil {
		t.Fatalf("leave error: %v", err)
	}

	status = svc.QueueStatus()
	if status["poker"] != 0 {
		t.Errorf("queue size = %d, want 0", status["poker"])
	}
}

func TestPokerMatchForming(t *testing.T) {
	var mu sync.Mutex
	var matchedPlayers []*QueueEntry

	svc := NewService(nil, func(ctx context.Context, gt models.GameType, players []*QueueEntry) error {
		mu.Lock()
		matchedPlayers = players
		mu.Unlock()
		return nil
	})

	// Add 6 players (poker requires 6)
	for i := 0; i < 6; i++ {
		id := "agent-" + string(rune('a'+i))
		err := svc.Join(models.GameTypePoker, id, trueskill.DefaultMu, trueskill.DefaultSigma)
		if err != nil {
			t.Fatalf("join error: %v", err)
		}
	}

	// Wait for async match creation
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(matchedPlayers) != 6 {
		t.Errorf("matched = %d, want 6", len(matchedPlayers))
	}
	mu.Unlock()

	// Queue should be empty now
	status := svc.QueueStatus()
	if status["poker"] != 0 {
		t.Errorf("queue size after match = %d, want 0", status["poker"])
	}
}

func TestWerewolfMatchForming(t *testing.T) {
	var mu sync.Mutex
	var matchedPlayers []*QueueEntry

	svc := NewService(nil, func(ctx context.Context, gt models.GameType, players []*QueueEntry) error {
		mu.Lock()
		matchedPlayers = players
		mu.Unlock()
		return nil
	})

	// Add 5 players (werewolf requires 5)
	for i := 0; i < 5; i++ {
		id := "wolf-" + string(rune('a'+i))
		svc.Join(models.GameTypeWerewolf, id, trueskill.DefaultMu, trueskill.DefaultSigma)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(matchedPlayers) != 5 {
		t.Errorf("matched = %d, want 5", len(matchedPlayers))
	}
	mu.Unlock()
}

func TestSkillRangeRelaxation(t *testing.T) {
	sigma := trueskill.DefaultSigma

	r0 := skillRange(0, sigma)
	r10 := skillRange(10*time.Second, sigma)
	r20 := skillRange(20*time.Second, sigma)
	r45 := skillRange(45*time.Second, sigma)
	r120 := skillRange(120*time.Second, sigma)

	// 0-15s: 1σ
	if r0 != sigma || r10 != sigma {
		t.Errorf("0-15s range should be 1σ, got %f / %f", r0, r10)
	}

	// 15-30s: 2σ
	if r20 != 2*sigma {
		t.Errorf("15-30s range should be 2σ, got %f", r20)
	}

	// 30-60s: 3σ
	if r45 != 3*sigma {
		t.Errorf("30-60s range should be 3σ, got %f", r45)
	}

	// 60s+: very large
	if r120 < 100 {
		t.Errorf("60s+ range should be effectively unlimited, got %f", r120)
	}
}

func TestNoMatchWithTooFewPlayers(t *testing.T) {
	matchCalled := false
	svc := NewService(nil, func(ctx context.Context, gt models.GameType, players []*QueueEntry) error {
		matchCalled = true
		return nil
	})

	// Only 3 players for poker (needs 6)
	for i := 0; i < 3; i++ {
		id := "agent-" + string(rune('a'+i))
		svc.Join(models.GameTypePoker, id, trueskill.DefaultMu, trueskill.DefaultSigma)
	}

	time.Sleep(50 * time.Millisecond)

	if matchCalled {
		t.Error("match should not be formed with only 3 players")
	}

	status := svc.QueueStatus()
	if status["poker"] != 3 {
		t.Errorf("queue size = %d, want 3", status["poker"])
	}
}

func TestInvalidGameType(t *testing.T) {
	svc := NewService(nil, nil)
	err := svc.Join("chess", "agent-1", 25, 8.333)
	if err == nil {
		t.Error("expected error for unsupported game type")
	}
}
