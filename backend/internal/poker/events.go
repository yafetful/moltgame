package poker

import "github.com/cardrank/cardrank"

// EventType identifies the type of game event.
type EventType string

const (
	EventHandStart        EventType = "hand_start"
	EventBlindsPosted     EventType = "blinds_posted"
	EventHoleDealt        EventType = "hole_dealt"
	EventCommunityDealt   EventType = "community_dealt"
	EventPlayerAction     EventType = "player_action"
	EventShowdown         EventType = "showdown"
	EventPotAwarded       EventType = "pot_awarded"
	EventPlayerEliminated EventType = "player_eliminated"
	EventHandEnd          EventType = "hand_end"
	EventGameOver         EventType = "game_over"
)

// Event is a game event for Event Sourcing and spectator broadcasting.
type Event struct {
	Type    EventType   `json:"type"`
	HandNum int         `json:"hand_num"`
	Data    interface{} `json:"data"`
}

// HandStartData is emitted at the beginning of each hand.
type HandStartData struct {
	HandNum    int          `json:"hand_num"`
	DealerSeat int          `json:"dealer_seat"`
	SmallBlind int          `json:"small_blind"`
	BigBlind   int          `json:"big_blind"`
	Players    []PlayerInfo `json:"players"`
}

// PlayerInfo is a snapshot of a player's public info.
type PlayerInfo struct {
	ID    string `json:"id"`
	Seat  int    `json:"seat"`
	Chips int    `json:"chips"`
}

// BlindsPostedData is emitted after blinds are posted.
type BlindsPostedData struct {
	SmallBlindSeat   int `json:"sb_seat"`
	SmallBlindAmount int `json:"sb_amount"`
	BigBlindSeat     int `json:"bb_seat"`
	BigBlindAmount   int `json:"bb_amount"`
}

// HoleDealtData is emitted when hole cards are dealt to a player.
type HoleDealtData struct {
	Seat  int             `json:"seat"`
	Cards []cardrank.Card `json:"cards"`
}

// CommunityDealtData is emitted when community cards are dealt.
type CommunityDealtData struct {
	Phase string          `json:"phase"` // "flop", "turn", "river"
	Cards []cardrank.Card `json:"cards"` // newly dealt cards
	Board []cardrank.Card `json:"board"` // full board so far
}

// PlayerActionData is emitted for each player action.
type PlayerActionData struct {
	Seat      int        `json:"seat"`
	PlayerID  string     `json:"player_id"`
	Action    ActionType `json:"action"`
	Amount    int        `json:"amount"`
	ChipsLeft int        `json:"chips_left"`
	TotalPot  int        `json:"total_pot"`
}

// ShowdownData is emitted when hands are revealed at showdown.
type ShowdownData struct {
	Players []ShowdownPlayer `json:"players"`
	Board   []cardrank.Card  `json:"board"`
}

// ShowdownPlayer is a player's hand at showdown.
type ShowdownPlayer struct {
	Seat     int             `json:"seat"`
	PlayerID string          `json:"player_id"`
	Hole     []cardrank.Card `json:"hole"`
	HandDesc string          `json:"hand_desc"`
	HandRank int             `json:"hand_rank"`
}

// PotAwardedData is emitted when a pot is awarded.
type PotAwardedData struct {
	PotIndex int         `json:"pot_index"`
	Amount   int         `json:"amount"`
	Winners  []PotWinner `json:"winners"`
}

// PotWinner is a winner of a pot.
type PotWinner struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"player_id"`
	Amount   int    `json:"amount"`
}

// PlayerEliminatedData is emitted when a player is eliminated.
type PlayerEliminatedData struct {
	Seat     int    `json:"seat"`
	PlayerID string `json:"player_id"`
	Rank     int    `json:"rank"`
}

// HandEndData is emitted at the end of each hand.
type HandEndData struct {
	HandNum int          `json:"hand_num"`
	Players []PlayerInfo `json:"players"`
}

// GameOverData is emitted when the tournament ends.
type GameOverData struct {
	WinnerSeat int            `json:"winner_seat"`
	WinnerID   string         `json:"winner_id"`
	Rankings   []RankingEntry `json:"rankings"`
}

// RankingEntry is a player's final tournament ranking.
type RankingEntry struct {
	Rank     int    `json:"rank"`
	Seat     int    `json:"seat"`
	PlayerID string `json:"player_id"`
}
