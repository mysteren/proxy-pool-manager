-- Кэш гео по IP: гео узла меняется редко, поэтому храним его отдельно и
-- переиспользуем (TTL настраивается, кнопка сброса — в настройках).
CREATE TABLE geo_cache (
    ip         TEXT PRIMARY KEY,
    country    TEXT,
    city       TEXT,
    latitude   REAL,
    longitude  REAL,
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
