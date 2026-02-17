package trueskill

import (
	"math"
	"testing"
)

func TestNewRating(t *testing.T) {
	r := NewRating()
	if r.Mu != DefaultMu {
		t.Errorf("mu = %f, want %f", r.Mu, DefaultMu)
	}
	if math.Abs(r.Sigma-DefaultSigma) > 0.001 {
		t.Errorf("sigma = %f, want ~%f", r.Sigma, DefaultSigma)
	}
}

func TestConservativeRating(t *testing.T) {
	r := NewRating()
	cr := r.ConservativeRating()
	expected := r.Mu - 3*r.Sigma
	if math.Abs(cr-expected) > 0.001 {
		t.Errorf("conservative = %f, want %f", cr, expected)
	}
}

func Test2PlayerWin(t *testing.T) {
	w := NewRating()
	l := NewRating()

	wNew, lNew := Update2Player(w, l, false)

	// Winner should gain rating
	if wNew.Mu <= w.Mu {
		t.Errorf("winner mu should increase: %f -> %f", w.Mu, wNew.Mu)
	}
	// Loser should lose rating
	if lNew.Mu >= l.Mu {
		t.Errorf("loser mu should decrease: %f -> %f", l.Mu, lNew.Mu)
	}
	// Both sigmas should decrease (more certain)
	if wNew.Sigma >= w.Sigma+DefaultTau {
		t.Errorf("winner sigma should decrease: %f -> %f", w.Sigma, wNew.Sigma)
	}
	if lNew.Sigma >= l.Sigma+DefaultTau {
		t.Errorf("loser sigma should decrease: %f -> %f", l.Sigma, lNew.Sigma)
	}

	t.Logf("Winner: mu %f -> %f, sigma %f -> %f", w.Mu, wNew.Mu, w.Sigma, wNew.Sigma)
	t.Logf("Loser:  mu %f -> %f, sigma %f -> %f", l.Mu, lNew.Mu, l.Sigma, lNew.Sigma)
}

func Test2PlayerDraw(t *testing.T) {
	w := Rating{Mu: 30, Sigma: 5}
	l := Rating{Mu: 20, Sigma: 5}

	wNew, lNew := Update2Player(w, l, true)

	// Higher rated player should lose mu in a draw
	if wNew.Mu >= w.Mu {
		t.Errorf("higher rated player mu should decrease in draw: %f -> %f", w.Mu, wNew.Mu)
	}
	// Lower rated player should gain mu in a draw
	if lNew.Mu <= l.Mu {
		t.Errorf("lower rated player mu should increase in draw: %f -> %f", l.Mu, lNew.Mu)
	}

	t.Logf("High: mu %f -> %f", w.Mu, wNew.Mu)
	t.Logf("Low:  mu %f -> %f", l.Mu, lNew.Mu)
}

func TestUpsetWin(t *testing.T) {
	weak := Rating{Mu: 15, Sigma: 4}
	strong := Rating{Mu: 35, Sigma: 4}

	weakNew, strongNew := Update2Player(weak, strong, false)

	// Upset: weak player wins against strong
	// Weak should gain a LOT
	muGain := weakNew.Mu - weak.Mu
	muLoss := strong.Mu - strongNew.Mu

	if muGain < 1 {
		t.Errorf("weak winner should gain significant mu: %f -> %f (gain %f)", weak.Mu, weakNew.Mu, muGain)
	}
	if muLoss < 1 {
		t.Errorf("strong loser should lose significant mu: %f -> %f (loss %f)", strong.Mu, strongNew.Mu, muLoss)
	}

	t.Logf("Upset gain: %f, loss: %f", muGain, muLoss)
}

func TestMultiplayerRatings(t *testing.T) {
	// 6-player poker: rank 1 (winner) through 6 (first eliminated)
	players := []RankedPlayer{
		{ID: "p1", Rating: NewRating(), Rank: 1},
		{ID: "p2", Rating: NewRating(), Rank: 2},
		{ID: "p3", Rating: NewRating(), Rank: 3},
		{ID: "p4", Rating: NewRating(), Rank: 4},
		{ID: "p5", Rating: NewRating(), Rank: 5},
		{ID: "p6", Rating: NewRating(), Rank: 6},
	}

	result := UpdateRatings(players)

	// Ratings should be monotonically decreasing by rank
	for i := 0; i < len(result)-1; i++ {
		if result[i].Rating.Mu < result[i+1].Rating.Mu {
			t.Errorf("rank %d mu (%f) < rank %d mu (%f)", result[i].Rank, result[i].Rating.Mu, result[i+1].Rank, result[i+1].Rating.Mu)
		}
	}

	// First place should have gained mu
	if result[0].Rating.Mu <= DefaultMu {
		t.Errorf("1st place mu should increase: %f", result[0].Rating.Mu)
	}
	// Last place should have lost mu
	if result[len(result)-1].Rating.Mu >= DefaultMu {
		t.Errorf("last place mu should decrease: %f", result[len(result)-1].Rating.Mu)
	}

	for _, p := range result {
		t.Logf("Rank %d (%s): mu=%.2f sigma=%.2f conservative=%.2f",
			p.Rank, p.ID, p.Rating.Mu, p.Rating.Sigma, p.Rating.ConservativeRating())
	}
}

func TestRepeatedGames(t *testing.T) {
	// Simulate a strong player consistently winning
	strong := NewRating()
	weak := NewRating()

	for i := 0; i < 20; i++ {
		strong, weak = Update2Player(strong, weak, false)
	}

	// After 20 wins, strong should be much higher
	if strong.Mu < 35 {
		t.Errorf("strong mu after 20 wins = %f, expected > 35", strong.Mu)
	}
	if weak.Mu > 15 {
		t.Errorf("weak mu after 20 losses = %f, expected < 15", weak.Mu)
	}
	// Sigma should be smaller (more certain)
	if strong.Sigma > 5 {
		t.Errorf("strong sigma = %f, expected < 5 after many games", strong.Sigma)
	}

	t.Logf("After 20 games: strong=%.2f±%.2f, weak=%.2f±%.2f",
		strong.Mu, strong.Sigma, weak.Mu, weak.Sigma)
}
