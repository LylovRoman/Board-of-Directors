-- up

ALTER TABLE user_respects
    ADD COLUMN IF NOT EXISTS week_start TIMESTAMP;

UPDATE user_respects
SET week_start = date_trunc('week', created_at)
WHERE week_start IS NULL;

ALTER TABLE user_respects
    ALTER COLUMN week_start SET DEFAULT date_trunc('week', (NOW() AT TIME ZONE 'UTC')),
    ALTER COLUMN week_start SET NOT NULL;

ALTER TABLE user_respects
    DROP CONSTRAINT IF EXISTS user_respects_pkey;

ALTER TABLE user_respects
    ADD PRIMARY KEY (giver_user_id, receiver_user_id, week_start);

CREATE INDEX IF NOT EXISTS idx_user_respects_receiver_week_start
    ON user_respects (receiver_user_id, week_start);

-- down
/*
DROP INDEX IF EXISTS idx_user_respects_receiver_week_start;
ALTER TABLE user_respects
    DROP CONSTRAINT IF EXISTS user_respects_pkey;
ALTER TABLE user_respects
    ADD PRIMARY KEY (giver_user_id, receiver_user_id);
ALTER TABLE user_respects
    DROP COLUMN IF EXISTS week_start;
*/
