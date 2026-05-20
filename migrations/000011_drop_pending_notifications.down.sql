CREATE TABLE IF NOT EXISTS pending_notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient TEXT NOT NULL,
    search_name TEXT NOT NULL DEFAULT '',
    payload TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_notifications_created ON pending_notifications (created_at);
CREATE INDEX IF NOT EXISTS idx_pending_notifications_name_recipient ON pending_notifications (search_name, recipient);
