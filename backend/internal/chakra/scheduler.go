package chakra

import (
	"context"
	"log/slog"
	"time"
)

const (
	regenAmount    = 5   // Chakra per tick
	regenCap       = 500 // passive regen cap
	inactiveDays   = 7   // stop regen after N days inactive
)

// RunPassiveRegenLoop runs a periodic loop that gives passive Chakra regen to active agents.
// It blocks until ctx is cancelled.
func RunPassiveRegenLoop(ctx context.Context, repo *Repository, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("passive regen scheduler started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("passive regen scheduler stopped")
			return
		case <-ticker.C:
			runPassiveRegen(ctx, repo)
		}
	}
}

func runPassiveRegen(ctx context.Context, repo *Repository) {
	ids, err := repo.GetActiveAgentIDs(ctx, inactiveDays)
	if err != nil {
		slog.Error("passive regen: failed to get active agents", "error", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	var credited int
	for _, id := range ids {
		if err := repo.PassiveRegen(ctx, id, regenAmount, regenCap); err != nil {
			slog.Error("passive regen: failed for agent", "agent_id", id, "error", err)
			continue
		}
		credited++
	}

	slog.Info("passive regen complete", "eligible", len(ids), "credited", credited)
}
