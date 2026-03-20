package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// In-memory mock UserRepository for testing
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	mu    sync.Mutex
	users map[int64]*domain.UserProfile
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[int64]*domain.UserProfile)}
}

func (r *mockUserRepo) GetUser(_ context.Context, userID int64) (*domain.UserProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.users[userID]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (r *mockUserRepo) UpsertUser(_ context.Context, profile *domain.UserProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *profile
	r.users[profile.UserID] = &cp
	return nil
}

func (r *mockUserRepo) SetRole(ctx context.Context, userID int64, role domain.UserRole) error {
	r.mu.Lock()
	p, ok := r.users[userID]
	if !ok {
		p = &domain.UserProfile{UserID: userID}
		r.users[userID] = p
	}
	p.Role = role
	r.mu.Unlock()
	return nil
}

func (r *mockUserRepo) GetAllUsers(_ context.Context) ([]*domain.UserProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.UserProfile
	for _, p := range r.users {
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAuthorize_NewUser_CreatedAsFree(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	result := mw.Authorize(ctx, 123, "alice", "/help")

	if !result.Allowed {
		t.Fatal("new user should be allowed")
	}
	if result.Profile == nil {
		t.Fatal("profile should not be nil")
	}
	if result.Profile.Role != domain.RoleFree {
		t.Errorf("new user role = %s, want free", result.Profile.Role)
	}
}

func TestAuthorize_OwnerAutoDetected(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 42)
	defer mw.Stop()

	ctx := context.Background()
	result := mw.Authorize(ctx, 42, "owner", "/help")

	if !result.Allowed {
		t.Fatal("owner should be allowed")
	}
	if result.Profile.Role != domain.RoleOwner {
		t.Errorf("owner role = %s, want owner", result.Profile.Role)
	}
}

func TestAuthorize_BannedUserDenied(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[50] = &domain.UserProfile{
		UserID:           50,
		Role:             domain.RoleBanned,
		CounterResetDate: time.Now().Format("2006-01-02"),
	}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	result := mw.Authorize(ctx, 50, "banned_user", "/help")

	if result.Allowed {
		t.Fatal("banned user should be denied")
	}
	if result.Reason == "" {
		t.Error("denial should have a reason")
	}
}

func TestAuthorize_FreeDailyLimit(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()

	// Free tier = 10 commands/day.
	// First call creates user (no counter increment), then 10 more allowed = 11 total.
	for i := 0; i < 11; i++ {
		result := mw.Authorize(ctx, 200, "user", "/help")
		if !result.Allowed {
			t.Fatalf("command %d should be allowed", i+1)
		}
	}

	// 12th command should be denied (counter at 10 = limit)
	result := mw.Authorize(ctx, 200, "user", "/help")
	if result.Allowed {
		t.Fatal("12th command should be denied (daily limit)")
	}
}

func TestAuthorize_OwnerUnlimited(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 300)
	defer mw.Stop()

	ctx := context.Background()

	// Owner should never be rate limited
	for i := 0; i < 100; i++ {
		result := mw.Authorize(ctx, 300, "owner", "/help")
		if !result.Allowed {
			t.Fatalf("owner command %d should not be rate limited", i+1)
		}
	}
}

func TestCheckAIQuota_FreeLimit(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()

	// First register the user
	mw.Authorize(ctx, 400, "user", "/help")

	// Free = 3 AI calls/day
	for i := 0; i < 3; i++ {
		allowed, reason := mw.CheckAIQuota(ctx, 400)
		if !allowed {
			t.Fatalf("AI call %d should be allowed, reason: %s", i+1, reason)
		}
	}

	// 4th should be denied
	allowed, reason := mw.CheckAIQuota(ctx, 400)
	if allowed {
		t.Fatal("4th AI call should be denied (daily limit)")
	}
	if reason == "" {
		t.Error("denial should have a reason")
	}
}

func TestAuthorizeCallback_BannedDenied(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[60] = &domain.UserProfile{
		UserID: 60,
		Role:   domain.RoleBanned,
	}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	result := mw.AuthorizeCallback(ctx, 60)

	if result.Allowed {
		t.Fatal("banned user callback should be denied")
	}
}

func TestAuthorizeCallback_OwnerAlwaysAllowed(t *testing.T) {
	repo := newMockUserRepo()
	mw := NewMiddleware(repo, 77)
	defer mw.Stop()

	ctx := context.Background()
	result := mw.AuthorizeCallback(ctx, 77)

	if !result.Allowed {
		t.Fatal("owner callback should always be allowed")
	}
}

func TestEffectiveAlertFilters_FreeGetsUSDHighOnly(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[500] = &domain.UserProfile{
		UserID: 500,
		Role:   domain.RoleFree,
	}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	currencies, impacts := mw.EffectiveAlertFilters(ctx, 500, []string{"EUR", "GBP"}, []string{"High", "Medium"})

	if len(currencies) != 1 || currencies[0] != "USD" {
		t.Errorf("Free currencies = %v, want [USD]", currencies)
	}
	if len(impacts) != 1 || impacts[0] != "High" {
		t.Errorf("Free impacts = %v, want [High]", impacts)
	}
}

func TestEffectiveAlertFilters_MemberGetsOwnPrefs(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[501] = &domain.UserProfile{
		UserID: 501,
		Role:   domain.RoleMember,
	}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	currencies, impacts := mw.EffectiveAlertFilters(ctx, 501, []string{"EUR", "GBP"}, []string{"High", "Medium"})

	if len(currencies) != 2 {
		t.Errorf("Member currencies = %v, want [EUR GBP]", currencies)
	}
	if len(impacts) != 2 {
		t.Errorf("Member impacts = %v, want [High Medium]", impacts)
	}
}

func TestEffectiveAlertFilters_BannedGetsBogusFilter(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[502] = &domain.UserProfile{
		UserID: 502,
		Role:   domain.RoleBanned,
	}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	currencies, impacts := mw.EffectiveAlertFilters(ctx, 502, nil, nil)

	if len(currencies) != 1 || currencies[0] != "__BANNED__" {
		t.Errorf("Banned currencies = %v, want [__BANNED__]", currencies)
	}
	if len(impacts) != 1 || impacts[0] != "__BANNED__" {
		t.Errorf("Banned impacts = %v, want [__BANNED__]", impacts)
	}
}

func TestShouldReceiveFREDAlerts(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[600] = &domain.UserProfile{UserID: 600, Role: domain.RoleFree}
	repo.users[601] = &domain.UserProfile{UserID: 601, Role: domain.RoleMember}
	repo.users[602] = &domain.UserProfile{UserID: 602, Role: domain.RoleBanned}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()

	if mw.ShouldReceiveFREDAlerts(ctx, 600) {
		t.Error("Free user should NOT receive FRED alerts")
	}
	if !mw.ShouldReceiveFREDAlerts(ctx, 601) {
		t.Error("Member should receive FRED alerts")
	}
	if mw.ShouldReceiveFREDAlerts(ctx, 602) {
		t.Error("Banned user should NOT receive FRED alerts")
	}
}

func TestSetUserRole_UsesPerUserMutex(t *testing.T) {
	repo := newMockUserRepo()
	repo.users[700] = &domain.UserProfile{UserID: 700, Role: domain.RoleFree}
	mw := NewMiddleware(repo, 999)
	defer mw.Stop()

	ctx := context.Background()
	err := mw.SetUserRole(ctx, 700, domain.RoleMember)
	if err != nil {
		t.Fatalf("SetUserRole failed: %v", err)
	}

	role := mw.GetUserRole(ctx, 700)
	if role != domain.RoleMember {
		t.Errorf("role after SetUserRole = %s, want member", role)
	}
}

func TestFormatUserList_EscapesHTML(t *testing.T) {
	users := []*domain.UserProfile{
		{
			UserID:   1,
			Username: "<script>alert(1)</script>",
			Role:     domain.RoleFree,
		},
	}

	result := FormatUserList(users)

	if contains(result, "<script>") {
		t.Error("FormatUserList should HTML-escape usernames")
	}
	if !contains(result, "&lt;script&gt;") {
		t.Error("FormatUserList should contain escaped HTML entities")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
