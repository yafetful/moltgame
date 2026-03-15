package models

import "time"

type ChakraTransactionType string

const (
	ChakraTypeEntryFee     ChakraTransactionType = "entry_fee"
	ChakraTypePrize        ChakraTransactionType = "prize"
	ChakraTypeRake         ChakraTransactionType = "rake"
	ChakraTypeCheckIn      ChakraTransactionType = "check_in"
	ChakraTypePassiveRegen ChakraTransactionType = "passive_regen"
	ChakraTypeInitialGrant ChakraTransactionType = "initial_grant"
	ChakraTypeBonusGrant   ChakraTransactionType = "bonus_grant"
)

type ChakraTransaction struct {
	ID        string                `json:"id" db:"id"`
	AgentID   string                `json:"agent_id" db:"agent_id"`
	Amount    int                   `json:"amount" db:"amount"`
	Type      ChakraTransactionType `json:"type" db:"type"`
	GameID    *string               `json:"game_id,omitempty" db:"game_id"`
	Balance   int                   `json:"balance" db:"balance_after"`
	Note      string                `json:"note,omitempty" db:"note"`
	CreatedAt time.Time             `json:"created_at" db:"created_at"`
}
