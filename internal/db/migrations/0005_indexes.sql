-- Индексы под горячие выборки и счётчики на больших пулах (десятки–сотни тысяч).
-- Приложение всегда фильтрует по protocol, поэтому индексы составные с protocol
-- в начале — тогда и COUNT(*), и сортировка идут по индексу без temp b-tree.
-- Сортировка метрик использует выражение "latency_ms IS NULL" (пустые — в конец),
-- поэтому для неё нужны expression-индексы, повторяющие это выражение целиком.

-- Счётчики: protocol + статус / проверка (покрывающие индексы).
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_working ON proxies(protocol, is_working);
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_checked ON proxies(protocol, last_checked);

-- Сортировка (совпадает с ORDER BY в proxyOrder, по умолчанию — latency).
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_latency  ON proxies(protocol, (latency_ms IS NULL), latency_ms, id);
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_download ON proxies(protocol, (download_mbps IS NULL), download_mbps, id);
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_host     ON proxies(protocol, host, id);
CREATE INDEX IF NOT EXISTS idx_proxies_protocol_port     ON proxies(protocol, port, id);

-- MTProto: фильтры и сортировка по score (по умолчанию).
CREATE INDEX IF NOT EXISTS idx_mtproto_checked ON mtproto_proxies(last_checked);
CREATE INDEX IF NOT EXISTS idx_mtproto_score   ON mtproto_proxies(score);
