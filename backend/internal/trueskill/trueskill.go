package trueskill

import (
	"math"
	"sort"
)

// Default TrueSkill parameters
const (
	DefaultMu    = 25.0
	DefaultSigma = 25.0 / 3.0 // ~8.333
	DefaultBeta  = 25.0 / 6.0 // ~4.167 (performance variance)
	DefaultTau   = 25.0 / 300.0 // ~0.083 (dynamics factor)
)

// Rating represents a player's TrueSkill rating.
type Rating struct {
	Mu    float64
	Sigma float64
}

// NewRating creates a default rating.
func NewRating() Rating {
	return Rating{Mu: DefaultMu, Sigma: DefaultSigma}
}

// ConservativeRating returns mu - 3*sigma (lower bound estimate).
func (r Rating) ConservativeRating() float64 {
	return r.Mu - 3*r.Sigma
}

// RankedPlayer is a player with a rank (1 = first place).
type RankedPlayer struct {
	ID     string
	Rating Rating
	Rank   int // 1-based rank (1 = winner)
}

// UpdateRatings computes new TrueSkill ratings for a ranked list of players.
// Players should be sorted by rank (1 = first place).
// Uses the simplified TrueSkill update for free-for-all games via pairwise decomposition.
func UpdateRatings(players []RankedPlayer) []RankedPlayer {
	n := len(players)
	if n < 2 {
		return players
	}

	beta2 := DefaultBeta * DefaultBeta
	tau2 := DefaultTau * DefaultTau

	// Sort by rank
	sort.Slice(players, func(i, j int) bool {
		return players[i].Rank < players[j].Rank
	})

	// Apply dynamics factor (increase uncertainty slightly each game)
	for i := range players {
		players[i].Rating.Sigma = math.Sqrt(players[i].Rating.Sigma*players[i].Rating.Sigma + tau2)
	}

	// Pairwise update: each adjacent pair contributes an update
	muDeltas := make([]float64, n)
	sigmaFactors := make([]float64, n)
	for i := range sigmaFactors {
		sigmaFactors[i] = 1.0
	}

	for i := 0; i < n-1; i++ {
		j := i + 1
		ri := players[i].Rating
		rj := players[j].Rating

		si2 := ri.Sigma * ri.Sigma
		sj2 := rj.Sigma * rj.Sigma
		c2 := si2 + sj2 + 2*beta2
		c := math.Sqrt(c2)

		// Mean difference (winner - loser)
		meanDelta := ri.Mu - rj.Mu

		var v, w float64
		if players[i].Rank == players[j].Rank {
			// Draw
			v = vDraw(meanDelta/c, DefaultBeta/c)
			w = wDraw(meanDelta/c, DefaultBeta/c)
		} else {
			// Win (i beat j)
			v = vWin(meanDelta / c)
			w = wWin(meanDelta / c)
		}

		// Update deltas
		muDeltas[i] += (si2 / c) * v
		muDeltas[j] -= (sj2 / c) * v

		sigmaFactors[i] *= (1 - (si2/c2)*w)
		sigmaFactors[j] *= (1 - (sj2/c2)*w)
	}

	// Apply updates
	result := make([]RankedPlayer, n)
	for i := range players {
		newMu := players[i].Rating.Mu + muDeltas[i]
		newSigma := players[i].Rating.Sigma * math.Sqrt(math.Max(sigmaFactors[i], 0.01))
		result[i] = RankedPlayer{
			ID:     players[i].ID,
			Rating: Rating{Mu: newMu, Sigma: newSigma},
			Rank:   players[i].Rank,
		}
	}
	return result
}

// Update2Player computes new ratings for a 2-player match.
// winner beat loser. If draw is true, it was a tie.
func Update2Player(winner, loser Rating, draw bool) (Rating, Rating) {
	players := []RankedPlayer{
		{ID: "w", Rating: winner, Rank: 1},
		{ID: "l", Rating: loser, Rank: 2},
	}
	if draw {
		players[1].Rank = 1
	}
	result := UpdateRatings(players)
	return result[0].Rating, result[1].Rating
}

// --- Gaussian helper functions ---

// Standard normal PDF
func phi(x float64) float64 {
	return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
}

// Standard normal CDF
func bigPhi(x float64) float64 {
	return (1 + math.Erf(x/math.Sqrt2)) / 2
}

// v function for wins (truncated Gaussian)
func vWin(t float64) float64 {
	denom := bigPhi(t)
	if denom < 1e-15 {
		return -t
	}
	return phi(t) / denom
}

// w function for wins
func wWin(t float64) float64 {
	v := vWin(t)
	return v * (v + t)
}

// v function for draws
func vDraw(t, epsilon float64) float64 {
	a := bigPhi(epsilon - t)
	b := bigPhi(-epsilon - t)
	denom := a - b
	if denom < 1e-15 {
		return 0
	}
	return (phi(-epsilon-t) - phi(epsilon-t)) / denom
}

// w function for draws
func wDraw(t, epsilon float64) float64 {
	a := bigPhi(epsilon - t)
	b := bigPhi(-epsilon - t)
	denom := a - b
	if denom < 1e-15 {
		return 1
	}
	v := vDraw(t, epsilon)
	return v*v + ((epsilon-t)*phi(epsilon-t)-(-epsilon-t)*phi(-epsilon-t))/denom
}
