package werewolf

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
)

var (
	ErrGameOver       = errors.New("game is over")
	ErrGameNotStarted = errors.New("game not started")
	ErrNotYourTurn    = errors.New("not your turn")
	ErrInvalidAction  = errors.New("invalid action")
	ErrInvalidTarget  = errors.New("invalid target")
	ErrAlreadyActed   = errors.New("already acted this phase")
)

const (
	MaxSpeechLength     = 500
	DiscussionRounds    = 1 // each alive player speaks once per day
)

// RoleDistribution defines the roles for a given player count.
var RoleDistribution = map[int][]Role{
	5: {RoleWerewolf, RoleWerewolf, RoleSeer, RoleVillager, RoleVillager},
}

// Game represents a werewolf game.
type Game struct {
	ID      string
	Players []*Player

	Day      int   // current day (starts at 1)
	Phase    Phase
	Finished bool

	// Night state
	wolfTarget    string          // agreed kill target for this night
	wolfVotes     map[string]string // wolfID → targetID
	seerResults   map[string][]SeerResult // playerID → investigation results

	// Day state
	speakingOrder []string // player IDs in order
	speakIdx      int      // index into speakingOrder for current speaker
	speeches      []Speech // all speeches this day

	// Vote state
	votes map[string]string // voterID → targetID (or "" for skip)

	// Events
	Events []Event

	// RNG
	rng *rand.Rand
}

// NewGame creates a new werewolf game.
func NewGame(id string, playerIDs []string, seed int64) (*Game, error) {
	roles, ok := RoleDistribution[len(playerIDs)]
	if !ok {
		return nil, fmt.Errorf("unsupported player count: %d (supported: 5)", len(playerIDs))
	}

	rng := rand.New(rand.NewSource(seed))

	// Shuffle roles
	shuffledRoles := make([]Role, len(roles))
	copy(shuffledRoles, roles)
	rng.Shuffle(len(shuffledRoles), func(i, j int) {
		shuffledRoles[i], shuffledRoles[j] = shuffledRoles[j], shuffledRoles[i]
	})

	players := make([]*Player, len(playerIDs))
	for i, pid := range playerIDs {
		role := shuffledRoles[i]
		team := TeamVillage
		if role == RoleWerewolf {
			team = TeamWerewolf
		}
		players[i] = &Player{
			ID:    pid,
			Seat:  i,
			Role:  role,
			Team:  team,
			Alive: true,
		}
	}

	g := &Game{
		ID:          id,
		Players:     players,
		Day:         0,
		Phase:       PhaseIdle,
		seerResults: make(map[string][]SeerResult),
		rng:         rng,
	}

	return g, nil
}

// Start begins the game. Returns events from role assignment.
func (g *Game) Start() ([]Event, error) {
	if g.Phase != PhaseIdle {
		return nil, fmt.Errorf("game already started")
	}

	g.Events = nil

	// Emit game start
	playerInfos := make([]PlayerInfo, len(g.Players))
	for i, p := range g.Players {
		playerInfos[i] = PlayerInfo{ID: p.ID, Seat: p.Seat}
	}
	g.emit(EventGameStart, GameStartData{
		PlayerCount: len(g.Players),
		Players:     playerInfos,
	})

	// Emit role assignments (private - only visible to each player)
	for _, p := range g.Players {
		g.emit(EventRoleAssigned, RoleAssignedData{
			PlayerID: p.ID,
			Seat:     p.Seat,
			Role:     p.Role,
		})
	}

	// Start first night
	g.startNight()

	return g.Events, nil
}

// CurrentActor returns the player ID who needs to act, or "" if no action needed.
func (g *Game) CurrentActor() string {
	switch g.Phase {
	case PhaseNight:
		// Find first player who hasn't completed their night action
		for _, p := range g.Players {
			if p.Alive && !p.NightActionDone && p.Role != RoleVillager {
				return p.ID
			}
		}
		// Check wolves: need consensus
		for _, p := range g.Players {
			if p.Alive && p.Role == RoleWerewolf && !p.NightActionDone {
				return p.ID
			}
		}
		return ""
	case PhaseDay:
		if g.speakIdx < len(g.speakingOrder) {
			return g.speakingOrder[g.speakIdx]
		}
		return ""
	case PhaseVote:
		for _, p := range g.Players {
			if p.Alive && !p.HasVoted {
				return p.ID
			}
		}
		return ""
	default:
		return ""
	}
}

// Act processes a player action. Returns events emitted.
func (g *Game) Act(playerID string, action Action) ([]Event, error) {
	if g.Finished {
		return nil, ErrGameOver
	}
	if g.Phase == PhaseIdle {
		return nil, ErrGameNotStarted
	}

	startEvtCount := len(g.Events)

	switch g.Phase {
	case PhaseNight:
		if err := g.handleNightAction(playerID, action); err != nil {
			return nil, err
		}
	case PhaseDay:
		if err := g.handleDayAction(playerID, action); err != nil {
			return nil, err
		}
	case PhaseVote:
		if err := g.handleVoteAction(playerID, action); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: no actions allowed in phase %s", ErrInvalidAction, g.Phase)
	}

	return g.Events[startEvtCount:], nil
}

// GetGameState returns the game state visible to a specific player.
func (g *Game) GetGameState(playerID string) GameState {
	player := g.findPlayer(playerID)

	state := GameState{
		GameID:  g.ID,
		Day:     g.Day,
		Phase:   g.Phase.String(),
		Players: g.makePlayerStates(),
	}

	if player != nil {
		state.YourRole = player.Role

		// Wolves see their teammates
		if player.Role == RoleWerewolf {
			for _, p := range g.Players {
				if p.Role == RoleWerewolf && p.ID != playerID {
					state.WolfTeammates = append(state.WolfTeammates, p.ID)
				}
			}
		}

		// Seer sees investigation results
		if player.Role == RoleSeer {
			state.SeerResults = g.seerResults[playerID]
		}
	}

	// Day speeches are public
	state.Speeches = g.speeches

	// Action required for current player
	if g.CurrentActor() == playerID && player != nil {
		state.ActionRequired = g.getActionRequired(player)
	}

	return state
}

// GetSpectatorState returns the full game state (god view).
func (g *Game) GetSpectatorState() GameState {
	state := GameState{
		GameID:   g.ID,
		Day:      g.Day,
		Phase:    g.Phase.String(),
		Players:  g.makeFullPlayerStates(),
		Speeches: g.speeches,
	}
	return state
}

// IsGameOver returns whether the game has ended.
func (g *Game) IsGameOver() bool {
	return g.Finished
}

// --- Night handling ---

func (g *Game) startNight() {
	g.Day++
	g.Phase = PhaseNight
	g.wolfTarget = ""
	g.wolfVotes = make(map[string]string)

	for _, p := range g.Players {
		p.NightActionDone = false
		p.NightTarget = ""
	}

	g.emit(EventNightStart, NightStartData{Day: g.Day})
}

func (g *Game) handleNightAction(playerID string, action Action) error {
	player := g.findPlayer(playerID)
	if player == nil || !player.Alive {
		return ErrNotYourTurn
	}

	switch player.Role {
	case RoleWerewolf:
		if action.Type != ActionKill {
			return fmt.Errorf("%w: werewolf must use 'kill' action", ErrInvalidAction)
		}
		return g.handleWolfKill(player, action.TargetID)

	case RoleSeer:
		if action.Type != ActionInvestigate {
			return fmt.Errorf("%w: seer must use 'investigate' action", ErrInvalidAction)
		}
		return g.handleSeerInvestigate(player, action.TargetID)

	case RoleVillager:
		// Villagers have no night action
		return fmt.Errorf("%w: villagers have no night action", ErrInvalidAction)

	default:
		return ErrInvalidAction
	}
}

func (g *Game) handleWolfKill(wolf *Player, targetID string) error {
	if wolf.NightActionDone {
		return ErrAlreadyActed
	}

	target := g.findPlayer(targetID)
	if target == nil || !target.Alive {
		return fmt.Errorf("%w: target must be alive", ErrInvalidTarget)
	}
	if target.Role == RoleWerewolf {
		return fmt.Errorf("%w: cannot kill fellow werewolf", ErrInvalidTarget)
	}

	wolf.NightActionDone = true
	wolf.NightTarget = targetID
	g.wolfVotes[wolf.ID] = targetID

	g.emit(EventNightAction, NightActionData{
		PlayerID: wolf.ID,
		Role:     RoleWerewolf,
		Action:   ActionKill,
		TargetID: targetID,
	})

	// Check if all wolves have voted
	aliveWolves := g.aliveWolves()
	if len(g.wolfVotes) >= len(aliveWolves) {
		g.resolveWolfKill()
	}

	// Check if all night actions are done
	g.checkNightComplete()
	return nil
}

func (g *Game) resolveWolfKill() {
	// Find the most voted target (first wolf's choice breaks ties)
	voteCounts := make(map[string]int)
	for _, target := range g.wolfVotes {
		voteCounts[target]++
	}

	maxVotes := 0
	var chosen string
	for _, wolf := range g.aliveWolves() {
		target := g.wolfVotes[wolf.ID]
		if voteCounts[target] > maxVotes {
			maxVotes = voteCounts[target]
			chosen = target
		}
	}

	g.wolfTarget = chosen
}

func (g *Game) handleSeerInvestigate(seer *Player, targetID string) error {
	if seer.NightActionDone {
		return ErrAlreadyActed
	}

	target := g.findPlayer(targetID)
	if target == nil || !target.Alive {
		return fmt.Errorf("%w: target must be alive", ErrInvalidTarget)
	}
	if target.ID == seer.ID {
		return fmt.Errorf("%w: cannot investigate yourself", ErrInvalidTarget)
	}

	seer.NightActionDone = true
	seer.NightTarget = targetID

	isWolf := target.Role == RoleWerewolf
	result := SeerResult{TargetID: targetID, IsWolf: isWolf}
	g.seerResults[seer.ID] = append(g.seerResults[seer.ID], result)

	g.emit(EventNightAction, NightActionData{
		PlayerID: seer.ID,
		Role:     RoleSeer,
		Action:   ActionInvestigate,
		TargetID: targetID,
	})

	g.checkNightComplete()
	return nil
}

func (g *Game) checkNightComplete() {
	for _, p := range g.Players {
		if p.Alive && p.Role != RoleVillager && !p.NightActionDone {
			return // still waiting
		}
	}

	// All night actions done - resolve night
	g.resolveNight()
}

func (g *Game) resolveNight() {
	g.Phase = PhaseNightReveal

	// Kill the target
	if g.wolfTarget != "" {
		target := g.findPlayer(g.wolfTarget)
		if target != nil && target.Alive {
			target.Alive = false
			target.EliminatedDay = g.Day
			target.DeathCause = "killed"

			g.emit(EventNightResult, NightResultData{
				Day:      g.Day,
				KilledID: target.ID,
			})
			g.emit(EventPlayerDeath, PlayerDeathData{
				PlayerID:   target.ID,
				Seat:       target.Seat,
				Role:       target.Role,
				DeathCause: "killed",
				Day:        g.Day,
			})
		}
	}

	// Check win condition
	if g.checkWinCondition() {
		return
	}

	// Start day discussion
	g.startDay()
}

// --- Day handling ---

func (g *Game) startDay() {
	g.Phase = PhaseDay
	g.speeches = nil
	g.speakIdx = 0

	// Build speaking order: alive players starting from a rotating position
	alive := g.alivePlayers()
	startIdx := (g.Day - 1) % len(alive) // rotate starting speaker
	order := make([]string, len(alive))
	for i := range alive {
		order[i] = alive[(startIdx+i)%len(alive)].ID
	}
	g.speakingOrder = order

	for _, p := range g.Players {
		p.HasSpoken = false
	}

	g.emit(EventDayStart, DayStartData{
		Day:           g.Day,
		SpeakingOrder: g.speakingOrder,
	})
}

func (g *Game) handleDayAction(playerID string, action Action) error {
	if action.Type != ActionSpeak {
		return fmt.Errorf("%w: must use 'speak' action during day", ErrInvalidAction)
	}

	if g.speakIdx >= len(g.speakingOrder) {
		return fmt.Errorf("%w: all players have spoken", ErrInvalidAction)
	}
	if g.speakingOrder[g.speakIdx] != playerID {
		return ErrNotYourTurn
	}

	player := g.findPlayer(playerID)
	if player == nil || !player.Alive {
		return ErrNotYourTurn
	}

	// Sanitize and truncate message
	msg := StripControlChars(action.Message)
	msg, _ = SanitizeSpeech(msg)
	if len(msg) > MaxSpeechLength {
		msg = msg[:MaxSpeechLength]
	}

	speech := Speech{
		PlayerID: playerID,
		Seat:     player.Seat,
		Message:  msg,
		Order:    g.speakIdx,
	}
	g.speeches = append(g.speeches, speech)
	player.HasSpoken = true

	g.emit(EventSpeech, SpeechData{
		PlayerID: playerID,
		Seat:     player.Seat,
		Message:  msg,
		Order:    g.speakIdx,
	})

	g.speakIdx++

	// Check if all players have spoken
	if g.speakIdx >= len(g.speakingOrder) {
		g.startVote()
	}

	return nil
}

// --- Vote handling ---

func (g *Game) startVote() {
	g.Phase = PhaseVote
	g.votes = make(map[string]string)

	for _, p := range g.Players {
		p.HasVoted = false
		p.VotedFor = ""
	}

	candidates := make([]string, 0)
	for _, p := range g.Players {
		if p.Alive {
			candidates = append(candidates, p.ID)
		}
	}

	g.emit(EventVoteStart, VoteStartData{
		Day:        g.Day,
		Candidates: candidates,
	})
}

func (g *Game) handleVoteAction(playerID string, action Action) error {
	player := g.findPlayer(playerID)
	if player == nil || !player.Alive {
		return ErrNotYourTurn
	}
	if player.HasVoted {
		return ErrAlreadyActed
	}

	switch action.Type {
	case ActionVotePlayer:
		target := g.findPlayer(action.TargetID)
		if target == nil || !target.Alive {
			return fmt.Errorf("%w: vote target must be alive", ErrInvalidTarget)
		}
		player.HasVoted = true
		player.VotedFor = action.TargetID
		g.votes[playerID] = action.TargetID

		g.emit(EventVoteCast, VoteCastData{
			VoterID:  playerID,
			TargetID: action.TargetID,
		})

	case ActionSkipVote:
		player.HasVoted = true
		player.VotedFor = ""
		g.votes[playerID] = ""

		g.emit(EventVoteCast, VoteCastData{
			VoterID:  playerID,
			TargetID: "",
		})

	default:
		return fmt.Errorf("%w: must use 'vote' or 'skip' action during voting", ErrInvalidAction)
	}

	// Check if all alive players have voted
	allVoted := true
	for _, p := range g.Players {
		if p.Alive && !p.HasVoted {
			allVoted = false
			break
		}
	}

	if allVoted {
		g.resolveVote()
	}

	return nil
}

func (g *Game) resolveVote() {
	g.Phase = PhaseExecution

	// Count votes
	tally := make(map[string]int)
	for _, targetID := range g.votes {
		if targetID != "" {
			tally[targetID]++
		}
	}

	// Find max votes
	maxVotes := 0
	for _, count := range tally {
		if count > maxVotes {
			maxVotes = count
		}
	}

	// Check for tie
	topCandidates := make([]string, 0)
	for pid, count := range tally {
		if count == maxVotes {
			topCandidates = append(topCandidates, pid)
		}
	}

	tied := len(topCandidates) > 1

	g.emit(EventVoteResult, VoteResultData{
		Day:      g.Day,
		Tally:    tally,
		MaxVotes: maxVotes,
		Tied:     tied,
	})

	if tied || maxVotes == 0 {
		// Tied vote: no one is executed
		g.emit(EventNoExecution, nil)
	} else {
		// Execute the player with most votes
		executed := g.findPlayer(topCandidates[0])
		if executed != nil {
			executed.Alive = false
			executed.EliminatedDay = g.Day
			executed.DeathCause = "executed"

			g.emit(EventExecution, ExecutionData{
				PlayerID: executed.ID,
				Role:     executed.Role,
			})
			g.emit(EventPlayerDeath, PlayerDeathData{
				PlayerID:   executed.ID,
				Seat:       executed.Seat,
				Role:       executed.Role,
				DeathCause: "executed",
				Day:        g.Day,
			})
		}
	}

	// Check win condition
	if g.checkWinCondition() {
		return
	}

	// Start next night
	g.startNight()
}

// --- Win condition ---

func (g *Game) checkWinCondition() bool {
	wolves := 0
	villagers := 0
	for _, p := range g.Players {
		if p.Alive {
			if p.Team == TeamWerewolf {
				wolves++
			} else {
				villagers++
			}
		}
	}

	var winner Team
	if wolves == 0 {
		winner = TeamVillage
	} else if wolves > villagers {
		winner = TeamWerewolf
	} else {
		return false // game continues
	}

	g.Finished = true
	g.Phase = PhaseGameOver

	finalPlayers := make([]FinalPlayer, len(g.Players))
	for i, p := range g.Players {
		finalPlayers[i] = FinalPlayer{
			ID:    p.ID,
			Seat:  p.Seat,
			Role:  p.Role,
			Team:  p.Team,
			Alive: p.Alive,
		}
	}

	g.emit(EventGameOver, GameOverData{
		WinningTeam: winner,
		Day:         g.Day,
		Players:     finalPlayers,
	})

	return true
}

// FindPlayer returns the player with the given ID, or nil.
func (g *Game) FindPlayer(id string) *Player {
	return g.findPlayer(id)
}

// GetValidTargets returns valid target IDs for the given player's current action.
func (g *Game) GetValidTargets(playerID string) []string {
	p := g.findPlayer(playerID)
	if p == nil || !p.Alive {
		return nil
	}
	var targets []string
	for _, other := range g.Players {
		if other.Alive && other.ID != playerID {
			if p.Role == RoleWerewolf && other.Role == RoleWerewolf {
				continue // wolves can't target each other
			}
			targets = append(targets, other.ID)
		}
	}
	return targets
}

// --- Helpers ---

func (g *Game) emit(t EventType, data interface{}) {
	g.Events = append(g.Events, Event{
		Type: t,
		Day:  g.Day,
		Data: data,
	})
}

func (g *Game) findPlayer(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (g *Game) alivePlayers() []*Player {
	var result []*Player
	for _, p := range g.Players {
		if p.Alive {
			result = append(result, p)
		}
	}
	return result
}

func (g *Game) aliveWolves() []*Player {
	var result []*Player
	for _, p := range g.Players {
		if p.Alive && p.Role == RoleWerewolf {
			result = append(result, p)
		}
	}
	return result
}

func (g *Game) makePlayerStates() []PlayerState {
	states := make([]PlayerState, len(g.Players))
	for i, p := range g.Players {
		states[i] = PlayerState{
			ID:            p.ID,
			Seat:          p.Seat,
			Alive:         p.Alive,
			EliminatedDay: p.EliminatedDay,
			DeathCause:    p.DeathCause,
		}
		// Only reveal role for dead players
		if !p.Alive {
			states[i].Role = p.Role
		}
	}
	return states
}

func (g *Game) makeFullPlayerStates() []PlayerState {
	states := make([]PlayerState, len(g.Players))
	for i, p := range g.Players {
		states[i] = PlayerState{
			ID:            p.ID,
			Seat:          p.Seat,
			Alive:         p.Alive,
			Role:          p.Role, // all roles visible
			EliminatedDay: p.EliminatedDay,
			DeathCause:    p.DeathCause,
		}
	}
	return states
}

func (g *Game) getActionRequired(player *Player) *ActionRequired {
	switch g.Phase {
	case PhaseNight:
		switch player.Role {
		case RoleWerewolf:
			if !player.NightActionDone {
				targets := make([]string, 0)
				for _, p := range g.Players {
					if p.Alive && p.Role != RoleWerewolf {
						targets = append(targets, p.ID)
					}
				}
				return &ActionRequired{
					Type:         ActionKill,
					ValidTargets: targets,
				}
			}
		case RoleSeer:
			if !player.NightActionDone {
				targets := make([]string, 0)
				for _, p := range g.Players {
					if p.Alive && p.ID != player.ID {
						targets = append(targets, p.ID)
					}
				}
				return &ActionRequired{
					Type:         ActionInvestigate,
					ValidTargets: targets,
				}
			}
		}
	case PhaseDay:
		if g.speakIdx < len(g.speakingOrder) && g.speakingOrder[g.speakIdx] == player.ID {
			return &ActionRequired{
				Type:      ActionSpeak,
				MaxLength: MaxSpeechLength,
			}
		}
	case PhaseVote:
		if !player.HasVoted {
			targets := make([]string, 0)
			for _, p := range g.Players {
				if p.Alive {
					targets = append(targets, p.ID)
				}
			}
			return &ActionRequired{
				Type:         ActionVotePlayer,
				ValidTargets: targets,
			}
		}
	}
	return nil
}

// WinningTeam returns the winning team, or "" if game is not over.
func (g *Game) WinningTeam() Team {
	if !g.Finished {
		return ""
	}
	// Check last event
	for i := len(g.Events) - 1; i >= 0; i-- {
		if g.Events[i].Type == EventGameOver {
			if data, ok := g.Events[i].Data.(GameOverData); ok {
				return data.WinningTeam
			}
		}
	}
	return ""
}

// Rankings returns the final rankings (winners first).
func (g *Game) Rankings() []FinalPlayer {
	players := make([]FinalPlayer, len(g.Players))
	for i, p := range g.Players {
		players[i] = FinalPlayer{
			ID:    p.ID,
			Seat:  p.Seat,
			Role:  p.Role,
			Team:  p.Team,
			Alive: p.Alive,
		}
	}

	winner := g.WinningTeam()
	sort.Slice(players, func(i, j int) bool {
		// Winning team first
		iWon := (players[i].Team == winner)
		jWon := (players[j].Team == winner)
		if iWon != jWon {
			return iWon
		}
		// Alive before dead
		if players[i].Alive != players[j].Alive {
			return players[i].Alive
		}
		return players[i].Seat < players[j].Seat
	})

	return players
}
