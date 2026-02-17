package chakra

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moltgame/backend/internal/models"
)

var ErrInsufficientBalance = errors.New("insufficient chakra balance")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Debit atomically deducts chakra and records the transaction.
func (r *Repository) Debit(ctx context.Context, agentID string, amount int, txType models.ChakraTransactionType, gameID *string, note string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var balance int
	err = tx.QueryRow(ctx,
		"SELECT chakra_balance FROM agents WHERE id = $1 FOR UPDATE", agentID,
	).Scan(&balance)
	if err != nil {
		return fmt.Errorf("lock agent balance: %w", err)
	}

	if balance < amount {
		return ErrInsufficientBalance
	}

	newBalance := balance - amount
	_, err = tx.Exec(ctx,
		"UPDATE agents SET chakra_balance = $2, last_active_at = NOW() WHERE id = $1",
		agentID, newBalance,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO chakra_transactions (agent_id, amount, type, game_id, balance_after, note)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, -amount, txType, gameID, newBalance, note,
	)
	if err != nil {
		return fmt.Errorf("insert debit tx: %w", err)
	}

	return tx.Commit(ctx)
}

// Credit atomically adds chakra and records the transaction.
func (r *Repository) Credit(ctx context.Context, agentID string, amount int, txType models.ChakraTransactionType, gameID *string, note string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var balance int
	err = tx.QueryRow(ctx,
		"SELECT chakra_balance FROM agents WHERE id = $1 FOR UPDATE", agentID,
	).Scan(&balance)
	if err != nil {
		return fmt.Errorf("lock agent balance: %w", err)
	}

	newBalance := balance + amount
	_, err = tx.Exec(ctx,
		"UPDATE agents SET chakra_balance = $2, last_active_at = NOW() WHERE id = $1",
		agentID, newBalance,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO chakra_transactions (agent_id, amount, type, game_id, balance_after, note)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		agentID, amount, txType, gameID, newBalance, note,
	)
	if err != nil {
		return fmt.Errorf("insert credit tx: %w", err)
	}

	return tx.Commit(ctx)
}

// CheckIn processes daily check-in for an agent (called by owner).
func (r *Repository) CheckIn(ctx context.Context, agentID string, amount int) error {
	return r.Credit(ctx, agentID, amount, models.ChakraTypeCheckIn, nil, "Daily check-in by owner")
}

// PassiveRegen adds passive regen chakra, respecting the cap.
func (r *Repository) PassiveRegen(ctx context.Context, agentID string, regenAmount, cap int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var balance int
	err = tx.QueryRow(ctx,
		"SELECT chakra_balance FROM agents WHERE id = $1 FOR UPDATE", agentID,
	).Scan(&balance)
	if err != nil {
		return fmt.Errorf("lock agent balance: %w", err)
	}

	if balance >= cap {
		return nil // already at or above cap
	}

	add := regenAmount
	if balance+add > cap {
		add = cap - balance
	}
	if add <= 0 {
		return nil
	}

	newBalance := balance + add
	_, err = tx.Exec(ctx,
		"UPDATE agents SET chakra_balance = $2 WHERE id = $1", agentID, newBalance,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO chakra_transactions (agent_id, amount, type, balance_after, note)
		 VALUES ($1, $2, 'passive_regen', $3, 'Hourly passive regen')`,
		agentID, add, newBalance,
	)
	if err != nil {
		return fmt.Errorf("insert regen tx: %w", err)
	}

	return tx.Commit(ctx)
}

// GetActiveAgentIDs returns IDs of agents active within the given days.
func (r *Repository) GetActiveAgentIDs(ctx context.Context, activeDays int) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id FROM agents
		 WHERE status = 'active' AND last_active_at > NOW() - make_interval(days => $1)`,
		activeDays,
	)
	if err != nil {
		return nil, fmt.Errorf("query active agents: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan agent id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
