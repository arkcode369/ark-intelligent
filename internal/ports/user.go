package ports

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// UserRepository defines storage operations for user profiles and access control.
type UserRepository interface {
	// GetUser retrieves a user profile by ID. Returns nil if not found.
	GetUser(ctx context.Context, userID int64) (*domain.UserProfile, error)

	// UpsertUser creates or updates a user profile.
	UpsertUser(ctx context.Context, profile *domain.UserProfile) error

	// SetRole updates the role of a user. Creates a minimal profile if user does not exist.
	SetRole(ctx context.Context, userID int64, role domain.UserRole) error

	// GetAllUsers retrieves all stored user profiles.
	GetAllUsers(ctx context.Context) ([]*domain.UserProfile, error)
}
