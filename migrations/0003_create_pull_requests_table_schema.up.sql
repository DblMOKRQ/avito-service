-- Таблица для хранения возможных статусов PR. 
CREATE TABLE IF NOT EXISTS pr_statuses (
    status_name TEXT PRIMARY KEY,
    description TEXT
);

-- Заполняем начальными значениями
INSERT INTO pr_statuses (status_name) VALUES ('OPEN'), ('MERGED');

CREATE TABLE IF NOT EXISTS pull_requests (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL REFERENCES pr_statuses(status_name) ON DELETE RESTRICT,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    merged_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pull_requests_author_id ON pull_requests(author_id);
CREATE INDEX IF NOT EXISTS idx_pull_requests_status ON pull_requests(status);
