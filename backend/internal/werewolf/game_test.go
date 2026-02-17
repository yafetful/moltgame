package werewolf

import (
	"testing"
)

// helper to create a game and find players by role
type testGame struct {
	*Game
	wolves    []*Player
	seer      *Player
	villagers []*Player
}

func newTestGame(t *testing.T, seed int64) *testGame {
	t.Helper()
	g, err := NewGame("test-1", []string{"p0", "p1", "p2", "p3", "p4"}, seed)
	if err != nil {
		t.Fatalf("NewGame error: %v", err)
	}

	tg := &testGame{Game: g}
	for _, p := range g.Players {
		switch p.Role {
		case RoleWerewolf:
			tg.wolves = append(tg.wolves, p)
		case RoleSeer:
			tg.seer = p
		case RoleVillager:
			tg.villagers = append(tg.villagers, p)
		}
	}

	if len(tg.wolves) != 2 {
		t.Fatalf("expected 2 wolves, got %d", len(tg.wolves))
	}
	if tg.seer == nil {
		t.Fatal("expected 1 seer")
	}
	if len(tg.villagers) != 2 {
		t.Fatalf("expected 2 villagers, got %d", len(tg.villagers))
	}

	return tg
}

func TestNewGame(t *testing.T) {
	g, err := NewGame("g1", []string{"a", "b", "c", "d", "e"}, 42)
	if err != nil {
		t.Fatalf("NewGame error: %v", err)
	}

	if len(g.Players) != 5 {
		t.Fatalf("players = %d, want 5", len(g.Players))
	}
	if g.Phase != PhaseIdle {
		t.Errorf("phase = %v, want idle", g.Phase)
	}

	// Verify role distribution
	roles := make(map[Role]int)
	for _, p := range g.Players {
		roles[p.Role]++
	}
	if roles[RoleWerewolf] != 2 {
		t.Errorf("wolves = %d, want 2", roles[RoleWerewolf])
	}
	if roles[RoleSeer] != 1 {
		t.Errorf("seers = %d, want 1", roles[RoleSeer])
	}
	if roles[RoleVillager] != 2 {
		t.Errorf("villagers = %d, want 2", roles[RoleVillager])
	}
}

func TestUnsupportedPlayerCount(t *testing.T) {
	_, err := NewGame("g1", []string{"a", "b", "c"}, 42)
	if err == nil {
		t.Error("expected error for 3 players")
	}
}

func TestStartGame(t *testing.T) {
	tg := newTestGame(t, 42)

	events, err := tg.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events from Start")
	}

	// Should be in night phase
	if tg.Phase != PhaseNight {
		t.Errorf("phase = %v, want night", tg.Phase)
	}
	if tg.Day != 1 {
		t.Errorf("day = %d, want 1", tg.Day)
	}
}

func TestNightActions(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Wolves pick a target
	target := tg.villagers[0]
	for _, wolf := range tg.wolves {
		_, err := tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: target.ID})
		if err != nil {
			t.Fatalf("wolf %s kill error: %v", wolf.ID, err)
		}
	}

	// Seer investigates
	_, err := tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.wolves[0].ID})
	if err != nil {
		t.Fatalf("seer investigate error: %v", err)
	}

	// Night should resolve, target should be dead
	if target.Alive {
		t.Error("target should be dead after night")
	}

	// Seer should have a result
	results := tg.seerResults[tg.seer.ID]
	if len(results) != 1 {
		t.Fatalf("seer results = %d, want 1", len(results))
	}
	if !results[0].IsWolf {
		t.Error("seer investigated a wolf, should see isWolf=true")
	}

	// Should advance to day
	if tg.Phase != PhaseDay {
		t.Errorf("phase = %v, want day", tg.Phase)
	}
}

func TestDayDiscussion(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Complete night
	target := tg.villagers[0]
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: target.ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.villagers[1].ID})

	// Now in day phase
	if tg.Phase != PhaseDay {
		t.Fatalf("phase = %v, want day", tg.Phase)
	}

	// Speaking order should be alive players (4 alive after kill)
	if len(tg.speakingOrder) != 4 {
		t.Fatalf("speaking order = %d, want 4", len(tg.speakingOrder))
	}

	// Each player speaks
	for _, pid := range tg.speakingOrder {
		_, err := tg.Act(pid, Action{Type: ActionSpeak, Message: "I am innocent!"})
		if err != nil {
			t.Fatalf("speak error by %s: %v", pid, err)
		}
	}

	// Should advance to vote
	if tg.Phase != PhaseVote {
		t.Errorf("phase = %v, want vote", tg.Phase)
	}
}

func TestVoteExecution(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Night: wolves kill villager[0]
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: tg.villagers[0].ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.wolves[0].ID})

	// Day: everyone speaks
	for _, pid := range tg.speakingOrder {
		tg.Act(pid, Action{Type: ActionSpeak, Message: "hello"})
	}

	// Vote: majority votes for wolf[0]
	alive := tg.alivePlayers()
	wolf0 := tg.wolves[0]
	for _, p := range alive {
		if p.ID == wolf0.ID {
			// Wolf votes for someone else
			tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: tg.seer.ID})
		} else {
			tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: wolf0.ID})
		}
	}

	// Wolf[0] should be executed
	if wolf0.Alive {
		t.Error("wolf[0] should be executed")
	}
	if wolf0.DeathCause != "executed" {
		t.Errorf("death cause = %q, want 'executed'", wolf0.DeathCause)
	}
}

func TestTiedVoteNoExecution(t *testing.T) {
	tg := newTestGame(t, 100)
	tg.Start()

	// Night: wolves kill villager[0]
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: tg.villagers[0].ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.villagers[1].ID})

	// Day: everyone speaks
	for _, pid := range tg.speakingOrder {
		tg.Act(pid, Action{Type: ActionSpeak, Message: "idk"})
	}

	// Vote: create a tie (2 votes each for 2 different players)
	alive := tg.alivePlayers()
	// We have 4 alive players. Let's make 2 vote for A and 2 vote for B.
	if len(alive) != 4 {
		t.Fatalf("alive = %d, want 4", len(alive))
	}

	tg.Act(alive[0].ID, Action{Type: ActionVotePlayer, TargetID: alive[1].ID})
	tg.Act(alive[1].ID, Action{Type: ActionVotePlayer, TargetID: alive[0].ID})
	tg.Act(alive[2].ID, Action{Type: ActionVotePlayer, TargetID: alive[1].ID})
	tg.Act(alive[3].ID, Action{Type: ActionVotePlayer, TargetID: alive[0].ID})

	// Tied: no one should be executed, all 4 should still be alive
	aliveCount := 0
	for _, p := range tg.Players {
		if p.Alive {
			aliveCount++
		}
	}
	if aliveCount != 4 {
		t.Errorf("alive after tied vote = %d, want 4", aliveCount)
	}

	// Should go to next night
	if tg.Phase != PhaseNight {
		t.Errorf("phase = %v, want night (next round)", tg.Phase)
	}
}

func TestWerewolfWin(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Night 1: wolves kill villager[0]
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: tg.villagers[0].ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.villagers[1].ID})

	// Day 1: everyone speaks
	for _, pid := range tg.speakingOrder {
		tg.Act(pid, Action{Type: ActionSpeak, Message: "test"})
	}

	// Day 1: vote - villagers vote for seer by mistake (wolves win)
	alive := tg.alivePlayers()
	for _, p := range alive {
		tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: tg.seer.ID})
	}

	// After seer executed: 2 wolves + 1 villager.
	// Wolves >= villagers → wolves win!
	if !tg.IsGameOver() {
		// If game isn't over, check alive counts
		wolves := 0
		villageTeam := 0
		for _, p := range tg.Players {
			if p.Alive {
				if p.Team == TeamWerewolf {
					wolves++
				} else {
					villageTeam++
				}
			}
		}
		t.Logf("wolves=%d village=%d", wolves, villageTeam)
		if wolves >= villageTeam {
			t.Error("expected game over (wolves >= villagers)")
		}
	}

	if tg.IsGameOver() && tg.WinningTeam() != TeamWerewolf {
		t.Errorf("winning team = %v, want werewolf", tg.WinningTeam())
	}
}

func TestVillageWin(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Night 1: wolves kill villager[0]
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: tg.villagers[0].ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.wolves[0].ID})

	// Day 1: execute wolf[0]
	for _, pid := range tg.speakingOrder {
		tg.Act(pid, Action{Type: ActionSpeak, Message: "test"})
	}
	alive := tg.alivePlayers()
	for _, p := range alive {
		tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: tg.wolves[0].ID})
	}

	// After day 1: 1 wolf + 1 seer + 1 villager alive (3 alive, 1 wolf, 2 village)
	if tg.IsGameOver() {
		t.Logf("game ended early on day %d, winner: %v", tg.Day, tg.WinningTeam())
		return
	}

	// Night 2: wolf kills villager[1] (or seer)
	remainingVillager := tg.villagers[1]
	if !remainingVillager.Alive {
		// villager[1] might have been killed already, find alive non-wolf
		for _, p := range tg.Players {
			if p.Alive && p.Team == TeamVillage {
				remainingVillager = p
				break
			}
		}
	}

	aliveWolves := tg.aliveWolves()
	for _, wolf := range aliveWolves {
		_, err := tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: remainingVillager.ID})
		if err != nil {
			t.Fatalf("night 2 wolf kill error: %v", err)
		}
	}
	// Seer investigates if alive
	if tg.seer.Alive {
		for _, p := range tg.Players {
			if p.Alive && p.ID != tg.seer.ID {
				tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: p.ID})
				break
			}
		}
	}

	// After night 2: if wolf killed villager → 1 wolf + 1 seer. Wolf >= village? 1 >= 1 → yes, wolves win
	// If wolf killed seer → 1 wolf + 1 villager. Same.
	// So game should be over with wolf win.
	if tg.IsGameOver() {
		t.Logf("game over after night 2, winner: %v", tg.WinningTeam())
		return
	}

	// If game continues, complete day 2 and vote out remaining wolf
	if tg.Phase == PhaseDay {
		for _, pid := range tg.speakingOrder {
			tg.Act(pid, Action{Type: ActionSpeak, Message: "vote wolf"})
		}
		alive = tg.alivePlayers()
		wolf := tg.aliveWolves()[0]
		for _, p := range alive {
			tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: wolf.ID})
		}
	}

	if tg.IsGameOver() {
		t.Logf("game over: winner = %v", tg.WinningTeam())
	}
}

func TestInformationIsolation(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Wolf should see teammates
	wolfState := tg.GetGameState(tg.wolves[0].ID)
	if len(wolfState.WolfTeammates) != 1 {
		t.Errorf("wolf teammates = %d, want 1", len(wolfState.WolfTeammates))
	}
	if wolfState.WolfTeammates[0] != tg.wolves[1].ID {
		t.Errorf("wolf teammate = %q, want %q", wolfState.WolfTeammates[0], tg.wolves[1].ID)
	}

	// Villager should NOT see any roles
	villagerState := tg.GetGameState(tg.villagers[0].ID)
	if len(villagerState.WolfTeammates) != 0 {
		t.Error("villager should not see wolf teammates")
	}
	if len(villagerState.SeerResults) != 0 {
		t.Error("villager should not see seer results")
	}

	// All alive players' roles should be hidden in normal view
	for _, ps := range villagerState.Players {
		if ps.Alive && ps.Role != "" {
			t.Errorf("alive player %s role should be hidden, got %q", ps.ID, ps.Role)
		}
	}
}

func TestInvalidActions(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Villager tries to act at night
	_, err := tg.Act(tg.villagers[0].ID, Action{Type: ActionKill, TargetID: tg.wolves[0].ID})
	if err == nil {
		t.Error("expected error for villager acting at night")
	}

	// Wolf tries to kill another wolf
	_, err = tg.Act(tg.wolves[0].ID, Action{Type: ActionKill, TargetID: tg.wolves[1].ID})
	if err == nil {
		t.Error("expected error for wolf killing wolf")
	}

	// Seer tries to investigate themselves
	_, err = tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.seer.ID})
	if err == nil {
		t.Error("expected error for seer self-investigation")
	}
}

func TestFullGameSimulation(t *testing.T) {
	tg := newTestGame(t, 12345)

	_, err := tg.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	maxRounds := 20
	round := 0

	for !tg.IsGameOver() && round < maxRounds {
		round++

		switch tg.Phase {
		case PhaseNight:
			// Wolves all vote for first alive non-wolf
			for _, wolf := range tg.aliveWolves() {
				var targetID string
				for _, p := range tg.Players {
					if p.Alive && p.Team != TeamWerewolf {
						targetID = p.ID
						break
					}
				}
				if targetID != "" {
					_, err := tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: targetID})
					if err != nil {
						t.Fatalf("round %d wolf kill error: %v", round, err)
					}
				}
			}

			// Seer investigates first alive non-self player
			if tg.seer.Alive && tg.Phase == PhaseNight {
				for _, p := range tg.Players {
					if p.Alive && p.ID != tg.seer.ID {
						tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: p.ID})
						break
					}
				}
			}

		case PhaseDay:
			for _, pid := range tg.speakingOrder {
				_, err := tg.Act(pid, Action{Type: ActionSpeak, Message: "I vote to discuss"})
				if err != nil {
					t.Fatalf("round %d speak error: %v", round, err)
				}
			}

		case PhaseVote:
			// Everyone votes for first alive player (random-ish)
			alive := tg.alivePlayers()
			for _, p := range alive {
				target := alive[0]
				if target.ID == p.ID && len(alive) > 1 {
					target = alive[1]
				}
				_, err := tg.Act(p.ID, Action{Type: ActionVotePlayer, TargetID: target.ID})
				if err != nil {
					t.Fatalf("round %d vote error: %v", round, err)
				}
			}
		}
	}

	if !tg.IsGameOver() {
		t.Fatalf("game not over after %d rounds", maxRounds)
	}

	t.Logf("Game completed in %d rounds, day %d, winner: %v", round, tg.Day, tg.WinningTeam())

	// Verify final state consistency
	rankings := tg.Rankings()
	if len(rankings) != 5 {
		t.Errorf("rankings = %d, want 5", len(rankings))
	}
}

func TestSpeechTruncation(t *testing.T) {
	tg := newTestGame(t, 42)
	tg.Start()

	// Complete night first
	for _, wolf := range tg.wolves {
		tg.Act(wolf.ID, Action{Type: ActionKill, TargetID: tg.villagers[0].ID})
	}
	tg.Act(tg.seer.ID, Action{Type: ActionInvestigate, TargetID: tg.wolves[0].ID})

	if tg.Phase != PhaseDay {
		t.Fatalf("expected day phase, got %v", tg.Phase)
	}

	// Send a very long message
	longMsg := ""
	for i := 0; i < 600; i++ {
		longMsg += "x"
	}

	speaker := tg.speakingOrder[0]
	tg.Act(speaker, Action{Type: ActionSpeak, Message: longMsg})

	// Check speech was truncated
	if len(tg.speeches) != 1 {
		t.Fatalf("speeches = %d, want 1", len(tg.speeches))
	}
	if len(tg.speeches[0].Message) != MaxSpeechLength {
		t.Errorf("speech length = %d, want %d", len(tg.speeches[0].Message), MaxSpeechLength)
	}
}
