CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    user_id UUID PRIMARY KEY
);

CREATE TABLE conversations (
    user_id    UUID NOT NULL REFERENCES users(user_id),
    conv_id    UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, conv_id)
);

CREATE TABLE messages (
    user_id    UUID NOT NULL,
    conv_id    UUID NOT NULL,
    msg_id     UUID NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, conv_id, msg_id),
    FOREIGN KEY (user_id, conv_id) REFERENCES conversations(user_id, conv_id)
);
