package model

import (
	"strconv"
	"time"
)

type (
	ID interface {
		Aid | AidX
	}
	Aid  string
	AidX string
)

// ParseAid extracts the timestamp (ms since epoch) from an aid ID.
// aid format: base36 time (from 2000-01-01) 8 chars + 2 chars noise.
func ParseAid(id Aid) (time.Time, bool) {
	const baseOffset = 946684800000 // 2000-01-01 in ms
	if len(id) < 8 {
		return time.Time{}, false
	}
	// Only first 8 chars are time; ignore noise.
	ms, err := strconv.ParseInt(string(id[:8]), 36, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms+baseOffset).UTC(), true
}

// MustParseAid is a helper that panics on parse failure; useful in tests.
func MustParseAid(id Aid) time.Time {
	if t, ok := ParseAid(id); ok {
		return t
	}
	panic("invalid aid")
}
