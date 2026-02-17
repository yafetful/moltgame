package werewolf

// Role is a player's secret role.
type Role string

const (
	RoleWerewolf Role = "werewolf"
	RoleSeer     Role = "seer"
	RoleVillager Role = "villager"
)

// Phase is the current game phase.
type Phase int

const (
	PhaseIdle       Phase = iota // before game starts
	PhaseNight                   // werewolves pick target, seer investigates
	PhaseNightReveal             // night results announced
	PhaseDay                     // discussion phase
	PhaseVote                    // voting phase
	PhaseExecution               // execution result announced
	PhaseGameOver                // game has ended
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhaseNight:
		return "night"
	case PhaseNightReveal:
		return "night_reveal"
	case PhaseDay:
		return "day"
	case PhaseVote:
		return "vote"
	case PhaseExecution:
		return "execution"
	case PhaseGameOver:
		return "game_over"
	default:
		return "unknown"
	}
}

// ActionType is the type of action a player can take.
type ActionType string

const (
	ActionKill        ActionType = "kill"        // werewolf: choose kill target
	ActionInvestigate ActionType = "investigate" // seer: choose investigation target
	ActionSpeak       ActionType = "speak"       // day: say something
	ActionVotePlayer  ActionType = "vote"        // vote: choose who to eliminate
	ActionSkipVote    ActionType = "skip"        // vote: abstain
)

// Action is a player's action.
type Action struct {
	Type     ActionType `json:"type"`
	TargetID string     `json:"target_id,omitempty"` // target player ID (for kill/investigate/vote)
	Message  string     `json:"message,omitempty"`   // speech content (for speak)
}

// Team is the faction.
type Team string

const (
	TeamWerewolf Team = "werewolf"
	TeamVillage  Team = "village"
)

// Player represents a player in the game.
type Player struct {
	ID   string
	Seat int
	Role Role
	Team Team

	Alive       bool
	EliminatedDay int // day number when eliminated (0 = still alive)
	DeathCause  string // "killed" (night) or "executed" (vote)

	// Night action tracking
	NightActionDone bool
	NightTarget     string // target player ID for this night

	// Day tracking
	HasSpoken bool
	SpeechIdx int // order of speech in current day

	// Vote tracking
	HasVoted bool
	VotedFor string // player ID voted for
}

// SeerResult is the result of a seer investigation.
type SeerResult struct {
	TargetID string `json:"target_id"`
	IsWolf   bool   `json:"is_wolf"`
}

// GameState is the state visible to a specific player via API.
type GameState struct {
	GameID   string        `json:"game_id"`
	Day      int           `json:"day"`
	Phase    string        `json:"phase"`
	Players  []PlayerState `json:"players"`
	YourRole Role          `json:"your_role,omitempty"`

	// Visible only to wolves
	WolfTeammates []string `json:"wolf_teammates,omitempty"`

	// Visible only to seer
	SeerResults []SeerResult `json:"seer_results,omitempty"`

	// Day discussion history
	Speeches []Speech `json:"speeches,omitempty"`

	// Vote results (after voting)
	VoteResults map[string]int `json:"vote_results,omitempty"`

	// Current action requirement
	ActionRequired *ActionRequired `json:"action_required,omitempty"`
}

// PlayerState is the public state of a player.
type PlayerState struct {
	ID           string `json:"id"`
	Seat         int    `json:"seat"`
	Alive        bool   `json:"alive"`
	Role         Role   `json:"role,omitempty"` // only shown for dead players
	EliminatedDay int   `json:"eliminated_day,omitempty"`
	DeathCause   string `json:"death_cause,omitempty"`
}

// Speech is a player's speech during discussion.
type Speech struct {
	PlayerID string `json:"player_id"`
	Seat     int    `json:"seat"`
	Message  string `json:"message"`
	Order    int    `json:"order"`
}

// ActionRequired describes what action the current player needs to take.
type ActionRequired struct {
	Type       ActionType `json:"type"`
	ValidTargets []string `json:"valid_targets,omitempty"` // valid target player IDs
	MaxLength  int        `json:"max_length,omitempty"`    // max speech length
}
