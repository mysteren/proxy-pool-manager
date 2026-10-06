package parser

import (
	"net/url"
	"strconv"
	"strings"

	"proxy-pool-manager/internal/models"
)

// ParseMTProtoText разбирает строки Telegram-прокси:
// tg://proxy?..., t.me/proxy?..., tg://socks?..., host:port:secret.
// Обычные host:port сюда не попадают — их обрабатывает ParseText.
func ParseMTProtoText(content string) []models.ParsedMTProto {
	out := make([]models.ParsedMTProto, 0)
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		p, ok := parseMTProtoLine(line)
		if !ok {
			continue
		}
		key := p.Host + ":" + strconv.Itoa(p.Port) + ":" + p.Secret
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

func parseMTProtoLine(line string) (models.ParsedMTProto, bool) {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "tg://") || strings.HasPrefix(lower, "t.me/") {
		return parseTGLink(line)
	}
	// host:port:secret
	parts := strings.Split(line, ":")
	if len(parts) >= 3 && isNumeric(parts[1]) {
		port, err := strconv.Atoi(parts[1])
		host := strings.ToLower(strings.TrimSpace(parts[0]))
		secret := strings.TrimSpace(parts[2])
		if err != nil || host == "" || port < 1 || port > 65535 || secret == "" {
			return models.ParsedMTProto{}, false
		}
		return models.ParsedMTProto{Host: host, Port: port, Secret: secret, Type: "mtproto"}, true
	}
	return models.ParsedMTProto{}, false
}

func parseTGLink(line string) (models.ParsedMTProto, bool) {
	normalized := line
	if strings.HasPrefix(strings.ToLower(normalized), "t.me/") {
		normalized = "tg://" + normalized[len("t.me/"):]
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return models.ParsedMTProto{}, false
	}
	q := u.Query()
	host := strings.ToLower(strings.TrimSpace(q.Get("server")))
	port, err := strconv.Atoi(strings.TrimSpace(q.Get("port")))
	if err != nil || host == "" || port < 1 || port > 65535 {
		return models.ParsedMTProto{}, false
	}
	tgType := "mtproto"
	if strings.Contains(strings.ToLower(u.Host), "socks") {
		tgType = "socks"
	}
	return models.ParsedMTProto{Host: host, Port: port, Secret: strings.TrimSpace(q.Get("secret")), Type: tgType}, true
}

// MTProtoLink собирает ссылку tg:// для буфера обмена и экспорта.
func MTProtoLink(p models.MTProtoProxy) string {
	if p.Type == "socks" {
		return "tg://socks?server=" + p.Host + "&port=" + strconv.Itoa(p.Port)
	}
	return "tg://proxy?server=" + p.Host + "&port=" + strconv.Itoa(p.Port) + "&secret=" + p.Secret
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
