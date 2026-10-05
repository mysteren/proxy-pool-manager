package services

import (
	"context"
	"fmt"
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

func TestSortByDownloadNullsLast(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.InsertProxies([]models.ParsedProxy{
		{Host: "a", Port: 1, Protocol: "http"},
		{Host: "b", Port: 2, Protocol: "http"},
		{Host: "c", Port: 3, Protocol: "http"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, "UPDATE proxies SET download_mbps = 5 WHERE host = 'a'")
	mustExec(t, s, "UPDATE proxies SET download_mbps = 20 WHERE host = 'b'")

	got, err := s.GetProxies(models.ProxyFilter{SortBy: "download", SortDir: "desc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("получено %d, ожидалось 3", len(got))
	}
	if got[0].Host != "b" || got[1].Host != "a" {
		t.Fatalf("сортировка по скорости неверна: %+v", got)
	}
	if got[2].DownloadMbps != nil {
		t.Fatalf("строка без скорости должна быть последней: %+v", got[2])
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

// TestChunkedByIDs проверяет работу с большим числом ID (лимит переменных SQLite).
func TestChunkedByIDs(t *testing.T) {
	s := newTestStorage(t)
	const n = 1500
	proxies := make([]models.ParsedProxy, n)
	for i := range proxies {
		proxies[i] = models.ParsedProxy{Host: fmt.Sprintf("h%d.example", i), Port: 1000 + i, Protocol: "http"}
	}
	added, err := s.InsertProxies(proxies, nil)
	if err != nil {
		t.Fatal(err)
	}
	if added != n {
		t.Fatalf("added=%d, ожидалось %d", added, n)
	}

	rows, err := s.GetProxies(models.ProxyFilter{Limit: n + 10})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	got, err := s.GetProxiesByIDs(ids)
	if err != nil {
		t.Fatalf("GetProxiesByIDs: %v", err)
	}
	if len(got) != n {
		t.Fatalf("GetProxiesByIDs вернул %d, ожидалось %d", len(got), n)
	}

	mustExec(t, s, "UPDATE proxies SET is_working = 1, latency_ms = 42")
	cleared, err := s.ClearStatusByIDs(ids)
	if err != nil {
		t.Fatalf("ClearStatusByIDs: %v", err)
	}
	if cleared != n {
		t.Fatalf("cleared=%d, ожидалось %d", cleared, n)
	}

	if err := s.DeleteProxies(ids); err != nil {
		t.Fatalf("DeleteProxies: %v", err)
	}
	count, err := s.CountProxies(models.ProxyFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("после удаления count=%d, ожидалось 0", count)
	}
}

func TestForEachProxyAndClearStatus(t *testing.T) {
	s := newTestStorage(t)
	proxies := make([]models.ParsedProxy, 5)
	for i := range proxies {
		proxies[i] = models.ParsedProxy{Host: fmt.Sprintf("f%d.example", i), Port: 2000 + i, Protocol: "http"}
	}
	if _, err := s.InsertProxies(proxies, nil); err != nil {
		t.Fatal(err)
	}

	seen := 0
	if err := s.ForEachProxy(context.Background(), models.ProxyFilter{}, 2, func(models.Proxy) bool {
		seen++
		return true
	}); err != nil {
		t.Fatalf("ForEachProxy: %v", err)
	}
	if seen != 5 {
		t.Fatalf("ForEachProxy просмотрел %d, ожидалось 5", seen)
	}

	mustExec(t, s, "UPDATE proxies SET is_working = 1, latency_ms = 10 WHERE host = 'f0.example'")
	cleared, err := s.ClearStatus(models.ProxyFilter{OnlyWorking: boolPtr(true)})
	if err != nil {
		t.Fatalf("ClearStatus: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("ClearStatus сбросил %d, ожидалось 1", cleared)
	}
}
