package ports

import "context"

// ---------------------------------------------------------------------------
// ConversationRepository — Per-user chat history persistence
// ---------------------------------------------------------------------------

// ConversationRepository manages per-user conversation history in storage.
// History is bounded by message count and TTL.
type ConversationRepository interface {
	// GetHistory returns the most recent N messages for a user.
	// Returns empty slice (not error) if no history exists.
	GetHistory(ctx context.Context, userID int64, limit int) ([]ChatMessage, error)

	// AppendMessage adds a single message to the user's conversation history.
	// Automatically prunes oldest messages if the per-user cap is exceeded.
	AppendMessage(ctx context.Context, userID int64, msg ChatMessage) error

	// ClearHistory deletes all conversation history for a user.
	ClearHistory(ctx context.Context, userID int64) error
}
