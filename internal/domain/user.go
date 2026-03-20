package domain

import "time"

// UserRole defines access tier levels for the bot.
type UserRole string

const (
	RoleOwner  UserRole = "owner"
	RoleAdmin  UserRole = "admin"
	RoleMember UserRole = "member" // paid tier
	RoleFree   UserRole = "free"   // default tier
	RoleBanned UserRole = "banned"
)

// RoleHierarchy returns the numeric level of a role (higher = more privileged).
func RoleHierarchy(r UserRole) int {
	switch r {
	case RoleOwner:
		return 100
	case RoleAdmin:
		return 80
	case RoleMember:
		return 50
	case RoleFree:
		return 10
	case RoleBanned:
		return 0
	default:
		return 10 // default to free
	}
}

// UserProfile stores per-user identity, role, and daily usage counters.
type UserProfile struct {
	UserID   int64    `json:"user_id"`
	Username string   `json:"username"`
	Role     UserRole `json:"role"`

	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`

	// Daily usage counters (reset at 00:00 WIB each day)
	DailyCommandCount int    `json:"daily_command_count"`
	DailyAICount      int    `json:"daily_ai_count"`
	CounterResetDate  string `json:"counter_reset_date"` // "2006-01-02"
}

// TierLimits defines rate limits and feature gates per role.
type TierLimits struct {
	// Commands: max per window. For Free, window is daily; for Member+, per-minute.
	CommandLimit int
	CommandDaily bool // true = daily limit, false = per-minute sliding window

	// AI calls per day
	AICallsPerDay int

	// AI cooldown between requests
	AICooldownSec int
}

// GetTierLimits returns the rate limits for a given role.
func GetTierLimits(role UserRole) TierLimits {
	switch role {
	case RoleOwner:
		return TierLimits{CommandLimit: 0, CommandDaily: false, AICallsPerDay: 0, AICooldownSec: 0} // 0 = unlimited
	case RoleAdmin:
		return TierLimits{CommandLimit: 30, CommandDaily: false, AICallsPerDay: 50, AICooldownSec: 10}
	case RoleMember:
		return TierLimits{CommandLimit: 15, CommandDaily: false, AICallsPerDay: 10, AICooldownSec: 30}
	case RoleFree:
		return TierLimits{CommandLimit: 10, CommandDaily: true, AICallsPerDay: 3, AICooldownSec: 30}
	case RoleBanned:
		return TierLimits{CommandLimit: 0, CommandDaily: true, AICallsPerDay: 0, AICooldownSec: 0} // banned: all zero
	default:
		return TierLimits{CommandLimit: 10, CommandDaily: true, AICallsPerDay: 3, AICooldownSec: 30}
	}
}

// FreeAlertCurrencies returns the currencies Free-tier users receive alerts for.
// Returns a fresh copy each call to prevent accidental mutation.
func FreeAlertCurrencies() []string {
	return []string{"USD"}
}

// FreeAlertImpacts returns the impact levels Free-tier users receive alerts for.
// Returns a fresh copy each call to prevent accidental mutation.
func FreeAlertImpacts() []string {
	return []string{"High"}
}
