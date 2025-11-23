CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    team_name TEXT NOT NULL REFERENCES teams(name) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_users_team_name ON users(team_name);
