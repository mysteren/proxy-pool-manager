CREATE TABLE sources (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    url           TEXT,
    file_path     TEXT,
    last_fetched  DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (url IS NOT NULL OR file_path IS NOT NULL)
);

CREATE TABLE proxies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    host            TEXT    NOT NULL,
    port            INTEGER NOT NULL,
    protocol        TEXT    NOT NULL DEFAULT 'http',
    country         TEXT,
    latency_ms      INTEGER,
    download_mbps   REAL,
    last_checked    DATETIME,
    is_working      BOOLEAN NOT NULL DEFAULT 0,
    source_id       INTEGER REFERENCES sources(id) ON DELETE SET NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(host, port, protocol)
);

CREATE INDEX idx_proxies_is_working ON proxies(is_working);
CREATE INDEX idx_proxies_latency    ON proxies(latency_ms);
CREATE INDEX idx_proxies_protocol   ON proxies(protocol);
CREATE INDEX idx_proxies_source     ON proxies(source_id);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
