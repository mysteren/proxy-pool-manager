package services

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"proxy-pool-manager/internal/db"
	"proxy-pool-manager/internal/models"
)

func newTestTester(t *testing.T) (*TesterService, *SettingsService, *StorageService) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	storage := NewStorageService(conn)
	settings := NewSettingsService(storage)
	return NewTesterService(storage, settings), settings, storage
}

// mockProxyServer изображает HTTP-прокси: отвечает 200 на любой запрос.
func mockProxyServer(t *testing.T, delay time.Duration) models.ParsedProxy {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	return models.ParsedProxy{Host: host, Port: port, Protocol: "http"}
}

func saveSettings(t *testing.T, settings *SettingsService, s Settings) {
	t.Helper()
	if _, err := settings.Update(s); err != nil {
		t.Fatal(err)
	}
}

func insertProxy(t *testing.T, storage *StorageService, p models.ParsedProxy) int64 {
	t.Helper()
	if _, err := storage.InsertProxies([]models.ParsedProxy{p}, nil); err != nil {
		t.Fatal(err)
	}
	proxies, err := storage.GetProxies(models.ProxyFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range proxies {
		if pr.Host == p.Host && pr.Port == p.Port {
			return pr.ID
		}
	}
	t.Fatal("прокси не найден после вставки")
	return 0
}

func TestTestProxyThroughHTTPProxy(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxyRec := mockProxyServer(t, 0)

	tester, settings, storage := newTestTester(t)
	saveSettings(t, settings, Settings{
		Theme:             "system",
		LatencyTimeoutMs:  2000,
		TestConcurrency:   5,
		CopyFormat:        "uri",
		ValidateViaHTTP:   true,
		HTTPValidationURL: target.URL,
	})
	id := insertProxy(t, storage, proxyRec)

	res, err := tester.TestProxy(id)
	if err != nil {
		t.Fatalf("TestProxy: %v", err)
	}
	if !res.IsWorking {
		t.Fatalf("ожидался рабочий прокси, error=%q", res.Error)
	}
	if res.LatencyMs == nil {
		t.Fatal("latency не заполнена")
	}

	saved, err := storage.GetProxyByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || !saved.IsWorking || saved.LastChecked == nil {
		t.Fatalf("результат не сохранён в БД: %+v", saved)
	}
}

func TestTestProxyFailure(t *testing.T) {
	tester, settings, storage := newTestTester(t)
	saveSettings(t, settings, Settings{
		Theme:             "system",
		LatencyTimeoutMs:  500,
		TestConcurrency:   5,
		CopyFormat:        "uri",
		ValidateViaHTTP:   false,
		HTTPValidationURL: DefaultSettings().HTTPValidationURL,
	})
	// Порт 1 на localhost заведомо закрыт.
	id := insertProxy(t, storage, models.ParsedProxy{Host: "127.0.0.1", Port: 1, Protocol: "http"})

	res, err := tester.TestProxy(id)
	if err != nil {
		t.Fatalf("TestProxy: %v", err)
	}
	if res.IsWorking {
		t.Fatal("ожидался нерабочий прокси")
	}
}

func TestParseMeta(t *testing.T) {
	info := parseMeta([]byte(`{"clientIp":"1.2.3.4","country":"us","city":"Dallas","latitude":32.7,"longitude":-96.8}`))
	if info.exitIP != "1.2.3.4" || info.country != "US" || info.city != "Dallas" || info.latitude == nil {
		t.Fatalf("JSON meta разобран неверно: %+v", info)
	}
	trace := parseMeta([]byte("ip=5.6.7.8\nloc=DE\ncolo=FRA\n"))
	if trace.exitIP != "5.6.7.8" || trace.country != "DE" {
		t.Fatalf("trace разобран неверно: %+v", trace)
	}
}

func TestRunBatchCancelReturns(t *testing.T) {
	proxyRec := mockProxyServer(t, 2*time.Second)

	tester, _, storage := newTestTester(t)
	insertProxy(t, storage, proxyRec)
	proxies, err := storage.GetProxies(models.ProxyFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()

	cfg := testConfig{timeoutMs: 5000, concurrency: 5, validateViaHTTP: true}
	feed := func(feedCtx context.Context, yield func(models.Proxy) bool) {
		_ = storage.ForEachProxy(feedCtx, models.ProxyFilter{}, 500, yield)
	}
	done := make(chan struct{})
	go func() {
		tester.runBatch(ctx, len(proxies), feed, cfg, tester.testOne, tester.storage.UpdateTestResult, 5)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runBatch не завершился после отмены")
	}
}
