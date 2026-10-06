package mtproto

import (
	"context"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

func TestParseSecretClassic(t *testing.T) {
	s, err := ParseSecret("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("ParseSecret: %v", err)
	}
	if s.Mode != ModeClassic || len(s.Key) != 16 {
		t.Fatalf("ожидался classic/16, получено %+v", s)
	}
}

func TestParseSecretTLS(t *testing.T) {
	key := "00112233445566778899aabbccddeeff"
	secret := "ee" + key + hex.EncodeToString([]byte("www.google.com"))
	s, err := ParseSecret(secret)
	if err != nil {
		t.Fatalf("ParseSecret: %v", err)
	}
	if s.Mode != ModeTLS || hex.EncodeToString(s.Key) != key || s.Domain != "www.google.com" {
		t.Fatalf("TLS-секрет разобран неверно: %+v (domain=%q)", s, s.Domain)
	}
}

func TestParseSecretSecure(t *testing.T) {
	key := "00112233445566778899aabbccddeeff"
	s, err := ParseSecret("dd" + key)
	if err != nil {
		t.Fatalf("ParseSecret: %v", err)
	}
	if s.Mode != ModeSecure || hex.EncodeToString(s.Key) != key {
		t.Fatalf("secure-секрет разобран неверно: %+v", s)
	}
}

func TestParseSecretEmpty(t *testing.T) {
	if _, err := ParseSecret(""); err == nil {
		t.Fatal("ожидалась ошибка для пустого секрета")
	}
}

func TestCheckTCPSocks(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	res := Check(context.Background(), "127.0.0.1", addr.Port, "", "socks", 2*time.Second)
	if !res.Working || res.Method != "tcp" {
		t.Fatalf("ожидался рабочий tcp, получено %+v", res)
	}
}

func TestCheckDeadPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close() // порт закрыт

	res := Check(context.Background(), "127.0.0.1", addr.Port, "", "socks", time.Second)
	if res.Working {
		t.Fatal("закрытый порт не должен быть рабочим")
	}
}
