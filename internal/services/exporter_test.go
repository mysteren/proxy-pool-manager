package services

import (
	"bytes"
	"strings"
	"testing"

	"proxy-pool-manager/internal/models"
)

func sampleProxies() []models.Proxy {
	lat := 120
	mbps := 5.4
	country := "US"
	return []models.Proxy{
		{ID: 1, Host: "1.1.1.1", Port: 8080, Protocol: "http", LatencyMs: &lat, DownloadMbps: &mbps, Country: &country, IsWorking: true},
		{ID: 2, Host: "2.2.2.2", Port: 1080, Protocol: "socks5"},
	}
}

func writeAll(t *testing.T, format string, proxies []models.Proxy) string {
	t.Helper()
	var buf bytes.Buffer
	sink := newProxySink(format, &buf)
	for _, p := range proxies {
		if err := sink.Write(p); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.String()
}

func TestExportTXT(t *testing.T) {
	out := writeAll(t, "txt", sampleProxies())
	want := "http://1.1.1.1:8080\nsocks5://2.2.2.2:1080\n"
	if out != want {
		t.Fatalf("TXT = %q, ожидалось %q", out, want)
	}
}

func TestExportCSV(t *testing.T) {
	out := writeAll(t, "csv", sampleProxies())
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("строк CSV = %d, ожидалось 3\n%s", len(lines), out)
	}
	if lines[0] != "host,port,protocol,latency_ms,download_mbps,country" {
		t.Fatalf("заголовок CSV неверен: %q", lines[0])
	}
	if lines[1] != "1.1.1.1,8080,http,120,5.4,US" {
		t.Fatalf("строка CSV неверна: %q", lines[1])
	}
	if lines[2] != "2.2.2.2,1080,socks5,,," {
		t.Fatalf("строка CSV неверна: %q", lines[2])
	}
}

func TestExportJSON(t *testing.T) {
	out := writeAll(t, "json", sampleProxies())
	if !strings.HasPrefix(out, "[") || !strings.Contains(out, `"host":"1.1.1.1"`) {
		t.Fatalf("JSON неверен: %s", out)
	}

	var empty bytes.Buffer
	sink := newProxySink("json", &empty)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(empty.String()) != "[]" {
		t.Fatalf("пустой JSON = %q, ожидалось []", empty.String())
	}
}
