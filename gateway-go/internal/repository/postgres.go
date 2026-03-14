package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgRepository is the PostgreSQL implementation of Repository.
type PgRepository struct {
	pool *pgxpool.Pool
}

// New creates a connection pool and verifies connectivity.
func New(ctx context.Context, databaseURL string) (*PgRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PgRepository{pool: pool}, nil
}

// Close releases all pool connections.
func (r *PgRepository) Close() {
	r.pool.Close()
}

func (r *PgRepository) EnsureUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	return err
}

func (r *PgRepository) CreateConversation(ctx context.Context, userID, convID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO conversations (user_id, conv_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, convID,
	)
	return err
}

func (r *PgRepository) SaveMessage(ctx context.Context, msg Message) error {
	if msg.MsgID == uuid.Nil {
		msg.MsgID = uuid.New()
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages (user_id, conv_id, msg_id, role, content)
		 VALUES ($1, $2, $3, $4, $5)`,
		msg.UserID, msg.ConvID, msg.MsgID, msg.Role, msg.Content,
	)
	return err
}

func (r *PgRepository) GetMessages(ctx context.Context, userID, convID uuid.UUID) ([]Message, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT user_id, conv_id, msg_id, role, content, created_at
		 FROM messages
		 WHERE user_id = $1 AND conv_id = $2
		 ORDER BY created_at`,
		userID, convID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.UserID, &m.ConvID, &m.MsgID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
