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
// Blinds escalate every 6 hands (one full dealer orbit for 6 players), doubling each level.
var DefaultSchedule = &BlindSchedule{
	HandsPerLevel: 6,
	Levels: []BlindLevel{
		{40, 80},     // Level 1: hands 1-6
		{80, 160},    // Level 2: hands 7-12
		{160, 320},   // Level 3: hands 13-18
		{320, 640},   // Level 4: hands 19-24
		{640, 1280},  // Level 5: hands 25-30
		{1280, 2560}, // Level 6+: hands 31+
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
