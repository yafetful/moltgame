package poker

// BlindLevel defines small and big blind amounts.
type BlindLevel struct {
	Small int `json:"small"`
	Big   int `json:"big"`
}

// BlindSchedule manages blind level escalation.
type BlindSchedule struct {
	Levels        []BlindLevel
	HandsPerLevel int
}

// DefaultSchedule is the standard blind schedule for tournaments.
// Blinds escalate every 10 hands.
var DefaultSchedule = &BlindSchedule{
	HandsPerLevel: 10,
	Levels: []BlindLevel{
		{10, 20},   // Level 1: hands 1-10
		{20, 40},   // Level 2: hands 11-20
		{30, 60},   // Level 3: hands 21-30
		{50, 100},  // Level 4: hands 31-40
		{100, 200}, // Level 5: hands 41-50
		{150, 300}, // Level 6: hands 51-60
		{200, 400}, // Level 7: hands 61-70
		{300, 600}, // Level 8+: hands 71+
	},
}

// GetBlinds returns the small and big blind for the given hand number (1-indexed).
func (s *BlindSchedule) GetBlinds(handNum int) (small, big int) {
	if handNum < 1 {
		handNum = 1
	}
	levelIdx := (handNum - 1) / s.HandsPerLevel
	if levelIdx >= len(s.Levels) {
		levelIdx = len(s.Levels) - 1
	}
	return s.Levels[levelIdx].Small, s.Levels[levelIdx].Big
}
