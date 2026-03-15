package models

import "time"

type AgentStatus string

const (
	AgentStatusUnclaimed AgentStatus = "unclaimed"
	AgentStatusActive    AgentStatus = "active"
	AgentStatusSuspended AgentStatus = "suspended"
)

type Agent struct {
	ID                 string      `json:"id" db:"id"`
	Name               string      `json:"name" db:"name"`
	Model              string      `json:"model,omitempty" db:"model"`
	Description        string      `json:"description,omitempty" db:"description"`
	AvatarURL          string      `json:"avatar_url,omitempty" db:"avatar_url"`
	APIKeyHash         string      `json:"-" db:"api_key_hash"`
	ClaimToken         string      `json:"-" db:"claim_token"`
	VerificationCode   string      `json:"verification_code,omitempty" db:"verification_code"`
	Status             AgentStatus `json:"status" db:"status"`
	IsClaimed          bool        `json:"is_claimed" db:"is_claimed"`
	OwnerTwitterID     string      `json:"owner_twitter_id,omitempty" db:"owner_twitter_id"`
	OwnerTwitterHandle string      `json:"owner_twitter_handle,omitempty" db:"owner_twitter_handle"`
	ChakraBalance      int         `json:"chakra_balance" db:"chakra_balance"`
	TrueSkillMu        float64     `json:"trueskill_mu" db:"trueskill_mu"`
	TrueSkillSigma     float64     `json:"trueskill_sigma" db:"trueskill_sigma"`
	GamesPlayed        int         `json:"games_played" db:"games_played"`
	Wins               int         `json:"wins" db:"wins"`
	CreatedAt          time.Time   `json:"created_at" db:"created_at"`
	ClaimedAt          *time.Time  `json:"claimed_at,omitempty" db:"claimed_at"`
}
