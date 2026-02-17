package poker

import "sort"

// Pot represents a main pot or side pot.
type Pot struct {
	Amount   int   `json:"amount"`
	Eligible []int `json:"eligible"` // seat indices eligible to win this pot
}

// CalculatePots calculates main pot and side pots from player bets.
// Players who folded contribute to pots but are not eligible to win.
func CalculatePots(players []*Player) []Pot {
	// Collect unique bet levels
	levelSet := make(map[int]struct{})
	for _, p := range players {
		if p.TotalBet > 0 {
			levelSet[p.TotalBet] = struct{}{}
		}
	}

	if len(levelSet) == 0 {
		return nil
	}

	levels := make([]int, 0, len(levelSet))
	for l := range levelSet {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	var pots []Pot
	prevLevel := 0

	for _, level := range levels {
		increment := level - prevLevel
		if increment <= 0 {
			continue
		}

		potAmount := 0
		var eligible []int

		for _, p := range players {
			// Calculate this player's contribution to this pot segment
			if p.TotalBet > prevLevel {
				contrib := increment
				remaining := p.TotalBet - prevLevel
				if remaining < increment {
					contrib = remaining
				}
				potAmount += contrib
			}

			// Player is eligible if not folded/eliminated and bet at least this level
			if !p.Folded && !p.Eliminated && p.TotalBet >= level {
				eligible = append(eligible, p.Seat)
			}
		}

		if potAmount > 0 {
			if len(eligible) == 0 {
				// Dead money from folded players - add to previous pot
				if len(pots) > 0 {
					pots[len(pots)-1].Amount += potAmount
				}
			} else if len(pots) > 0 && sameEligible(pots[len(pots)-1].Eligible, eligible) {
				// Same eligible set as previous pot - merge
				pots[len(pots)-1].Amount += potAmount
			} else {
				pots = append(pots, Pot{Amount: potAmount, Eligible: eligible})
			}
		}

		prevLevel = level
	}

	return pots
}

func sameEligible(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
