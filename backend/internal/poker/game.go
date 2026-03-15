package poker

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/cardrank/cardrank"
)

var (
	ErrNotYourTurn    = errors.New("not your turn")
	ErrInvalidAction  = errors.New("invalid action")
	ErrGameOver       = errors.New("game is over")
	ErrHandInProgress = errors.New("hand already in progress")
	ErrNoHandActive   = errors.New("no hand in progress")
)

const (
	StartingChips = 1500
)

// Game represents a poker tournament game.
type Game struct {
	ID      string
	Players []*Player

	// Tournament state
	DealerIdx int   // dealer button position (index in Players)
	HandNum   int   // current hand number (1-indexed)
	Phase     Phase // current game phase
	Finished  bool

	// Deck & community
	deck      *cardrank.Deck
	Community []cardrank.Card

	// Betting round state
	CurrentBet int // highest bet in current round
	MinRaise   int // minimum raise increment
	ActionIdx  int // index in Players of current actor (-1 = no action needed)

	// Blinds
	Blinds *BlindSchedule

	// Event log for current hand
	Events []Event

	// RNG for reproducible shuffling
	rng *rand.Rand
}

// NewGame creates a new poker tournament game.
// playerNames is optional: maps player ID → display name.
// playerAvatars is optional: maps player ID → avatar URL.
func NewGame(id string, playerIDs []string, seed int64, playerNames map[string]string, playerAvatars map[string]string) *Game {
	players := make([]*Player, len(playerIDs))
	for i, pid := range playerIDs {
		players[i] = &Player{
			ID:        pid,
			Name:      playerNames[pid],
			AvatarURL: playerAvatars[pid],
			Seat:      i,
			Chips:     StartingChips,
		}
	}

	return &Game{
		ID:        id,
		Players:   players,
		DealerIdx: 0,
		HandNum:   0,
		Phase:     PhaseIdle,
		Blinds:    DefaultSchedule,
		ActionIdx: -1,
		rng:       rand.New(rand.NewSource(seed)),
	}
}

// StartHand begins a new hand. Returns events emitted during setup.
func (g *Game) StartHand() ([]Event, error) {
	if g.Finished {
		return nil, ErrGameOver
	}
	if g.Phase != PhaseIdle {
		return nil, ErrHandInProgress
	}

	g.HandNum++
	g.Events = nil

	// Moving Button — dealer always advances to the next alive player
	if g.HandNum > 1 {
		g.DealerIdx = g.nextSeatAfter(g.DealerIdx)
	}

	// Reset per-hand player state
	for _, p := range g.Players {
		p.TotalBet = 0 // always reset (prevents stale bets from eliminated players)
		if p.Eliminated {
			continue
		}
		p.Hole = nil
		p.Folded = false
		p.AllIn = false
		p.Bet = 0
		p.HasActed = false
		p.StartChips = p.Chips // m3: record starting chips for elimination ranking
	}
	g.Community = nil

	// Get blind amounts
	sb, bb := g.Blinds.GetBlinds(g.HandNum)

	// Emit hand_start event
	g.emit(EventHandStart, HandStartData{
		HandNum:    g.HandNum,
		DealerSeat: g.DealerIdx,
		SmallBlind: sb,
		BigBlind:   bb,
		Players:    g.makePlayerInfos(),
	})

	// Post blinds
	sbIdx := g.sbIdx()
	bbIdx := g.bbIdx()
	g.postBlind(sbIdx, sb)
	g.postBlind(bbIdx, bb)

	g.emit(EventBlindsPosted, BlindsPostedData{
		SmallBlindSeat:   sbIdx,
		SmallBlindAmount: g.Players[sbIdx].Bet,
		BigBlindSeat:     bbIdx,
		BigBlindAmount:   g.Players[bbIdx].Bet,
	})

	// M2: CurrentBet = nominal BB (not the actual posted amount which may be short)
	g.CurrentBet = bb
	g.MinRaise = bb

	// Shuffle and deal
	g.deck = cardrank.NewDeck()
	g.deck.Shuffle(g.rng, 1)

	// Deal hole cards to all non-eliminated players
	for _, p := range g.Players {
		if !p.Eliminated {
			p.Hole = g.deck.Draw(2)
			g.emit(EventHoleDealt, HoleDealtData{
				Seat:  p.Seat,
				Cards: p.Hole,
			})
		}
	}

	// Set phase and determine first to act
	g.Phase = PhasePreflop
	for _, p := range g.Players {
		if !p.Eliminated {
			p.HasActed = false
		}
	}

	g.ActionIdx = g.nextActiveAfter(bbIdx)

	// If everyone is already all-in from blinds, run to showdown
	if g.activePlayersCount() <= 1 && g.playersInHand() > 1 {
		g.runToShowdown()
	} else if g.activePlayersCount() == 0 {
		// Edge case: all players all-in or only 1 in hand
		if g.playersInHand() > 1 {
			g.runToShowdown()
		} else {
			g.endHandEarly()
		}
	}

	return g.Events, nil
}

// CurrentActor returns the ID of the player who needs to act, or "" if no action needed.
func (g *Game) CurrentActor() string {
	if g.Phase == PhaseIdle || g.Phase == PhaseShowdown || g.Finished {
		return ""
	}
	if g.ActionIdx < 0 || g.ActionIdx >= len(g.Players) {
		return ""
	}
	return g.Players[g.ActionIdx].ID
}

// ValidActions returns the valid actions for the current actor.
func (g *Game) ValidActions() []ActionOption {
	if g.CurrentActor() == "" {
		return nil
	}
	return g.validActionsFor(g.Players[g.ActionIdx])
}

// Act processes a player action. Returns events emitted as a result.
func (g *Game) Act(playerID string, action Action) ([]Event, error) {
	if g.Finished {
		return nil, ErrGameOver
	}
	if g.Phase == PhaseIdle {
		return nil, ErrNoHandActive
	}
	if g.CurrentActor() != playerID {
		return nil, ErrNotYourTurn
	}

	p := g.Players[g.ActionIdx]
	startEvtCount := len(g.Events)

	// Normal action resets timeout tracking (agent is responsive)
	if p.TimeoutCount > 0 {
		p.TimeoutCount = 0
	}
	if p.Disconnected {
		p.Disconnected = false
	}

	if err := g.executeAction(p, action); err != nil {
		return nil, err
	}

	g.advanceAction()

	return g.Events[startEvtCount:], nil
}

// IsHandOver returns true if the current hand has ended.
func (g *Game) IsHandOver() bool {
	return g.Phase == PhaseIdle
}

// IsGameOver returns true if the tournament is finished.
func (g *Game) IsGameOver() bool {
	return g.Finished
}

// GetGameState returns the game state visible to a specific player.
func (g *Game) GetGameState(playerID string) GameState {
	state := GameState{
		GameID:     g.ID,
		HandNum:    g.HandNum,
		Phase:      g.Phase.String(),
		Finished:   g.Finished,
		Community:  g.Community,
		CurrentBet: g.CurrentBet,
		DealerSeat: g.DealerIdx,
		Pots:       g.currentPots(),
		ActionOn:   -1,
		Players:    make([]PlayerState, 0, len(g.Players)),
	}

	sb, bb := g.Blinds.GetBlinds(g.HandNum)
	state.SmallBlind = sb
	state.BigBlind = bb

	if g.ActionIdx >= 0 && g.ActionIdx < len(g.Players) {
		state.ActionOn = g.Players[g.ActionIdx].Seat
	}

	for _, p := range g.Players {
		ps := PlayerState{
			ID:           p.ID,
			Name:         p.Name,
			AvatarURL:    p.AvatarURL,
			Seat:         p.Seat,
			Chips:        p.Chips,
			Bet:          p.Bet,      // m2: current round bet
			TotalBet:     p.TotalBet, // m2: total bet for the hand
			Folded:       p.Folded,
			AllIn:        p.AllIn,
			Eliminated:   p.Eliminated,
			Disconnected: p.Disconnected,
		}
		// Only show hole cards to the player themselves
		if p.ID == playerID {
			ps.Hole = p.Hole
		}
		state.Players = append(state.Players, ps)
	}

	if g.CurrentActor() == playerID {
		state.ValidActions = g.ValidActions()
	}

	return state
}

// GetSpectatorState returns the game state with all cards visible (god view).
func (g *Game) GetSpectatorState() GameState {
	state := g.GetGameState("")
	for i, p := range g.Players {
		state.Players[i].Hole = p.Hole
	}
	return state
}

// --- Action execution ---

func (g *Game) executeAction(p *Player, action Action) error {
	toCall := g.CurrentBet - p.Bet
	var cost int // m1: actual chips invested in this action

	switch action.Type {
	case ActionFold:
		// C1: allow fold at any time (removed toCall<=0 restriction)
		p.Folded = true

	case ActionCheck:
		if toCall > 0 {
			return fmt.Errorf("%w: must call %d or fold", ErrInvalidAction, toCall)
		}

	case ActionCall:
		if toCall <= 0 {
			return fmt.Errorf("%w: nothing to call", ErrInvalidAction)
		}
		// M4: auto all-in if can't afford full call
		if toCall >= p.Chips {
			cost = p.Chips
			p.Bet += cost
			p.TotalBet += cost
			p.Chips = 0
			p.AllIn = true
		} else {
			cost = toCall
			p.Chips -= cost
			p.Bet += cost
			p.TotalBet += cost
		}

	case ActionRaise:
		raiseTo := action.Amount
		minRaiseTo := g.CurrentBet + g.MinRaise
		maxRaiseTo := p.Bet + p.Chips

		if raiseTo < minRaiseTo {
			return fmt.Errorf("%w: raise must be at least %d, got %d", ErrInvalidAction, minRaiseTo, raiseTo)
		}
		if raiseTo > maxRaiseTo {
			return fmt.Errorf("%w: raise cannot exceed %d, got %d", ErrInvalidAction, maxRaiseTo, raiseTo)
		}

		raiseIncrement := raiseTo - g.CurrentBet
		cost = raiseTo - p.Bet

		p.Chips -= cost
		p.Bet += cost
		p.TotalBet += cost
		g.MinRaise = raiseIncrement
		g.CurrentBet = raiseTo

		if p.Chips == 0 {
			p.AllIn = true
		}

		// Raise reopens action for all other active players
		for _, other := range g.Players {
			if other.Seat != p.Seat && other.IsActive() {
				other.HasActed = false
			}
		}

	case ActionAllIn:
		cost = p.Chips
		if cost <= 0 {
			return fmt.Errorf("%w: no chips to go all-in", ErrInvalidAction)
		}
		newTotalBet := p.Bet + cost

		if newTotalBet > g.CurrentBet {
			raiseIncrement := newTotalBet - g.CurrentBet
			if raiseIncrement >= g.MinRaise {
				// Full raise - reopens action
				g.MinRaise = raiseIncrement
				for _, other := range g.Players {
					if other.Seat != p.Seat && other.IsActive() {
						other.HasActed = false
					}
				}
			}
			// Under-raise: does NOT reopen action
			g.CurrentBet = newTotalBet
		}

		p.Bet += cost
		p.TotalBet += cost
		p.Chips = 0
		p.AllIn = true

	default:
		return fmt.Errorf("%w: unknown action type %q", ErrInvalidAction, action.Type)
	}

	p.HasActed = true

	// m1: Amount is the actual chips invested in this action
	g.emit(EventPlayerAction, PlayerActionData{
		Seat:      p.Seat,
		PlayerID:  p.ID,
		Action:    action.Type,
		Amount:    cost,
		ChipsLeft: p.Chips,
		TotalPot:  g.totalPot(),
		Reason:    action.Reason,
	})

	return nil
}

// --- Action advancement ---

func (g *Game) advanceAction() {
	// Check if only 1 player remains in hand
	if g.playersInHand() <= 1 {
		g.endHandEarly()
		return
	}

	// Check if betting round is complete
	if g.isBettingRoundComplete() {
		g.advancePhase()
		return
	}

	// Find next player to act
	next := g.findNextActor()
	if next == -1 {
		g.advancePhase()
		return
	}
	g.ActionIdx = next
}

func (g *Game) isBettingRoundComplete() bool {
	for _, p := range g.Players {
		if p.IsActive() && !p.HasActed {
			return false
		}
	}
	return true
}

func (g *Game) findNextActor() int {
	n := len(g.Players)
	for i := 1; i < n; i++ {
		idx := (g.ActionIdx + i) % n
		p := g.Players[idx]
		if p.IsActive() && !p.HasActed {
			return idx
		}
	}
	return -1
}

func (g *Game) advancePhase() {
	switch g.Phase {
	case PhasePreflop:
		g.Phase = PhaseFlop
		g.dealCommunity(3, "flop")
	case PhaseFlop:
		g.Phase = PhaseTurn
		g.dealCommunity(1, "turn")
	case PhaseTurn:
		g.Phase = PhaseRiver
		g.dealCommunity(1, "river")
	case PhaseRiver:
		g.Phase = PhaseShowdown
		g.doShowdown()
		return
	}

	// Start new betting round
	g.startBettingRound()

	// If no active players can act (all all-in), run to showdown
	if g.activePlayersCount() <= 1 && g.playersInHand() > 1 {
		g.runToShowdown()
	}
}

func (g *Game) startBettingRound() {
	for _, p := range g.Players {
		p.Bet = 0
		p.HasActed = false
	}
	g.CurrentBet = 0
	_, bb := g.Blinds.GetBlinds(g.HandNum)
	g.MinRaise = bb

	g.ActionIdx = g.nextActiveAfter(g.DealerIdx)
}

// M1: burn a card before dealing community cards
func (g *Game) dealCommunity(count int, phase string) {
	g.deck.Draw(1) // burn card
	cards := g.deck.Draw(count)
	g.Community = append(g.Community, cards...)
	g.emit(EventCommunityDealt, CommunityDealtData{
		Phase: phase,
		Cards: cards,
		Board: g.Community,
	})
}

func (g *Game) runToShowdown() {
	// Deal remaining community cards without betting (each with burn)
	switch g.Phase {
	case PhasePreflop:
		g.dealCommunity(3, "flop")
		g.dealCommunity(1, "turn")
		g.dealCommunity(1, "river")
	case PhaseFlop:
		g.dealCommunity(1, "turn")
		g.dealCommunity(1, "river")
	case PhaseTurn:
		g.dealCommunity(1, "river")
	}
	g.Phase = PhaseShowdown
	g.doShowdown()
}

// --- Showdown ---

func (g *Game) doShowdown() {
	inHand := g.playersInHandSlice()

	// Evaluate all hands
	pockets := make([][]cardrank.Card, len(inHand))
	for i, p := range inHand {
		pockets[i] = p.Hole
	}

	evals := cardrank.Holdem.EvalPockets(pockets, g.Community)

	// Emit showdown event
	showdownPlayers := make([]ShowdownPlayer, len(inHand))
	for i, p := range inHand {
		showdownPlayers[i] = ShowdownPlayer{
			Seat:     p.Seat,
			PlayerID: p.ID,
			Hole:     p.Hole,
			HandDesc: fmt.Sprintf("%s", evals[i]),
			HandRank: int(evals[i].HiRank),
		}
	}
	g.emit(EventShowdown, ShowdownData{
		Players: showdownPlayers,
		Board:   g.Community,
	})

	// Calculate and award pots
	pots := CalculatePots(g.Players)

	for potIdx, pot := range pots {
		// Find evaluations for eligible players
		var eligibleEvals []*cardrank.Eval
		var eligiblePlayers []*Player

		for _, seat := range pot.Eligible {
			for i, p := range inHand {
				if p.Seat == seat {
					eligibleEvals = append(eligibleEvals, evals[i])
					eligiblePlayers = append(eligiblePlayers, p)
					break
				}
			}
		}

		if len(eligiblePlayers) == 0 {
			continue
		}

		// Find winner(s)
		winners := findPotWinners(eligiblePlayers, eligibleEvals)

		// C2: sort winners by clockwise distance from dealer (left of button first)
		// so the odd chip goes to the first player left of the button
		n := len(g.Players)
		sort.Slice(winners, func(i, j int) bool {
			distI := (winners[i].Seat - g.DealerIdx - 1 + n) % n
			distJ := (winners[j].Seat - g.DealerIdx - 1 + n) % n
			return distI < distJ
		})

		// Split pot among winners
		share := pot.Amount / len(winners)
		remainder := pot.Amount % len(winners)

		potWinners := make([]PotWinner, len(winners))
		for i, w := range winners {
			winAmount := share
			if i == 0 {
				winAmount += remainder // odd chip to first player left of button
			}
			w.Chips += winAmount
			potWinners[i] = PotWinner{
				Seat:     w.Seat,
				PlayerID: w.ID,
				Amount:   winAmount,
			}
		}

		g.emit(EventPotAwarded, PotAwardedData{
			PotIndex: potIdx,
			Amount:   pot.Amount,
			Winners:  potWinners,
		})
	}

	g.endHand()
}

func findPotWinners(players []*Player, evals []*cardrank.Eval) []*Player {
	if len(players) == 0 {
		return nil
	}

	bestIdx := 0
	for i := 1; i < len(evals); i++ {
		if evals[i].Comp(evals[bestIdx], false) < 0 {
			bestIdx = i
		}
	}

	var winners []*Player
	for i, ev := range evals {
		if ev.Comp(evals[bestIdx], false) == 0 {
			winners = append(winners, players[i])
		}
	}
	return winners
}

// --- Hand ending ---

func (g *Game) endHandEarly() {
	var winner *Player
	for _, p := range g.Players {
		if p.IsInHand() {
			winner = p
			break
		}
	}

	if winner != nil {
		totalPot := g.totalPot()
		winner.Chips += totalPot

		g.emit(EventPotAwarded, PotAwardedData{
			PotIndex: 0,
			Amount:   totalPot,
			Winners: []PotWinner{{
				Seat:     winner.Seat,
				PlayerID: winner.ID,
				Amount:   totalPot,
			}},
		})
	}

	g.endHand()
}

func (g *Game) endHand() {
	// C3: collect all players eliminated this hand, sort by StartChips for ranking
	var eliminated []*Player
	for _, p := range g.Players {
		if !p.Eliminated && p.Chips <= 0 {
			eliminated = append(eliminated, p)
		}
	}

	// Sort by StartChips descending: bigger starting stack = better rank among co-eliminated
	sort.Slice(eliminated, func(i, j int) bool {
		return eliminated[i].StartChips > eliminated[j].StartChips
	})

	aliveAfter := g.alivePlayers() - len(eliminated)

	for i, p := range eliminated {
		p.Eliminated = true
		p.EliminatedAt = g.HandNum
		rank := aliveAfter + 1 + i
		g.emit(EventPlayerEliminated, PlayerEliminatedData{
			Seat:     p.Seat,
			PlayerID: p.ID,
			Rank:     rank,
		})
	}

	// Emit hand end
	g.emit(EventHandEnd, HandEndData{
		HandNum: g.HandNum,
		Players: g.makePlayerInfos(),
	})

	// Check if game is over
	if g.alivePlayers() <= 1 {
		g.Finished = true
		var winner *Player
		for _, p := range g.Players {
			if !p.Eliminated {
				winner = p
				break
			}
		}
		if winner != nil {
			g.emit(EventGameOver, GameOverData{
				WinnerSeat: winner.Seat,
				WinnerID:   winner.ID,
				Rankings:   g.buildRankings(),
			})
		}
	}

	g.Phase = PhaseIdle
	g.ActionIdx = -1
}

// PlayerByID returns the player with the given ID, or nil.
func (g *Game) PlayerByID(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// --- Valid actions ---

func (g *Game) validActionsFor(p *Player) []ActionOption {
	var options []ActionOption
	toCall := g.CurrentBet - p.Bet

	if toCall <= 0 {
		// C1: always allow fold, even when there's nothing to call
		options = append(options, ActionOption{Type: ActionFold})
		options = append(options, ActionOption{Type: ActionCheck})

		if p.Chips > 0 {
			minRaiseTo := g.CurrentBet + g.MinRaise
			maxRaiseTo := p.Bet + p.Chips

			if maxRaiseTo >= minRaiseTo {
				options = append(options, ActionOption{
					Type:      ActionRaise,
					MinAmount: minRaiseTo,
					MaxAmount: maxRaiseTo,
				})
			} else {
				// Can't make minimum raise, but can still all-in
				options = append(options, ActionOption{
					Type:      ActionAllIn,
					MinAmount: maxRaiseTo,
					MaxAmount: maxRaiseTo,
				})
			}
		}
	} else {
		// There's a bet to match
		options = append(options, ActionOption{Type: ActionFold})

		if p.Chips <= toCall {
			// Can only fold or all-in (can't fully call)
			options = append(options, ActionOption{
				Type:      ActionAllIn,
				MinAmount: p.Bet + p.Chips,
				MaxAmount: p.Bet + p.Chips,
			})
		} else {
			// Can call
			options = append(options, ActionOption{
				Type:     ActionCall,
				CallCost: toCall,
			})

			// Can raise
			minRaiseTo := g.CurrentBet + g.MinRaise
			maxRaiseTo := p.Bet + p.Chips

			if maxRaiseTo >= minRaiseTo {
				options = append(options, ActionOption{
					Type:      ActionRaise,
					MinAmount: minRaiseTo,
					MaxAmount: maxRaiseTo,
				})
			} else if p.Chips > toCall {
				// Can't min raise but has more than call amount - all-in as raise
				options = append(options, ActionOption{
					Type:      ActionAllIn,
					MinAmount: maxRaiseTo,
					MaxAmount: maxRaiseTo,
				})
			}
		}
	}

	return options
}

// --- Helpers ---

func (g *Game) emit(t EventType, data interface{}) {
	g.Events = append(g.Events, Event{
		Type:    t,
		HandNum: g.HandNum,
		Data:    data,
	})
}

// M3: Dead Button — sbIdx accounts for possible dead button
func (g *Game) sbIdx() int {
	if g.alivePlayers() == 2 && !g.Players[g.DealerIdx].Eliminated {
		return g.DealerIdx // heads-up with live button: dealer is SB
	}
	return g.nextSeatAfter(g.DealerIdx)
}

func (g *Game) bbIdx() int {
	return g.nextSeatAfter(g.sbIdx())
}

// nextSeatAfter returns the next non-eliminated player seat.
func (g *Game) nextSeatAfter(seat int) int {
	n := len(g.Players)
	for i := 1; i < n; i++ {
		idx := (seat + i) % n
		if !g.Players[idx].Eliminated {
			return idx
		}
	}
	return seat
}

// nextActiveAfter returns the next player who can act.
func (g *Game) nextActiveAfter(seat int) int {
	n := len(g.Players)
	for i := 1; i < n; i++ {
		idx := (seat + i) % n
		if g.Players[idx].IsActive() {
			return idx
		}
	}
	return -1
}

func (g *Game) alivePlayers() int {
	count := 0
	for _, p := range g.Players {
		if !p.Eliminated {
			count++
		}
	}
	return count
}

func (g *Game) playersInHand() int {
	count := 0
	for _, p := range g.Players {
		if p.IsInHand() {
			count++
		}
	}
	return count
}

func (g *Game) playersInHandSlice() []*Player {
	var result []*Player
	for _, p := range g.Players {
		if p.IsInHand() {
			result = append(result, p)
		}
	}
	return result
}

func (g *Game) activePlayersCount() int {
	count := 0
	for _, p := range g.Players {
		if p.IsActive() {
			count++
		}
	}
	return count
}

func (g *Game) totalPot() int {
	total := 0
	for _, p := range g.Players {
		total += p.TotalBet
	}
	return total
}

func (g *Game) currentPots() []Pot {
	pots := CalculatePots(g.Players)
	if pots == nil {
		return []Pot{}
	}
	return pots
}

func (g *Game) makePlayerInfos() []PlayerInfo {
	infos := make([]PlayerInfo, 0)
	for _, p := range g.Players {
		if !p.Eliminated {
			infos = append(infos, PlayerInfo{
				ID:    p.ID,
				Seat:  p.Seat,
				Chips: p.Chips,
			})
		}
	}
	return infos
}

func (g *Game) postBlind(seat, amount int) {
	p := g.Players[seat]
	actual := amount
	if actual > p.Chips {
		actual = p.Chips
	}
	p.Chips -= actual
	p.Bet = actual
	p.TotalBet = actual
	if p.Chips == 0 {
		p.AllIn = true
	}
}

// GetRankings returns the final tournament rankings. Only valid when Finished is true.
func (g *Game) GetRankings() []RankingEntry {
	return g.buildRankings()
}

// C3: buildRankings with StartChips sub-sort for same-hand eliminations
func (g *Game) buildRankings() []RankingEntry {
	rankings := make([]RankingEntry, 0, len(g.Players))

	// Winner is the non-eliminated player
	for _, p := range g.Players {
		if !p.Eliminated {
			rankings = append(rankings, RankingEntry{Rank: 1, Seat: p.Seat, PlayerID: p.ID})
		}
	}

	// Eliminated players sorted by EliminatedAt descending (later = better rank)
	// For same-hand eliminations, higher StartChips = better rank
	type elimInfo struct {
		seat         int
		playerID     string
		eliminatedAt int
		startChips   int
	}
	var eliminated []elimInfo
	for _, p := range g.Players {
		if p.Eliminated {
			eliminated = append(eliminated, elimInfo{p.Seat, p.ID, p.EliminatedAt, p.StartChips})
		}
	}
	sort.Slice(eliminated, func(i, j int) bool {
		if eliminated[i].eliminatedAt != eliminated[j].eliminatedAt {
			return eliminated[i].eliminatedAt > eliminated[j].eliminatedAt
		}
		return eliminated[i].startChips > eliminated[j].startChips
	})
	for i, e := range eliminated {
		rankings = append(rankings, RankingEntry{
			Rank:     i + 2,
			Seat:     e.seat,
			PlayerID: e.playerID,
		})
	}

	return rankings
}
