-- up

CREATE TABLE IF NOT EXISTS user_respects (
    giver_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (giver_user_id, receiver_user_id),
    CONSTRAINT user_respects_no_self CHECK (giver_user_id <> receiver_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_respects_receiver_user_id
    ON user_respects (receiver_user_id);

-- down
/*
DROP TABLE IF EXISTS user_respects;
*/
