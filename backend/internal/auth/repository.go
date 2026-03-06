package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moltgame/backend/internal/models"
)

var ErrAgentNotFound = errors.New("agent not found")
var ErrNameTaken = errors.New("agent name already taken")

type AgentRepository struct {
	db *pgxpool.Pool
}

func NewAgentRepository(db *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) FindAgentByKeyHash(ctx context.Context, keyHash string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		"SELECT id FROM agents WHERE api_key_hash = $1", keyHash,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAgentNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query agent by key hash: %w", err)
	}
	return id, nil
}

// IsAgentActive checks if an agent has been claimed and is active.
func (r *AgentRepository) IsAgentActive(ctx context.Context, agentID string) (bool, error) {
	var status string
	err := r.db.QueryRow(ctx,
		"SELECT status FROM agents WHERE id = $1", agentID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query agent status: %w", err)
	}
	return status == "active", nil
}

func (r *AgentRepository) CreateAgent(ctx context.Context, name, model, description, avatarURL, keyHash, claimToken, verificationCode string) (*models.Agent, error) {
	agent := &models.Agent{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO agents (name, model, description, avatar_url, api_key_hash, claim_token, verification_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, name, COALESCE(model,''), description, avatar_url, status, is_claimed, chakra_balance,
		           trueskill_mu, trueskill_sigma, claim_token, verification_code, created_at`,
		name, model, description, avatarURL, keyHash, claimToken, verificationCode,
	).Scan(
		&agent.ID, &agent.Name, &agent.Model, &agent.Description, &agent.AvatarURL,
		&agent.Status, &agent.IsClaimed, &agent.ChakraBalance,
		&agent.TrueSkillMu, &agent.TrueSkillSigma,
		&agent.ClaimToken, &agent.VerificationCode, &agent.CreatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrNameTaken
		}
		return nil, fmt.Errorf("insert agent: %w", err)
	}
	return agent, nil
}

func (r *AgentRepository) GetAgentByID(ctx context.Context, id string) (*models.Agent, error) {
	agent := &models.Agent{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(model,''), COALESCE(description,''), COALESCE(avatar_url,''), status, is_claimed,
		        COALESCE(owner_twitter_id,''), COALESCE(owner_twitter_handle,''), chakra_balance,
		        trueskill_mu, trueskill_sigma, created_at, claimed_at
		 FROM agents WHERE id = $1`, id,
	).Scan(
		&agent.ID, &agent.Name, &agent.Model, &agent.Description, &agent.AvatarURL,
		&agent.Status, &agent.IsClaimed,
		&agent.OwnerTwitterID, &agent.OwnerTwitterHandle, &agent.ChakraBalance,
		&agent.TrueSkillMu, &agent.TrueSkillSigma, &agent.CreatedAt, &agent.ClaimedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query agent by id: %w", err)
	}
	return agent, nil
}

func (r *AgentRepository) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	agent := &models.Agent{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(model,''), COALESCE(description,''), COALESCE(avatar_url,''), status, is_claimed,
		        COALESCE(owner_twitter_handle,''), chakra_balance,
		        trueskill_mu, trueskill_sigma, created_at
		 FROM agents WHERE name = $1`, name,
	).Scan(
		&agent.ID, &agent.Name, &agent.Model, &agent.Description, &agent.AvatarURL,
		&agent.Status, &agent.IsClaimed,
		&agent.OwnerTwitterHandle, &agent.ChakraBalance,
		&agent.TrueSkillMu, &agent.TrueSkillSigma, &agent.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query agent by name: %w", err)
	}
	return agent, nil
}

func (r *AgentRepository) UpdateAgentProfile(ctx context.Context, id, description, avatarURL string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE agents SET description = $2, avatar_url = $3, last_active_at = NOW()
		 WHERE id = $1`, id, description, avatarURL,
	)
	return err
}

func (r *AgentRepository) ClaimAgent(ctx context.Context, id, twitterID, twitterHandle string, initialChakra int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	_, err = tx.Exec(ctx,
		`UPDATE agents
		 SET status = 'active', is_claimed = true,
		     owner_twitter_id = $2, owner_twitter_handle = $3,
		     chakra_balance = $4, claimed_at = $5, last_active_at = $5
		 WHERE id = $1 AND status = 'unclaimed'`,
		id, twitterID, twitterHandle, initialChakra, now,
	)
	if err != nil {
		return fmt.Errorf("update agent claim: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO chakra_transactions (agent_id, amount, type, balance_after, note)
		 VALUES ($1, $2, 'initial_grant', $2, 'Initial Chakra grant on claim')`,
		id, initialChakra,
	)
	if err != nil {
		return fmt.Errorf("insert initial chakra tx: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *AgentRepository) FindAgentByClaimToken(ctx context.Context, claimToken string) (*models.Agent, error) {
	agent := &models.Agent{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, status, is_claimed, verification_code, claim_token
		 FROM agents WHERE claim_token = $1`, claimToken,
	).Scan(&agent.ID, &agent.Name, &agent.Status, &agent.IsClaimed, &agent.VerificationCode, &agent.ClaimToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query agent by claim token: %w", err)
	}
	return agent, nil
}

func (r *AgentRepository) RotateAPIKey(ctx context.Context, agentID, newKeyHash string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE agents SET api_key_hash = $2 WHERE id = $1", agentID, newKeyHash,
	)
	return err
}

func (r *AgentRepository) GetAgentsByOwner(ctx context.Context, twitterID string) ([]*models.Agent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, COALESCE(model,''), COALESCE(description,''), COALESCE(avatar_url,''), status, is_claimed,
		        chakra_balance, trueskill_mu, trueskill_sigma, created_at, claimed_at
		 FROM agents WHERE owner_twitter_id = $1 ORDER BY created_at DESC`, twitterID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agents by owner: %w", err)
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		a := &models.Agent{}
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Model, &a.Description, &a.AvatarURL,
			&a.Status, &a.IsClaimed,
			&a.ChakraBalance, &a.TrueSkillMu, &a.TrueSkillSigma,
			&a.CreatedAt, &a.ClaimedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func isDuplicateKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
