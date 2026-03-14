package repository

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID uuid.UUID
}

type Conversation struct {
	UserID    uuid.UUID
	ConvID    uuid.UUID
	CreatedAt time.Time
}

type Message struct {
	UserID    uuid.UUID
	ConvID    uuid.UUID
	MsgID     uuid.UUID
	Role      string
	Content   string
	CreatedAt time.Time
}
