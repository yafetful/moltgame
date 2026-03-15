package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moltgame/backend/internal/models"
)

var ErrOwnerNotFound = errors.New("owner not found")
var ErrAlreadyBound = errors.New("already bound")

const bindBonusChakra = 2000

type OwnerRepository struct {
	db *pgxpool.Pool
}

func NewOwnerRepository(db *pgxpool.Pool) *OwnerRepository {
	return &OwnerRepository{db: db}
}

// UpsertOwner creates or updates the owner's profile on every successful OAuth login.
func (r *OwnerRepository) UpsertOwner(ctx context.Context, twitterID, twitterHandle, displayName, avatarURL string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO owner_accounts (twitter_id, twitter_handle, display_name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (twitter_id) DO UPDATE
			SET twitter_handle = EXCLUDED.twitter_handle,
			    display_name   = EXCLUDED.display_name,
			    avatar_url     = EXCLUDED.avatar_url`,
		twitterID, twitterHandle, displayName, avatarURL,
	)
	if err != nil {
		return fmt.Errorf("upsert owner: %w", err)
	}
	return nil
}

// GetOwner returns the owner account including bound_agent_id if any.
func (r *OwnerRepository) GetOwner(ctx context.Context, twitterID string) (*models.OwnerAccount, error) {
	owner := &models.OwnerAccount{}
	err := r.db.QueryRow(ctx, `
		SELECT id, twitter_id, twitter_handle,
		       COALESCE(display_name,''), COALESCE(avatar_url,''),
		       bound_agent_id, last_check_in, created_at
		FROM owner_accounts WHERE twitter_id = $1`, twitterID,
	).Scan(
		&owner.ID, &owner.TwitterID, &owner.TwitterHandle,
		&owner.DisplayName, &owner.AvatarURL,
		&owner.BoundAgentID, &owner.LastCheckIn, &owner.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOwnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query owner: %w", err)
	}
	return owner, nil
}

// BindOwnerToAgent atomically:
//   - Verifies neither side is already bound
//   - Sets agent owner fields and invalidates verification_code/claim_token
//   - Sets owner_accounts.bound_agent_id
//   - Credits bindBonusChakra (2000) to the agent
func (r *OwnerRepository) BindOwnerToAgent(ctx context.Context, ownerTwitterID, ownerHandle, agentID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bind tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock and check agent is not already bound
	var currentOwnerID *string
	err = tx.QueryRow(ctx,
		"SELECT owner_twitter_id FROM agents WHERE id = $1 FOR UPDATE", agentID,
	).Scan(&currentOwnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAgentNotFound
	}
	if err != nil {
		return fmt.Errorf("lock agent: %w", err)
	}
	if currentOwnerID != nil && *currentOwnerID != "" {
		return ErrAlreadyBound
	}

	// Lock and check this owner doesn't already have a bound agent
	var currentBound *string
	err = tx.QueryRow(ctx,
		"SELECT bound_agent_id FROM owner_accounts WHERE twitter_id = $1 FOR UPDATE", ownerTwitterID,
	).Scan(&currentBound)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOwnerNotFound
	}
	if err != nil {
		return fmt.Errorf("lock owner: %w", err)
	}
	if currentBound != nil {
		return ErrAlreadyBound
	}

	// Update agent: set owner, invalidate codes
	_, err = tx.Exec(ctx, `
		UPDATE agents
		SET owner_twitter_id     = $2,
		    owner_twitter_handle = $3,
		    is_claimed           = true,
		    claimed_at           = NOW(),
		    last_active_at       = NOW(),
		    verification_code    = '',
		    claim_token          = ''
		WHERE id = $1`,
		agentID, ownerTwitterID, ownerHandle,
	)
	if err != nil {
		return fmt.Errorf("update agent owner: %w", err)
	}

	// Update owner_accounts: record bound agent
	_, err = tx.Exec(ctx,
		"UPDATE owner_accounts SET bound_agent_id = $1 WHERE twitter_id = $2",
		agentID, ownerTwitterID,
	)
	if err != nil {
		return fmt.Errorf("update owner bound_agent: %w", err)
	}

	// Credit bonus Chakra (inline to keep it in the same transaction)
	var newBalance int
	err = tx.QueryRow(ctx, `
		UPDATE agents SET chakra_balance = chakra_balance + $2
		WHERE id = $1
		RETURNING chakra_balance`,
		agentID, bindBonusChakra,
	).Scan(&newBalance)
	if err != nil {
		return fmt.Errorf("credit bonus chakra: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO chakra_transactions (agent_id, amount, type, balance_after, note)
		VALUES ($1, $2, 'bonus_grant', $3, 'Dev bind bonus')`,
		agentID, bindBonusChakra, newBalance,
	)
	if err != nil {
		return fmt.Errorf("insert bonus chakra tx: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateCheckIn sets last_check_in = NOW() for the owner.
func (r *OwnerRepository) UpdateCheckIn(ctx context.Context, twitterID string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE owner_accounts SET last_check_in = NOW() WHERE twitter_id = $1",
		twitterID,
	)
	return err
}
