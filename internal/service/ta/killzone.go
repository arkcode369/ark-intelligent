package ta

import "time"

// KillzoneResult describes the current ICT trading session/killzone.
type KillzoneResult struct {
	ActiveKillzone      string // "LONDON_OPEN", "NY_OPEN", "LONDON_CLOSE", "NY_CLOSE", "ASIA", "OFF_HOURS"
	IsActive            bool   // true if currently in a killzone
	SessionDescription  string // human-readable label
	MinutesUntilNext    int    // minutes until next killzone starts
	NextKillzone        string // name of next killzone
	IntersessionOverlap bool   // true if in London-NY overlap (13:00-16:00 UTC)
}

// killzone defines a trading session window in UTC hours.
type killzone struct {
	name  string
	label string
	start int // UTC hour start (inclusive)
	end   int // UTC hour end (exclusive)
}

var killzones = []killzone{
	{"ASIA", "🌏 Asia Session", 0, 4},
	{"LONDON_OPEN", "🇬🇧 London Open Killzone", 7, 10},
	{"NY_OPEN", "🇺🇸 New York Open Killzone", 12, 15},
	{"LONDON_CLOSE", "🇬🇧 London Close", 15, 16},
	{"NY_CLOSE", "🇺🇸 New York Close Killzone", 20, 22},
}

// ClassifyKillzone determines which ICT killzone/session is active at time t.
func ClassifyKillzone(t time.Time) KillzoneResult {
	utc := t.UTC()
	hour := utc.Hour()

	result := KillzoneResult{
		ActiveKillzone:     "OFF_HOURS",
		SessionDescription: "⏸ Off-Hours (Low Activity)",
	}

	for _, kz := range killzones {
		if hour >= kz.start && hour < kz.end {
			result.ActiveKillzone = kz.name
			result.SessionDescription = kz.label
			result.IsActive = true
			break
		}
	}

	// London-NY overlap: 13:00-16:00 UTC (highest volatility window)
	if hour >= 13 && hour < 16 {
		result.IntersessionOverlap = true
		if result.IsActive {
			result.SessionDescription += " (London-NY Overlap 🔥)"
		}
	}

	// Calculate next killzone
	result.NextKillzone, result.MinutesUntilNext = nextKillzoneInfo(utc)

	return result
}

// nextKillzoneInfo returns the name of the next killzone and minutes until it starts.
func nextKillzoneInfo(utc time.Time) (string, int) {
	currentMinutes := utc.Hour()*60 + utc.Minute()

	type window struct {
		name  string
		start int // start in minutes from midnight UTC
	}

	windows := []window{
		{"ASIA", 0},
		{"LONDON_OPEN", 7 * 60},
		{"NY_OPEN", 12 * 60},
		{"LONDON_CLOSE", 15 * 60},
		{"NY_CLOSE", 20 * 60},
	}

	// Find the next window that hasn't started yet
	for _, w := range windows {
		if currentMinutes < w.start {
			return w.name, w.start - currentMinutes
		}
	}

	// All today's windows passed — next is ASIA tomorrow
	minutesUntilMidnight := 24*60 - currentMinutes
	return "ASIA", minutesUntilMidnight
}
