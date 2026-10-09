package services

import (
	"bytes"
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

func serverProxy(t *testing.T, srv *httptest.Server) models.ParsedProxy {
	t.Helper()
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

func TestSpeedZeroNotWorking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // пустое тело
	}))
	defer srv.Close()

	tester, _, storage := newTestTester(t)
	id := insertProxy(t, storage, serverProxy(t, srv))
	p, err := storage.GetProxyByID(id)
	if err != nil || p == nil {
		t.Fatal("прокси не найден")
	}

	mbps, err := tester.measureSpeed(context.Background(), *p, testConfig{speedURL: srv.URL, speedBytes: 1024}, 2*time.Second)
	if err == nil {
		t.Fatal("нулевая скорость должна быть ошибкой")
	}
	if mbps != nil {
		t.Fatalf("скорость не должна быть задана: %v", *mbps)
	}
}

func TestSpeedOk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 4096))
	}))
	defer srv.Close()

	tester, _, storage := newTestTester(t)
	id := insertProxy(t, storage, serverProxy(t, srv))
	p, _ := storage.GetProxyByID(id)

	mbps, err := tester.measureSpeed(context.Background(), *p, testConfig{speedURL: srv.URL, speedBytes: 4096}, 2*time.Second)
	if err != nil || mbps == nil {
		t.Fatalf("ожидалась ненулевая скорость: mbps=%v err=%v", mbps, err)
	}
}

func TestValidationEmptyBodyNotWorking(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // пустой ответ
	}))
	defer proxySrv.Close()

	tester, settings, storage := newTestTester(t)
	saveSettings(t, settings, Settings{
		Theme:             "system",
		LatencyTimeoutMs:  1000,
		TestConcurrency:   5,
		CopyFormat:        "uri",
		ValidateViaHTTP:   true,
		HTTPValidationURL: target.URL,
	})
	id := insertProxy(t, storage, serverProxy(t, proxySrv))

	res, err := tester.TestProxy(id)
	if err != nil {
		t.Fatalf("TestProxy: %v", err)
	}
	if res.IsWorking {
		t.Fatal("пустой ответ прокси должен считаться нерабочим")
	}
}

func TestParseGeo(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantIP  string
		wantCC  string
		wantLat bool
	}{
		{
			// Именно этот случай ломался: Cloudflare присылает координаты строкой.
			name:    "cloudflare (координаты строкой)",
			body:    `{"clientIp":"1.2.3.4","country":"US","city":"Dallas","latitude":"32.7767","longitude":"-96.7970"}`,
			wantIP:  "1.2.3.4",
			wantCC:  "US",
			wantLat: true,
		},
		{
			name:    "ipwho.is",
			body:    `{"ip":"5.6.7.8","success":true,"country":"Germany","country_code":"DE","city":"Berlin","latitude":52.52,"longitude":13.405}`,
			wantIP:  "5.6.7.8",
			wantCC:  "DE",
			wantLat: true,
		},
		{
			name:    "ip-api",
			body:    `{"status":"success","country":"United States","countryCode":"US","city":"Ashburn","lat":39.03,"lon":-77.5,"query":"9.9.9.9"}`,
			wantIP:  "9.9.9.9",
			wantCC:  "US",
			wantLat: true,
		},
		{
			name:    "ipinfo (loc строкой)",
			body:    `{"ip":"8.8.8.8","city":"Mountain View","country":"US","loc":"37.4056,-122.0775"}`,
			wantIP:  "8.8.8.8",
			wantCC:  "US",
			wantLat: true,
		},
		{
			name:   "trace",
			body:   "ip=5.6.7.8\nloc=DE\ncolo=FRA\n",
			wantIP: "5.6.7.8",
			wantCC: "DE",
		},
	}
	for _, tc := range cases {
		info := parseGeo([]byte(tc.body))
		if info.exitIP != tc.wantIP || info.country != tc.wantCC {
			t.Errorf("%s: ip=%q country=%q, ожидалось ip=%q country=%q", tc.name, info.exitIP, info.country, tc.wantIP, tc.wantCC)
		}
		if tc.wantLat && (info.latitude == nil || info.longitude == nil) {
			t.Errorf("%s: координаты не разобраны: %+v", tc.name, info)
		}
	}
}

func TestConsensusGeo(t *testing.T) {
	lat, lon := 32.7, -96.8
	merged, ok := consensusGeo([]geoResponse{
		{ok: true, info: validationInfo{exitIP: "1.2.3.4", country: "US", city: "Dallas", latitude: &lat, longitude: &lon}},
		{ok: true, info: validationInfo{exitIP: "1.2.3.4", country: "US", city: "Dallas"}},
		{ok: false},
	})
	if !ok {
		t.Fatal("ожидался успешный консенсус")
	}
	if merged.exitIP != "1.2.3.4" || merged.country != "US" || merged.city != "Dallas" {
		t.Fatalf("консенсус неверен: %+v", merged)
	}
	if merged.latitude == nil || merged.longitude == nil {
		t.Fatalf("координаты не выбраны: %+v", merged)
	}

	if _, ok := consensusGeo([]geoResponse{{ok: false}, {ok: false}}); ok {
		t.Fatal("без ответивших консенсус не должен быть успешным")
	}

	merged2, _ := consensusGeo([]geoResponse{
		{ok: true, info: validationInfo{exitIP: "1.1.1.1", country: "US"}},
		{ok: true, info: validationInfo{exitIP: "1.1.1.1", country: "US"}},
		{ok: true, info: validationInfo{exitIP: "2.2.2.2", country: "DE"}},
	})
	if merged2.exitIP != "1.1.1.1" || merged2.country != "US" {
		t.Fatalf("большинство не взяло верх: %+v", merged2)
	}
}

func TestValidationURLs(t *testing.T) {
	urls := validationURLs(testConfig{validationURL: cloudflareMetaURL, geoConsensus: true})
	if urls[0] != cloudflareMetaURL {
		t.Fatalf("первый URL должен быть настроенным: %v", urls)
	}
	seen := map[string]int{}
	for _, u := range urls {
		seen[u]++
	}
	if seen[cloudflareMetaURL] != 1 {
		t.Fatalf("дубликат Cloudflare: %v", urls)
	}
	if len(urls) != len(builtinGeoURLs) {
		t.Fatalf("ожидалось %d источников, получено %d: %v", len(builtinGeoURLs), len(urls), urls)
	}

	only := validationURLs(testConfig{validationURL: "https://example.com/x", geoConsensus: false})
	if len(only) != 1 || only[0] != "https://example.com/x" {
		t.Fatalf("без консенсуса ожидался один URL: %v", only)
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
