CREATE OR REPLACE FUNCTION set_update_time()
RETURNS TRIGGER AS $$
BEGIN
    NEW.update_time = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id          BIGSERIAL   PRIMARY KEY,
    username    TEXT        NOT NULL UNIQUE,
    is_admin    BOOLEAN     NOT NULL DEFAULT FALSE,
    create_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();
