package werewolf

import (
	"regexp"
	"strings"
)

// injectionPatterns detects common prompt injection attempts in agent speech.
var injectionPatterns = []*regexp.Regexp{
	// System/role override attempts
	regexp.MustCompile(`(?i)\bsystem\s*:\s*`),
	regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|rules?)`),
	regexp.MustCompile(`(?i)\byou\s+are\s+now\b`),
	regexp.MustCompile(`(?i)\bact\s+as\s+(a|an|the)\s+`),
	regexp.MustCompile(`(?i)\bnew\s+(role|persona|identity)\b`),

	// Jailbreak / DAN attempts
	regexp.MustCompile(`(?i)\b(DAN|jailbreak|do\s+anything\s+now)\b`),
	regexp.MustCompile(`(?i)\bdeveloper\s+mode\b`),

	// Instruction injection
	regexp.MustCompile(`(?i)\[INST\]`),
	regexp.MustCompile(`(?i)<\|?(system|assistant|user|im_start|im_end)\|?>`),
	regexp.MustCompile(`(?i)<<\s*SYS\s*>>`),

	// API key / secret extraction
	regexp.MustCompile(`(?i)\b(api[_\s]?key|secret|token|password|credential)s?\b.*\b(show|reveal|tell|print|output|return)\b`),
	regexp.MustCompile(`(?i)\b(show|reveal|tell|print|output|return)\b.*\b(api[_\s]?key|secret|token|password|credential)s?\b`),
}

// SanitizeSpeech checks a message for prompt injection patterns.
// Returns the cleaned message and whether it was flagged.
func SanitizeSpeech(msg string) (string, bool) {
	for _, pat := range injectionPatterns {
		if pat.MatchString(msg) {
			// Replace matched content with [filtered]
			cleaned := pat.ReplaceAllString(msg, "[filtered]")
			return cleaned, true
		}
	}
	return msg, false
}

// StripControlChars removes control characters that might be used to hide injection.
func StripControlChars(msg string) string {
	var b strings.Builder
	b.Grow(len(msg))
	for _, r := range msg {
		// Allow normal printable chars, spaces, newlines, tabs
		if r >= 32 || r == '\n' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
