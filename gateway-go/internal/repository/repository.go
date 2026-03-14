package repository

import (
	"context"

	"github.com/google/uuid"
)

// Repository abstracts persistence so the backing store can be swapped
// (e.g. PostgreSQL → Google Spanner) without touching business logic.
type Repository interface {
	EnsureUser(ctx context.Context, userID uuid.UUID) error
	CreateConversation(ctx context.Context, userID, convID uuid.UUID) error
	SaveMessage(ctx context.Context, msg Message) error
	GetMessages(ctx context.Context, userID, convID uuid.UUID) ([]Message, error)
}
