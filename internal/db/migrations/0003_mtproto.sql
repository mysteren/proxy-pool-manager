CREATE TABLE mtproto_proxies (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    host         TEXT    NOT NULL,
    port         INTEGER NOT NULL,
    secret       TEXT    NOT NULL DEFAULT '',
    tg_type      TEXT    NOT NULL DEFAULT 'mtproto',
    ping_ms      INTEGER,
    method       TEXT,
    is_working   BOOLEAN NOT NULL DEFAULT 0,
    last_checked DATETIME,
    source_id    INTEGER REFERENCES sources(id) ON DELETE SET NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(host, port, secret)
);

CREATE INDEX idx_mtproto_is_working ON mtproto_proxies(is_working);
CREATE INDEX idx_mtproto_ping       ON mtproto_proxies(ping_ms);
CREATE INDEX idx_mtproto_source     ON mtproto_proxies(source_id);
