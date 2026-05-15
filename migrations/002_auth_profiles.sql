-- up

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS login VARCHAR(255),
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS avatar_url TEXT,
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login_lower
    ON users (LOWER(login))
    WHERE login IS NOT NULL;

-- down
/*
DROP INDEX IF EXISTS idx_users_login_lower;
ALTER TABLE users
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS login;
*/
