-- Гео для Telegram-прокси определяется по IP сервера (через MTProto HTTP-запрос
-- провести нельзя, в отличие от SOCKS5), поэтому храним резолвнутый IP и гео.
ALTER TABLE mtproto_proxies ADD COLUMN country   TEXT;
ALTER TABLE mtproto_proxies ADD COLUMN city      TEXT;
ALTER TABLE mtproto_proxies ADD COLUMN latitude  REAL;
ALTER TABLE mtproto_proxies ADD COLUMN longitude REAL;
ALTER TABLE mtproto_proxies ADD COLUMN server_ip TEXT;
