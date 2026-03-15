package poker

import (
	"testing"
)

// TestBlindSchedule verifies blind level escalation.
func TestBlindSchedule(t *testing.T) {
	s := DefaultSchedule

	tests := []struct {
		handNum  int
		wantSB   int
		wantBB   int
	}{
		{1, 40, 80},
		{6, 40, 80},
		{7, 80, 160},
		{12, 80, 160},
		{13, 160, 320},
		{19, 320, 640},
		{25, 640, 1280},
		{31, 1280, 2560},
		{50, 1280, 2560}, // stays at max level
	}

	for _, tt := range tests {
		sb, bb := s.GetBlinds(tt.handNum)
		if sb != tt.wantSB || bb != tt.wantBB {
			t.Errorf("GetBlinds(%d) = (%d, %d), want (%d, %d)",
				tt.handNum, sb, bb, tt.wantSB, tt.wantBB)
		}
	}
}

// TestPotCalculation verifies side pot calculation.
func TestPotCalculation(t *testing.T) {
	t.Run("simple pot no side pots", func(t *testing.T) {
		players := []*Player{
			{Seat: 0, TotalBet: 100},
			{Seat: 1, TotalBet: 100},
			{Seat: 2, TotalBet: 100},
		}
		pots := CalculatePots(players)
		if len(pots) != 1 {
			t.Fatalf("expected 1 pot, got %d", len(pots))
		}
		if pots[0].Amount != 300 {
			t.Errorf("pot amount = %d, want 300", pots[0].Amount)
		}
		if len(pots[0].Eligible) != 3 {
			t.Errorf("eligible = %d, want 3", len(pots[0].Eligible))
		}
	})

	t.Run("one player all-in short", func(t *testing.T) {
		players := []*Player{
			{Seat: 0, TotalBet: 50, AllIn: true},  // short stack all-in
			{Seat: 1, TotalBet: 100},
			{Seat: 2, TotalBet: 100},
		}
		pots := CalculatePots(players)
		if len(pots) != 2 {
			t.Fatalf("expected 2 pots, got %d: %+v", len(pots), pots)
		}
		// Main pot: 50 * 3 = 150 (all 3 eligible)
		if pots[0].Amount != 150 {
			t.Errorf("main pot = %d, want 150", pots[0].Amount)
		}
		if len(pots[0].Eligible) != 3 {
			t.Errorf("main pot eligible = %d, want 3", len(pots[0].Eligible))
		}
		// Side pot: 50 * 2 = 100 (seats 1,2 eligible)
		if pots[1].Amount != 100 {
			t.Errorf("side pot = %d, want 100", pots[1].Amount)
		}
		if len(pots[1].Eligible) != 2 {
			t.Errorf("side pot eligible = %d, want 2", len(pots[1].Eligible))
		}
	})

	t.Run("folded player contributes but not eligible", func(t *testing.T) {
		players := []*Player{
			{Seat: 0, TotalBet: 50, Folded: true}, // folded after betting 50
			{Seat: 1, TotalBet: 100},
			{Seat: 2, TotalBet: 100},
		}
		pots := CalculatePots(players)
		// All money goes to pot contested by seats 1 and 2
		totalPot := 0
		for _, p := range pots {
			totalPot += p.Amount
		}
		if totalPot != 250 {
			t.Errorf("total pot = %d, want 250", totalPot)
		}
	})

	t.Run("three-way all-in different amounts", func(t *testing.T) {
		players := []*Player{
			{Seat: 0, TotalBet: 100, AllIn: true},
			{Seat: 1, TotalBet: 200, AllIn: true},
			{Seat: 2, TotalBet: 300},
		}
		pots := CalculatePots(players)
		if len(pots) != 3 {
			t.Fatalf("expected 3 pots, got %d: %+v", len(pots), pots)
		}
		// Main pot: 100 * 3 = 300
		if pots[0].Amount != 300 {
			t.Errorf("pot[0] = %d, want 300", pots[0].Amount)
		}
		// Side pot 1: 100 * 2 = 200
		if pots[1].Amount != 200 {
			t.Errorf("pot[1] = %d, want 200", pots[1].Amount)
		}
		// Side pot 2: 100 * 1 = 100
		if pots[2].Amount != 100 {
			t.Errorf("pot[2] = %d, want 100", pots[2].Amount)
		}
	})
}

// TestNewGame verifies game creation.
func TestNewGame(t *testing.T) {
	g := NewGame("test-1", []string{"agent_a", "agent_b", "agent_c"}, 42, nil, nil)

	if g.ID != "test-1" {
		t.Errorf("ID = %q, want %q", g.ID, "test-1")
	}
	if len(g.Players) != 3 {
		t.Fatalf("players = %d, want 3", len(g.Players))
	}
	for i, p := range g.Players {
		if p.Chips != StartingChips {
			t.Errorf("player %d chips = %d, want %d", i, p.Chips, StartingChips)
		}
		if p.Seat != i {
			t.Errorf("player %d seat = %d, want %d", i, p.Seat, i)
		}
	}
	if g.Phase != PhaseIdle {
		t.Errorf("phase = %v, want idle", g.Phase)
	}
}

// TestStartHand verifies hand initialization.
func TestStartHand(t *testing.T) {
	g := NewGame("test-2", []string{"a", "b", "c"}, 42, nil, nil)

	events, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events from StartHand")
	}

	// Verify phase
	if g.Phase != PhasePreflop {
		t.Errorf("phase = %v, want preflop", g.Phase)
	}

	// Verify hand number
	if g.HandNum != 1 {
		t.Errorf("HandNum = %d, want 1", g.HandNum)
	}

	// Verify blinds posted
	// Dealer = seat 0, SB = seat 1, BB = seat 2 (3 players)
	sb := g.Players[1]
	bb := g.Players[2]
	if sb.Bet != 40 {
		t.Errorf("SB bet = %d, want 40", sb.Bet)
	}
	if bb.Bet != 80 {
		t.Errorf("BB bet = %d, want 80", bb.Bet)
	}
	if sb.Chips != StartingChips-40 {
		t.Errorf("SB chips = %d, want %d", sb.Chips, StartingChips-40)
	}

	// Verify hole cards dealt
	for _, p := range g.Players {
		if len(p.Hole) != 2 {
			t.Errorf("player %d has %d hole cards, want 2", p.Seat, len(p.Hole))
		}
	}

	// Verify first to act is UTG (seat 0, after BB seat 2)
	if g.CurrentActor() != "a" {
		t.Errorf("current actor = %q, want %q", g.CurrentActor(), "a")
	}
}

// TestHeadsUpBlinds verifies heads-up blind posting (dealer is SB).
func TestHeadsUpBlinds(t *testing.T) {
	g := NewGame("hu-1", []string{"a", "b"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// Heads-up: dealer (seat 0) is SB, seat 1 is BB
	if g.Players[0].Bet != 40 {
		t.Errorf("dealer/SB bet = %d, want 40", g.Players[0].Bet)
	}
	if g.Players[1].Bet != 80 {
		t.Errorf("BB bet = %d, want 80", g.Players[1].Bet)
	}

	// In heads-up, SB/dealer acts first preflop
	if g.CurrentActor() != "a" {
		t.Errorf("current actor = %q, want %q (SB acts first in HU preflop)", g.CurrentActor(), "a")
	}
}

// TestSimpleHandAllFold verifies a hand where everyone folds to the last player.
func TestSimpleHandAllFold(t *testing.T) {
	g := NewGame("fold-1", []string{"a", "b", "c"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// UTG (a, seat 0) folds
	_, err = g.Act("a", Action{Type: ActionFold})
	if err != nil {
		t.Fatalf("UTG fold error: %v", err)
	}

	// SB (b, seat 1) folds
	_, err = g.Act("b", Action{Type: ActionFold})
	if err != nil {
		t.Fatalf("SB fold error: %v", err)
	}

	// Hand should be over, BB (c) wins
	if !g.IsHandOver() {
		t.Fatal("expected hand to be over")
	}

	// BB should have won the blinds
	expectedChips := StartingChips - 80 + 120 // BB posted 80, won 40+80=120 total pot
	if g.Players[2].Chips != expectedChips {
		t.Errorf("BB chips = %d, want %d", g.Players[2].Chips, expectedChips)
	}
}

// TestCheckDownToShowdown verifies a hand that checks all the way to showdown.
func TestCheckDownToShowdown(t *testing.T) {
	g := NewGame("check-1", []string{"a", "b"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// Preflop: SB/dealer calls (in HU, dealer/SB acts first)
	_, err = g.Act("a", Action{Type: ActionCall})
	if err != nil {
		t.Fatalf("SB call error: %v", err)
	}

	// BB checks
	_, err = g.Act("b", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("BB check error: %v", err)
	}

	// Should be on flop now
	if g.Phase != PhaseFlop {
		t.Fatalf("phase = %v, want flop", g.Phase)
	}
	if len(g.Community) != 3 {
		t.Fatalf("community = %d cards, want 3", len(g.Community))
	}

	// Flop: both check (in post-flop, BB acts first in HU... wait)
	// In heads-up post-flop: BB acts first (non-dealer acts first)
	// Dealer = seat 0, so seat 1 (BB) should act first
	if g.CurrentActor() != "b" {
		t.Fatalf("flop actor = %q, want b (BB acts first post-flop in HU)", g.CurrentActor())
	}

	_, err = g.Act("b", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("BB check flop error: %v", err)
	}
	_, err = g.Act("a", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("SB check flop error: %v", err)
	}

	// Turn
	if g.Phase != PhaseTurn {
		t.Fatalf("phase = %v, want turn", g.Phase)
	}
	if len(g.Community) != 4 {
		t.Fatalf("community = %d cards, want 4", len(g.Community))
	}

	_, err = g.Act("b", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("BB check turn error: %v", err)
	}
	_, err = g.Act("a", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("SB check turn error: %v", err)
	}

	// River
	if g.Phase != PhaseRiver {
		t.Fatalf("phase = %v, want river", g.Phase)
	}
	if len(g.Community) != 5 {
		t.Fatalf("community = %d cards, want 5", len(g.Community))
	}

	_, err = g.Act("b", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("BB check river error: %v", err)
	}
	_, err = g.Act("a", Action{Type: ActionCheck})
	if err != nil {
		t.Fatalf("SB check river error: %v", err)
	}

	// Should go to showdown and then hand over
	if !g.IsHandOver() {
		t.Fatal("expected hand to be over after showdown")
	}

	// Total chips should be conserved
	totalChips := 0
	for _, p := range g.Players {
		totalChips += p.Chips
	}
	if totalChips != StartingChips*2 {
		t.Errorf("total chips = %d, want %d", totalChips, StartingChips*2)
	}
}

// TestRaiseAndCall verifies raise/call action flow.
func TestRaiseAndCall(t *testing.T) {
	g := NewGame("raise-1", []string{"a", "b", "c"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// UTG raises to 240
	_, err = g.Act("a", Action{Type: ActionRaise, Amount: 240})
	if err != nil {
		t.Fatalf("UTG raise error: %v", err)
	}

	// SB calls 240 (needs to add 200 more, since SB posted 40)
	_, err = g.Act("b", Action{Type: ActionCall})
	if err != nil {
		t.Fatalf("SB call error: %v", err)
	}

	// BB calls 240 (needs to add 160 more, since BB posted 80)
	_, err = g.Act("c", Action{Type: ActionCall})
	if err != nil {
		t.Fatalf("BB call error: %v", err)
	}

	// Should advance to flop
	if g.Phase != PhaseFlop {
		t.Errorf("phase = %v, want flop", g.Phase)
	}

	// Total pot should be 240 * 3 = 720
	pot := g.totalPot()
	if pot != 720 {
		t.Errorf("total pot = %d, want 720", pot)
	}
}

// TestAllInShowdown verifies all-in leading to showdown.
func TestAllInShowdown(t *testing.T) {
	g := NewGame("allin-1", []string{"a", "b"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// SB goes all-in
	_, err = g.Act("a", Action{Type: ActionAllIn})
	if err != nil {
		t.Fatalf("SB all-in error: %v", err)
	}

	// BB calls all-in
	_, err = g.Act("b", Action{Type: ActionAllIn})
	if err != nil {
		t.Fatalf("BB call all-in error: %v", err)
	}

	// Hand should complete (showdown happens automatically)
	if !g.IsHandOver() {
		t.Fatal("expected hand to be over after double all-in")
	}

	// Community should have 5 cards
	if len(g.Community) != 5 {
		t.Errorf("community = %d cards, want 5", len(g.Community))
	}

	// Total chips conserved
	totalChips := 0
	for _, p := range g.Players {
		totalChips += p.Chips
	}
	if totalChips != StartingChips*2 {
		t.Errorf("total chips = %d, want %d", totalChips, StartingChips*2)
	}

	// One player should have all chips, other should be eliminated
	if !g.IsGameOver() {
		t.Error("expected game to be over (HU all-in)")
	}
}

// TestFullTournament simulates a full tournament until one player remains.
// Uses a mixed strategy (occasionally raising) to create larger pots and faster eliminations.
func TestFullTournament(t *testing.T) {
	g := NewGame("tourney-1", []string{"a", "b", "c", "d", "e", "f"}, 12345, nil, nil)

	totalStartChips := StartingChips * 6
	handCount := 0
	maxHands := 2000 // safety limit

	for !g.IsGameOver() && handCount < maxHands {
		_, err := g.StartHand()
		if err != nil {
			t.Fatalf("hand %d StartHand error: %v", handCount+1, err)
		}
		handCount++

		actionCount := 0
		for !g.IsHandOver() && actionCount < 100 {
			actor := g.CurrentActor()
			if actor == "" {
				break
			}

			actions := g.ValidActions()
			if len(actions) == 0 {
				break
			}

			// Mixed strategy: occasionally raise/all-in to speed up eliminations
			var chosen Action
			if handCount%5 == 0 && actionCount == 0 {
				// Every 5th hand, first actor goes all-in
				for _, a := range actions {
					if a.Type == ActionAllIn {
						chosen = Action{Type: ActionAllIn}
						break
					}
				}
			}
			if chosen.Type == "" {
				// Default: check > call > allin > fold
				for _, a := range actions {
					if a.Type == ActionCheck {
						chosen = Action{Type: ActionCheck}
						break
					}
					if a.Type == ActionCall {
						chosen = Action{Type: ActionCall}
						break
					}
					if a.Type == ActionAllIn {
						chosen = Action{Type: ActionAllIn}
						break
					}
				}
			}
			if chosen.Type == "" {
				chosen = Action{Type: ActionFold}
			}

			_, err := g.Act(actor, chosen)
			if err != nil {
				t.Fatalf("hand %d, action by %s error: %v (action: %+v, valid: %+v)",
					handCount, actor, err, chosen, actions)
			}
			actionCount++
		}

		// Verify chip conservation after hand
		totalChips := 0
		for _, p := range g.Players {
			totalChips += p.Chips
		}
		if totalChips != totalStartChips {
			t.Fatalf("hand %d: chip leak after hand! total=%d, want %d", handCount, totalChips, totalStartChips)
		}
	}

	if !g.IsGameOver() {
		// Debug: print alive players and their stacks
		for _, p := range g.Players {
			if !p.Eliminated {
				t.Logf("alive: seat=%d chips=%d", p.Seat, p.Chips)
			}
		}
		t.Fatalf("game not over after %d hands", maxHands)
	}

	// Verify chip conservation
	totalChips := 0
	for _, p := range g.Players {
		totalChips += p.Chips
	}
	if totalChips != totalStartChips {
		t.Errorf("total chips = %d, want %d (chip leak!)", totalChips, totalStartChips)
	}

	// Verify exactly one player alive
	alive := 0
	for _, p := range g.Players {
		if !p.Eliminated {
			alive++
		}
	}
	if alive != 1 {
		t.Errorf("alive players = %d, want 1", alive)
	}

	t.Logf("Tournament completed in %d hands", handCount)
}

// TestInvalidActions verifies that invalid actions are rejected.
func TestInvalidActions(t *testing.T) {
	g := NewGame("invalid-1", []string{"a", "b", "c"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// Wrong player tries to act
	_, err = g.Act("b", Action{Type: ActionFold})
	if err == nil {
		t.Error("expected error for wrong player acting")
	}

	// UTG tries to check (there's a BB to match)
	_, err = g.Act("a", Action{Type: ActionCheck})
	if err == nil {
		t.Error("expected error for checking when there's a bet")
	}

	// UTG tries to raise below minimum
	_, err = g.Act("a", Action{Type: ActionRaise, Amount: 120}) // min is 160 (80+80)
	if err == nil {
		t.Error("expected error for under-minimum raise")
	}
}

// TestDealerRotation verifies dealer button moves correctly.
func TestDealerRotation(t *testing.T) {
	g := NewGame("rotate-1", []string{"a", "b", "c"}, 42, nil, nil)

	// Hand 1: dealer at seat 0
	g.StartHand()
	if g.DealerIdx != 0 {
		t.Errorf("hand 1 dealer = %d, want 0", g.DealerIdx)
	}

	// Complete hand by folding
	g.Act("a", Action{Type: ActionFold})
	g.Act("b", Action{Type: ActionFold})

	// Hand 2: dealer at seat 1
	g.StartHand()
	if g.DealerIdx != 1 {
		t.Errorf("hand 2 dealer = %d, want 1", g.DealerIdx)
	}

	g.Act("b", Action{Type: ActionFold})
	g.Act("c", Action{Type: ActionFold})

	// Hand 3: dealer at seat 2
	g.StartHand()
	if g.DealerIdx != 2 {
		t.Errorf("hand 3 dealer = %d, want 2", g.DealerIdx)
	}
}

// TestValidActionsOptions verifies valid action generation.
func TestValidActionsOptions(t *testing.T) {
	g := NewGame("valid-1", []string{"a", "b", "c"}, 42, nil, nil)

	_, err := g.StartHand()
	if err != nil {
		t.Fatalf("StartHand error: %v", err)
	}

	// UTG should have: fold, call, raise, allin
	actions := g.ValidActions()
	hasAction := func(at ActionType) bool {
		for _, a := range actions {
			if a.Type == at {
				return true
			}
		}
		return false
	}

	if !hasAction(ActionFold) {
		t.Error("UTG should be able to fold")
	}
	if !hasAction(ActionCall) {
		t.Error("UTG should be able to call")
	}
	if !hasAction(ActionRaise) {
		t.Error("UTG should be able to raise")
	}

	// Check raise bounds
	for _, a := range actions {
		if a.Type == ActionRaise {
			if a.MinAmount != 160 { // BB=80, min raise = 80+80 = 160
				t.Errorf("min raise = %d, want 160", a.MinAmount)
			}
			if a.MaxAmount != StartingChips { // can bet up to all chips
				t.Errorf("max raise = %d, want %d", a.MaxAmount, StartingChips)
			}
		}
	}
}
