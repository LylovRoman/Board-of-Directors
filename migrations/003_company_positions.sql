-- up

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS company_position TEXT;

-- down
/*
ALTER TABLE users
    DROP COLUMN IF EXISTS company_position;
*/
