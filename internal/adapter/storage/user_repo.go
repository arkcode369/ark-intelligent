package storage

import (
	"context"
	"encoding/json"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// UserRepo implements ports.UserRepository using BadgerDB.
type UserRepo struct {
	db *badger.DB
}

// NewUserRepo creates a new UserRepo backed by the given DB.
func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db.Badger()}
}

func userKey(userID int64) []byte {
	return []byte(fmt.Sprintf("user:%d", userID))
}

// GetUser retrieves a user profile. Returns nil, nil if not found.
func (r *UserRepo) GetUser(ctx context.Context, userID int64) (*domain.UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var profile domain.UserProfile

	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(userKey(userID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &profile)
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", userID, err)
	}
	return &profile, nil
}

// UpsertUser creates or updates a user profile.
func (r *UserRepo) UpsertUser(ctx context.Context, profile *domain.UserProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(userKey(profile.UserID), data)
	})
}

// SetRole updates only the role of an existing user, or creates a minimal profile.
func (r *UserRepo) SetRole(ctx context.Context, userID int64, role domain.UserRole) error {
	profile, err := r.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if profile == nil {
		profile = &domain.UserProfile{
			UserID: userID,
		}
	}
	profile.Role = role
	return r.UpsertUser(ctx, profile)
}

// GetAllUsers retrieves all user profiles from the database.
func (r *UserRepo) GetAllUsers(ctx context.Context) ([]*domain.UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var users []*domain.UserProfile
	prefix := []byte("user:")

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var p domain.UserProfile
				if err := json.Unmarshal(val, &p); err != nil {
					return err
				}
				users = append(users, &p)
				return nil
			})
			if err != nil {
				return fmt.Errorf("read user: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get all users: %w", err)
	}
	return users, nil
}
