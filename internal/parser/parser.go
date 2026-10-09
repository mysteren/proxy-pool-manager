package parser

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"proxy-pool-manager/internal/models"
)

// ParseText разбирает построчный список прокси.
// Поддерживаются "ip:port" и "socks5://ip:port"; пустые строки и комментарии
// (#, //) пропускаются. Приложение работает только с SOCKS5, поэтому строки
// с другими протоколами (http/https/socks4) пропускаются, а строка без схемы
// ("host:port") считается SOCKS5.
func ParseText(content string) ([]models.ParsedProxy, error) {
	out := make([]models.ParsedProxy, 0)
	seen := make(map[string]struct{})

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		// Telegram-прокси (tg://, host:port:secret) идут в отдельный раздел.
		if _, isMTProto := parseMTProtoLine(line); isMTProto {
			continue
		}
		p, ok := parseLine(line)
		if !ok {
			continue
		}
		key := proxyKey(p)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

func parseLine(line string) (models.ParsedProxy, bool) {
	protocol := ""
	hostPort := line
	if i := strings.Index(line, "://"); i >= 0 {
		protocol = strings.ToLower(strings.TrimSpace(line[:i]))
		hostPort = line[i+3:]
	}
	// Отбросить путь/параметры (например, http://host:port/).
	if i := strings.IndexAny(hostPort, "/?#"); i >= 0 {
		hostPort = hostPort[:i]
	}

	host, portStr, ok := splitHostPort(hostPort)
	if !ok {
		return models.ParsedProxy{}, false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || host == "" || port < 1 || port > 65535 {
		return models.ParsedProxy{}, false
	}
	normalized, ok := normalizeProtocol(protocol)
	if !ok {
		return models.ParsedProxy{}, false
	}
	return models.ParsedProxy{Host: host, Port: port, Protocol: normalized}, true
}

func splitHostPort(s string) (string, string, bool) {
	if host, port, err := net.SplitHostPort(s); err == nil {
		return host, port, true
	}
	i := strings.LastIndex(s, ":")
	if i < 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// normalizeProtocol приводит протокол к поддерживаемому виду.
// Приложение работает только с SOCKS5, поэтому остальные протоколы
// (http/https/socks4 и неизвестные) пропускаются, а пустой протокол
// (строка вида "host:port") считается SOCKS5.
// Второе значение — false, если прокси нужно пропустить.
func normalizeProtocol(p string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "", "socks5", "socks5h":
		return "socks5", true
	default:
		return "", false
	}
}

func proxyKey(p models.ParsedProxy) string {
	return p.Protocol + "://" + p.Host + ":" + strconv.Itoa(p.Port)
}

// rawJSONProxy описывает запись списка в формате Proxifly и подобных.
type rawJSONProxy struct {
	IP        string          `json:"ip"`
	Port      json.RawMessage `json:"port"`
	Protocols []string        `json:"protocols"`
	Protocol  string          `json:"protocol"`
	Country   string          `json:"country"`
}

// ParseJSON разбирает JSON-массив или построчный JSON (JSONL).
func ParseJSON(content []byte) ([]models.ParsedProxy, error) {
	var items []rawJSONProxy
	if err := json.Unmarshal(content, &items); err != nil {
		items = items[:0]
		for _, raw := range strings.Split(string(content), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}
			var item rawJSONProxy
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				return nil, err
			}
			items = append(items, item)
		}
	}

	out := make([]models.ParsedProxy, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		port, ok := parsePort(item.Port)
		if !ok {
			continue
		}
		proto := ""
		if len(item.Protocols) > 0 {
			proto = item.Protocols[0]
		}
		if proto == "" {
			proto = item.Protocol
		}
		protocol, ok := normalizeProtocol(proto)
		if !ok {
			continue
		}
		host := strings.ToLower(strings.TrimSpace(item.IP))
		if host == "" || port < 1 || port > 65535 {
			continue
		}
		p := models.ParsedProxy{Host: host, Port: port, Protocol: protocol}
		if c := strings.TrimSpace(item.Country); c != "" {
			p.Country = &c
		}
		key := proxyKey(p)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

func parsePort(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, n > 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n, n > 0
		}
	}
	return 0, false
}
