package domain

import "testing"

func TestRoleHierarchy(t *testing.T) {
	tests := []struct {
		role UserRole
		want int
	}{
		{RoleOwner, 100},
		{RoleAdmin, 80},
		{RoleMember, 50},
		{RoleFree, 10},
		{RoleBanned, 0},
		{UserRole("unknown"), 10}, // defaults to free
	}

	for _, tt := range tests {
		got := RoleHierarchy(tt.role)
		if got != tt.want {
			t.Errorf("RoleHierarchy(%q) = %d, want %d", tt.role, got, tt.want)
		}
	}

	// Hierarchy ordering: Owner > Admin > Member > Free > Banned
	if RoleHierarchy(RoleOwner) <= RoleHierarchy(RoleAdmin) {
		t.Error("Owner should outrank Admin")
	}
	if RoleHierarchy(RoleAdmin) <= RoleHierarchy(RoleMember) {
		t.Error("Admin should outrank Member")
	}
	if RoleHierarchy(RoleMember) <= RoleHierarchy(RoleFree) {
		t.Error("Member should outrank Free")
	}
	if RoleHierarchy(RoleFree) <= RoleHierarchy(RoleBanned) {
		t.Error("Free should outrank Banned")
	}
}

func TestGetTierLimits(t *testing.T) {
	// Owner: unlimited
	owner := GetTierLimits(RoleOwner)
	if owner.CommandLimit != 0 || owner.AICallsPerDay != 0 || owner.AICooldownSec != 0 {
		t.Errorf("Owner should be unlimited, got %+v", owner)
	}

	// Admin: 30 cmd/min, 50 AI/day
	admin := GetTierLimits(RoleAdmin)
	if admin.CommandLimit != 30 || admin.CommandDaily {
		t.Errorf("Admin commands: want 30/min, got %d daily=%v", admin.CommandLimit, admin.CommandDaily)
	}
	if admin.AICallsPerDay != 50 {
		t.Errorf("Admin AI: want 50/day, got %d", admin.AICallsPerDay)
	}

	// Member: 15 cmd/min, 10 AI/day
	member := GetTierLimits(RoleMember)
	if member.CommandLimit != 15 || member.CommandDaily {
		t.Errorf("Member commands: want 15/min, got %d daily=%v", member.CommandLimit, member.CommandDaily)
	}
	if member.AICallsPerDay != 10 {
		t.Errorf("Member AI: want 10/day, got %d", member.AICallsPerDay)
	}

	// Free: 10 cmd/day, 3 AI/day
	free := GetTierLimits(RoleFree)
	if free.CommandLimit != 10 || !free.CommandDaily {
		t.Errorf("Free commands: want 10/day, got %d daily=%v", free.CommandLimit, free.CommandDaily)
	}
	if free.AICallsPerDay != 3 {
		t.Errorf("Free AI: want 3/day, got %d", free.AICallsPerDay)
	}

	// Unknown defaults to free limits
	unknown := GetTierLimits(UserRole("xyz"))
	if unknown.CommandLimit != free.CommandLimit || unknown.AICallsPerDay != free.AICallsPerDay {
		t.Errorf("Unknown role should default to free limits, got %+v", unknown)
	}
}

func TestFreeAlertCurrencies_ReturnsFreshCopy(t *testing.T) {
	a := FreeAlertCurrencies()
	b := FreeAlertCurrencies()

	if len(a) != 1 || a[0] != "USD" {
		t.Errorf("FreeAlertCurrencies() = %v, want [USD]", a)
	}

	// Mutate a, verify b is not affected
	a[0] = "EUR"
	if b[0] != "USD" {
		t.Error("FreeAlertCurrencies returned same underlying slice — mutation leaks")
	}
}

func TestFreeAlertImpacts_ReturnsFreshCopy(t *testing.T) {
	a := FreeAlertImpacts()
	b := FreeAlertImpacts()

	if len(a) != 1 || a[0] != "High" {
		t.Errorf("FreeAlertImpacts() = %v, want [High]", a)
	}

	a[0] = "Low"
	if b[0] != "High" {
		t.Error("FreeAlertImpacts returned same underlying slice — mutation leaks")
	}
}
