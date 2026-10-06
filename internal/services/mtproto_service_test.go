package services

import (
	"context"
	"net"
	"testing"
	"time"

	"proxy-pool-manager/internal/models"
)

func TestProbeSocksQuality(t *testing.T) {
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

	svc := NewMTProtoService(nil, nil)
	p := models.MTProtoProxy{Host: "127.0.0.1", Port: addr.Port, Type: "socks"}

	res := svc.probe(context.Background(), p, time.Second)
	if res.Attempts != mtprotoAttempts || res.Successes != mtprotoAttempts || !res.IsWorking {
		t.Fatalf("ожидались 3/3 успешных попыток: %+v", res)
	}
	if res.PingMs == nil || res.JitterMs == nil || res.Score == nil {
		t.Fatalf("метрики качества не заполнены: %+v", res)
	}
	if *res.Score <= 0 {
		t.Fatalf("оценка должна быть положительной: %v", *res.Score)
	}
}

func TestProbeDeadQuality(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close() // порт закрыт

	svc := NewMTProtoService(nil, nil)
	p := models.MTProtoProxy{Host: "127.0.0.1", Port: addr.Port, Type: "socks"}

	res := svc.probe(context.Background(), p, 500*time.Millisecond)
	if res.IsWorking || res.Successes != 0 {
		t.Fatalf("нерабочий прокси не должен быть рабочим: %+v", res)
	}
	if res.Score != nil {
		t.Fatalf("оценка нерабочего прокси должна быть пустой: %v", *res.Score)
	}
}
