package poker

import "github.com/cardrank/cardrank"

// Phase represents the current phase of a poker hand.
type Phase int

const (
	PhaseIdle     Phase = iota // between hands
	PhasePreflop               // hole cards dealt, pre-flop betting
	PhaseFlop                  // flop dealt, flop betting
	PhaseTurn                  // turn dealt, turn betting
	PhaseRiver                 // river dealt, river betting
	PhaseShowdown              // showdown, evaluate hands
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhasePreflop:
		return "preflop"
	case PhaseFlop:
		return "flop"
	case PhaseTurn:
		return "turn"
	case PhaseRiver:
		return "river"
	case PhaseShowdown:
		return "showdown"
	default:
		return "unknown"
	}
}

// ActionType represents a player action.
type ActionType string

const (
	ActionFold  ActionType = "fold"
	ActionCheck ActionType = "check"
	ActionCall  ActionType = "call"
	ActionRaise ActionType = "raise"
	ActionAllIn ActionType = "allin"
)

// Action is a player's action in a betting round.
type Action struct {
	Type   ActionType `json:"type"`
	Amount int        `json:"amount,omitempty"` // for raise: total bet amount
	Reason string     `json:"reason,omitempty"` // AI decision reason (stored in events for spectators)
}

// ActionOption describes a valid action a player can take.
type ActionOption struct {
	Type      ActionType `json:"type"`
	MinAmount int        `json:"min_amount,omitempty"` // for raise: minimum total bet
	MaxAmount int        `json:"max_amount,omitempty"` // for raise: maximum total bet (all-in)
	CallCost  int        `json:"call_cost,omitempty"`  // for call: chips needed
}

// Player is a player at the poker table.
type Player struct {
	ID   string // agent ID
	Seat int    // 0-indexed seat number

	// Chip state
	Chips      int // remaining chips (not counting current bets)
	StartChips int // chips at start of current hand (for elimination ranking)

	// Per-hand state (reset each hand)
	Hole     []cardrank.Card // hole cards
	TotalBet int             // total bet in current hand
	Folded   bool
	AllIn    bool

	// Per-round state (reset each betting round)
	Bet      int  // bet in current round
	HasActed bool // has acted in current round

	// Tournament state
	Eliminated   bool
	EliminatedAt int // hand number when eliminated

	// Timeout tracking
	TimeoutCount int  // consecutive timeout count (reset on normal action)
	Disconnected bool // marked after 3 consecutive timeouts, auto-fold immediately
}

// IsActive returns true if the player can still act in the current hand.
func (p *Player) IsActive() bool {
	return !p.Eliminated && !p.Folded && !p.AllIn
}

// IsInHand returns true if the player is still in the current hand (not folded/eliminated).
func (p *Player) IsInHand() bool {
	return !p.Eliminated && !p.Folded
}

// GameState is the game state sent to agents via API.
type GameState struct {
	GameID       string          `json:"game_id"`
	HandNum      int             `json:"hand_num"`
	Phase        string          `json:"phase"`
	Finished     bool            `json:"finished"`
	Community    []cardrank.Card `json:"community"`
	CurrentBet   int             `json:"current_bet"`
	SmallBlind   int             `json:"small_blind"`
	BigBlind     int             `json:"big_blind"`
	Pots         []Pot           `json:"pots"`
	ActionOn     int             `json:"action_on"`
	Players      []PlayerState   `json:"players"`
	ValidActions []ActionOption  `json:"valid_actions,omitempty"`
}

// PlayerState is the player state visible in GameState.
type PlayerState struct {
	ID           string          `json:"id"`
	Seat         int             `json:"seat"`
	Chips        int             `json:"chips"`
	Bet          int             `json:"bet"`
	TotalBet     int             `json:"total_bet"`
	Hole         []cardrank.Card `json:"hole,omitempty"`
	Folded       bool            `json:"folded"`
	AllIn        bool            `json:"all_in"`
	Eliminated   bool            `json:"eliminated"`
	Disconnected bool            `json:"disconnected,omitempty"`
}
