package werewolf

// EventType identifies the type of game event.
type EventType string

const (
	EventGameStart     EventType = "game_start"
	EventRoleAssigned  EventType = "role_assigned"
	EventNightStart    EventType = "night_start"
	EventNightAction   EventType = "night_action"   // hidden from spectators in suspense mode
	EventNightResult   EventType = "night_result"    // who was killed
	EventDayStart      EventType = "day_start"
	EventSpeech        EventType = "speech"
	EventVoteStart     EventType = "vote_start"
	EventVoteCast      EventType = "vote_cast"
	EventVoteResult    EventType = "vote_result"
	EventExecution     EventType = "execution"
	EventNoExecution   EventType = "no_execution"    // tied vote
	EventPlayerDeath   EventType = "player_death"
	EventGameOver      EventType = "game_over"
)

// Event is a game event for Event Sourcing.
type Event struct {
	Type    EventType   `json:"type"`
	Day     int         `json:"day"`
	Data    interface{} `json:"data"`
}

type GameStartData struct {
	PlayerCount int          `json:"player_count"`
	Players     []PlayerInfo `json:"players"`
}

type PlayerInfo struct {
	ID   string `json:"id"`
	Seat int    `json:"seat"`
}

type RoleAssignedData struct {
	PlayerID string `json:"player_id"`
	Seat     int    `json:"seat"`
	Role     Role   `json:"role"`
}

type NightStartData struct {
	Day int `json:"day"`
}

type NightActionData struct {
	PlayerID string     `json:"player_id"`
	Role     Role       `json:"role"`
	Action   ActionType `json:"action"`
	TargetID string     `json:"target_id"`
}

type NightResultData struct {
	Day      int    `json:"day"`
	KilledID string `json:"killed_id,omitempty"` // empty if no kill (shouldn't happen normally)
}

type DayStartData struct {
	Day           int      `json:"day"`
	SpeakingOrder []string `json:"speaking_order"` // player IDs in speaking order
}

type SpeechData struct {
	PlayerID string `json:"player_id"`
	Seat     int    `json:"seat"`
	Message  string `json:"message"`
	Order    int    `json:"order"`
}

type VoteStartData struct {
	Day          int      `json:"day"`
	Candidates   []string `json:"candidates"` // all alive player IDs
}

type VoteCastData struct {
	VoterID  string `json:"voter_id"`
	TargetID string `json:"target_id"` // empty for skip/abstain
}

type VoteResultData struct {
	Day     int            `json:"day"`
	Tally   map[string]int `json:"tally"`    // player_id → vote count
	MaxVotes int           `json:"max_votes"`
	Tied    bool           `json:"tied"`
}

type ExecutionData struct {
	PlayerID string `json:"player_id"`
	Role     Role   `json:"role"` // revealed upon death
}

type PlayerDeathData struct {
	PlayerID   string `json:"player_id"`
	Seat       int    `json:"seat"`
	Role       Role   `json:"role"`
	DeathCause string `json:"death_cause"` // "killed" or "executed"
	Day        int    `json:"day"`
}

type GameOverData struct {
	WinningTeam Team           `json:"winning_team"`
	Day         int            `json:"day"`
	Players     []FinalPlayer  `json:"players"`
}

type FinalPlayer struct {
	ID   string `json:"id"`
	Seat int    `json:"seat"`
	Role Role   `json:"role"`
	Team Team   `json:"team"`
	Alive bool  `json:"alive"`
}
