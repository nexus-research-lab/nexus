package memory

import (
	"regexp"
	"time"
)

var sessionSummaryEntryPattern = regexp.MustCompile(`(?m)^-\s*Entry:\s*(\S+)`)

type memoryCheckpoints struct {
	Scopes map[string]memoryScopeCheckpoint `json:"scopes"`
}

type memoryScopeCheckpoint struct {
	TurnCount     int       `json:"turn_count"`
	LastRoundID   string    `json:"last_round_id,omitempty"`
	LastExtractAt time.Time `json:"last_extract_at,omitempty"`
	RoundIDs      []string  `json:"round_ids,omitempty"`
}
