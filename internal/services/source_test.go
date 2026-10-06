package services

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"proxy-pool-manager/internal/db"
	"proxy-pool-manager/internal/models"
)

func newTestSourceService(t *testing.T) (*SourceService, *StorageService) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	storage := NewStorageService(conn)
	return NewSourceService(storage), storage
}

func TestAddFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("1.1.1.1:8080\nsocks5://2.2.2.2:1080\n"))
	}))
	defer srv.Close()

	svc, storage := newTestSourceService(t)
	result, err := svc.AddFromURL("тест", srv.URL)
	if err != nil {
		t.Fatalf("AddFromURL: %v", err)
	}
	if result.Fetched != 2 || result.Added != 2 {
		t.Fatalf("ожидалось fetched=2, added=2, получено %+v", result)
	}

	count, err := storage.CountProxies(models.ProxyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("в БД %d прокси, ожидалось 2", count)
	}

	sources, err := storage.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ProxyCount != 2 || sources[0].LastFetched == nil {
		t.Fatalf("источник заполнен неверно: %+v", sources)
	}

	again, err := svc.RefreshSource(sources[0].ID)
	if err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if again.Added != 0 {
		t.Fatalf("при повторной загрузке ожидалось 0 добавленных, получено %d", again.Added)
	}
}

func TestAddFromURLJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"ip":"3.3.3.3","port":3128,"protocols":["http"],"country":"US"}]`))
	}))
	defer srv.Close()

	svc, _ := newTestSourceService(t)
	result, err := svc.AddFromURL("json", srv.URL)
	if err != nil {
		t.Fatalf("AddFromURL (json): %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("ожидалось 1 добавленный, получено %d", result.Added)
	}
}

func TestAddFromURLRejectsBadScheme(t *testing.T) {
	svc, _ := newTestSourceService(t)
	if _, err := svc.AddFromURL("x", "ftp://example.com/list"); err == nil {
		t.Fatal("ожидалась ошибка для схемы ftp")
	}
}

func TestAddFromURLSplitsMTProto(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("1.1.1.1:8080\ntg://proxy?server=2.2.2.2&port=443&secret=aabbccddeeff00112233445566778899\n"))
	}))
	defer srv.Close()

	svc, storage := newTestSourceService(t)
	result, err := svc.AddFromURL("mixed", srv.URL)
	if err != nil {
		t.Fatalf("AddFromURL: %v", err)
	}
	if result.Added != 1 || result.MTProtoAdded != 1 || result.MTProtoFetched != 1 {
		t.Fatalf("ожидалось Added=1, MTProtoAdded=1, MTProtoFetched=1, получено %+v", result)
	}

	proxies, err := storage.CountProxies(models.ProxyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	mt, err := storage.CountMTProto(models.MTProtoFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if proxies != 1 || mt != 1 {
		t.Fatalf("в БД proxies=%d mtproto=%d, ожидалось 1/1", proxies, mt)
	}

	sources, err := storage.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ProxyCount != 2 {
		t.Fatalf("счётчик источника должен учитывать MTProto: %+v", sources)
	}
}

func TestAddFromURLIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("1.1.1.1:8080\n2.2.2.2:1080\n"))
	}))
	defer srv.Close()

	svc, storage := newTestSourceService(t)
	first, err := svc.AddFromURL("тест", srv.URL)
	if err != nil {
		t.Fatalf("первый AddFromURL: %v", err)
	}
	if first.Added != 2 {
		t.Fatalf("ожидалось added=2, получено %d", first.Added)
	}

	second, err := svc.AddFromURL("тест", srv.URL)
	if err != nil {
		t.Fatalf("повторный AddFromURL: %v", err)
	}
	if second.Added != 0 {
		t.Fatalf("при повторе ожидалось added=0, получено %d", second.Added)
	}

	sources, err := storage.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("ожидался 1 источник (без дубликатов), получено %d", len(sources))
	}
	if sources[0].ProxyCount != 2 {
		t.Fatalf("счётчик источника = %d, ожидалось 2", sources[0].ProxyCount)
	}
}

func TestNormalizeGitHubURL(t *testing.T) {
	in := "https://github.com/proxifly/free-proxy-list/blob/main/proxies/protocols/socks5/data.txt"
	want := "https://raw.githubusercontent.com/proxifly/free-proxy-list/main/proxies/protocols/socks5/data.txt"
	if got := normalizeGitHubURL(in); got != want {
		t.Fatalf("normalizeGitHubURL = %q, ожидалось %q", got, want)
	}
	if got := normalizeGitHubURL(want); got != want {
		t.Fatalf("raw-ссылка не должна меняться: %q", got)
	}
}

func TestAddFromURLHTMLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>not a proxy list</body></html>"))
	}))
	defer srv.Close()

	svc, storage := newTestSourceService(t)
	if _, err := svc.AddFromURL("html", srv.URL); err == nil {
		t.Fatal("ожидалась ошибка для HTML-страницы")
	}
	sources, err := storage.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Fatalf("пустой источник не должен создаваться, получено %+v", sources)
	}
}

func TestAddFromURLEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("какая-то ерунда\nне прокси\n"))
	}))
	defer srv.Close()

	svc, _ := newTestSourceService(t)
	if _, err := svc.AddFromURL("empty", srv.URL); err == nil {
		t.Fatal("ожидалась ошибка при отсутствии прокси")
	}
}
