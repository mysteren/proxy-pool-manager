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
