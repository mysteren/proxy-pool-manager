ALTER TABLE mtproto_proxies ADD COLUMN jitter_ms  INTEGER;
ALTER TABLE mtproto_proxies ADD COLUMN successes  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mtproto_proxies ADD COLUMN attempts   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mtproto_proxies ADD COLUMN score      REAL;
