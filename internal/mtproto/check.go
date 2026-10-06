// Package mtproto проверяет Telegram-прокси (MTProto) реальным рукопожатием:
// fake-TLS (HMAC в ClientHello), obfuscated2 (AES-CTR + req_pq к DC) или TCP.
// Порт логики из docs/proxy-parser/check-proxies.js на Go (только stdlib).
package mtproto

import (
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

// Mode — тип секрета MTProto.
type Mode int

const (
	ModeClassic Mode = iota // секрет без префикса
	ModeTLS                 // 0xee… — fake-TLS
	ModeSecure              // 0xdd… — secure
)

// Secret — разобранный секрет прокси.
type Secret struct {
	Mode   Mode
	Key    []byte
	Domain string
}

// Result — итог проверки.
type Result struct {
	Working bool
	PingMs  int
	Method  string
}

// ParseSecret разбирает hex- или base64url-секрет в ключ 16 байт и режим.
func ParseSecret(s string) (*Secret, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("пустой секрет")
	}
	var raw []byte
	if isHex(s) && len(s)%2 == 0 {
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, err
		}
		raw = b
	} else if isBase64ish(s) {
		std := strings.NewReplacer("-", "+", "_", "/").Replace(s)
		std = strings.TrimRight(std, "=")
		if m := len(std) % 4; m != 0 {
			std += strings.Repeat("=", 4-m)
		}
		b, err := base64.StdEncoding.DecodeString(std)
		if err != nil {
			return nil, err
		}
		raw = b
	} else {
		return nil, errors.New("некорректный секрет")
	}

	if len(raw) < 16 {
		return nil, errors.New("секрет короче 16 байт")
	}
	if raw[0] == 0xee && len(raw) >= 17 {
		return &Secret{Mode: ModeTLS, Key: raw[1:17], Domain: string(raw[17:])}, nil
	}
	if raw[0] == 0xdd && len(raw) >= 17 {
		return &Secret{Mode: ModeSecure, Key: raw[1:17]}, nil
	}
	return &Secret{Mode: ModeClassic, Key: raw[:16]}, nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

func isBase64ish(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '=':
		default:
			return false
		}
	}
	return len(s) > 0
}

// Check проверяет один прокси. Для type=socks или пустого секрета — только TCP.
func Check(ctx context.Context, host string, port int, secret, proxyType string, timeout time.Duration) Result {
	raw := strings.TrimSpace(secret)
	if proxyType == "socks" || raw == "" {
		return tcpResult(ctx, host, port, timeout)
	}
	sec, err := ParseSecret(raw)
	if err != nil {
		return tcpResult(ctx, host, port, timeout)
	}
	switch sec.Mode {
	case ModeTLS:
		if ping, ok := fakeTLSCheck(ctx, host, port, sec.Key, sec.Domain, timeout); ok {
			return Result{Working: true, PingMs: ping, Method: "fake-tls"}
		}
	case ModeSecure, ModeClassic:
		if ping, method, ok := obfuscated2Check(ctx, host, port, sec.Key, sec.Mode, timeout); ok {
			return Result{Working: true, PingMs: ping, Method: method}
		}
	}
	return Result{}
}

func tcpResult(ctx context.Context, host string, port int, timeout time.Duration) Result {
	if ping, ok := tcpCheck(ctx, host, port, timeout); ok {
		return Result{Working: true, PingMs: ping, Method: "tcp"}
	}
	return Result{}
}

func tcpCheck(ctx context.Context, host string, port int, timeout time.Duration) (int, bool) {
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, false
	}
	_ = conn.Close()
	return int(time.Since(start).Milliseconds()), true
}

// exchange открывает соединение, шлёт payload и ждёт, пока predicate не
// подтвердит успех по (расшифрованным) данным. Возвращает пинг в мс.
func exchange(ctx context.Context, host string, port int, payload []byte, timeout time.Duration, predicate func([]byte) bool, decrypter cipher.Stream) (int, bool) {
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	_ = conn.SetDeadline(start.Add(timeout))

	if _, err := conn.Write(payload); err != nil {
		return 0, false
	}

	buf := make([]byte, 4096)
	var out []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			if decrypter != nil {
				dec := make([]byte, n)
				decrypter.XORKeyStream(dec, buf[:n])
				out = append(out, dec...)
			} else {
				out = append(out, buf[:n]...)
			}
			if predicate(out) {
				return int(time.Since(start).Milliseconds()), true
			}
		}
		if err != nil {
			return 0, false
		}
	}
}

func hmac256(key []byte, parts ...[]byte) []byte {
	h := hmac.New(sha256.New, key)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func sha256sum(parts ...[]byte) []byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func uint16be(n int) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(n))
	return b
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func reverse(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[len(b)-1-i]
	}
	return out
}
