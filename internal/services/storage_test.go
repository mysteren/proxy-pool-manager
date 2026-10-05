package services

import (
	"path/filepath"
	"testing"

	"proxy-pool-manager/internal/db"
	"proxy-pool-manager/internal/models"
)

func newTestStorage(t *testing.T) *StorageService {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewStorageService(conn)
}

func mustExec(t *testing.T, s *StorageService, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func TestInsertProxiesDedup(t *testing.T) {
	s := newTestStorage(t)

	proxies := []models.ParsedProxy{
		{Host: "1.1.1.1", Port: 8080, Protocol: "http"},
		{Host: "1.1.1.1", Port: 8080, Protocol: "socks5"}, // другой протокол — отдельная строка
		{Host: "1.1.1.1", Port: 8080, Protocol: "http"},   // дубликат
	}
	added, err := s.InsertProxies(proxies, nil)
	if err != nil {
		t.Fatalf("InsertProxies: %v", err)
	}
	if added != 2 {
		t.Fatalf("ожидалось 2 добавленных, получено %d", added)
	}

	added, err = s.InsertProxies(proxies, nil)
	if err != nil {
		t.Fatalf("InsertProxies (повтор): %v", err)
	}
	if added != 0 {
		t.Fatalf("при повторе ожидалось 0 добавленных, получено %d", added)
	}
}

func TestGetProxiesFilterSortPaginate(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.InsertProxies([]models.ParsedProxy{
		{Host: "b.example", Port: 1, Protocol: "http"},
		{Host: "a.example", Port: 2, Protocol: "http"},
		{Host: "c.example", Port: 3, Protocol: "socks5"},
	}, nil); err != nil {
		t.Fatal(err)
	}

	mustExec(t, s, "UPDATE proxies SET is_working = 1, latency_ms = 100 WHERE host = 'a.example'")
	mustExec(t, s, "UPDATE proxies SET is_working = 1, latency_ms = 300 WHERE host = 'b.example'")

	onlyWorking := true
	got, err := s.GetProxies(models.ProxyFilter{OnlyWorking: &onlyWorking, SortBy: "latency", SortDir: "asc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Host != "a.example" || got[1].Host != "b.example" {
		t.Fatalf("фильтр/сортировка неверны: %+v", got)
	}

	count, err := s.CountProxies(models.ProxyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("CountProxies = %d, ожидалось 3", count)
	}

	page, err := s.GetProxies(models.ProxyFilter{SortBy: "host", SortDir: "asc", Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Host != "a.example" {
		t.Fatalf("страница 1 неверна: %+v", page)
	}
	page2, err := s.GetProxies(models.ProxyFilter{SortBy: "host", SortDir: "asc", Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Host != "c.example" {
		t.Fatalf("страница 2 неверна: %+v", page2)
	}
}

func TestDeleteSourceSetsNull(t *testing.T) {
	s := newTestStorage(t)
	url := "https://example.com/list.txt"
	id, err := s.CreateSource("test", &url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertProxies([]models.ParsedProxy{{Host: "9.9.9.9", Port: 80, Protocol: "http"}}, &id); err != nil {
		t.Fatal(err)
	}

	sources, err := s.ListSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ProxyCount != 1 {
		t.Fatalf("источник/счётчик неверны: %+v", sources)
	}

	if err := s.DeleteSource(id); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetProxies(models.ProxyFilter{SortBy: "host", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("прокси должен остаться в пуле, получено %d", len(got))
	}
	if got[0].SourceID != nil {
		t.Fatalf("ожидался source_id = NULL, получено %v", *got[0].SourceID)
	}
}
