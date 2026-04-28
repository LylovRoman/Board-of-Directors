-- up

CREATE TABLE IF NOT EXISTS users (
                       id BIGSERIAL PRIMARY KEY,
                       name VARCHAR(255) NOT NULL,
                       created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS games (
                       id BIGSERIAL PRIMARY KEY,
                       title VARCHAR(255) NOT NULL,
                       created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
                        id BIGSERIAL PRIMARY KEY,
                        game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
                        user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
                        actor_name VARCHAR(255),
                        event_type VARCHAR(255) NOT NULL,
                        event_value TEXT,
                        created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_game_id ON events(game_id);
CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);

-- down
/*
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS users;
*/