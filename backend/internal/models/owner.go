package models

import "time"

type OwnerAccount struct {
	ID            string     `json:"id"`
	TwitterID     string     `json:"twitter_id"`
	TwitterHandle string     `json:"twitter_handle"`
	DisplayName   string     `json:"display_name,omitempty"`
	AvatarURL     string     `json:"avatar_url,omitempty"`
	BoundAgentID  *string    `json:"bound_agent_id,omitempty"`
	LastCheckIn   *time.Time `json:"last_check_in,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
