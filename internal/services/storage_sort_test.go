package services

import (
	"testing"

	"proxy-pool-manager/internal/models"
)

func proxyHosts(list []models.Proxy) []string {
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.Host
	}
	return out
}

// TestProxyDistanceAndQualitySort проверяет сортировку по расстоянию, оценку
// качества и её монотонность: у близкого/быстрого/низколатентного прокси оценка
// должна быть выше, чем у далёкого/медленного.
func TestProxyDistanceAndQualitySort(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.InsertProxies([]models.ParsedProxy{
		{Host: "near.example", Port: 1, Protocol: "socks5"},
		{Host: "far.example", Port: 2, Protocol: "socks5"},
	}, nil); err != nil {
		t.Fatal(err)
	}

	// near — рядом с Москвой (55.75, 37.62), низкий пинг и высокая скорость.
	// far — Нью-Йорк, высокий пинг и низкая скорость.
	mustExec(t, s, `UPDATE proxies SET is_working = 1, latency_ms = 20, download_mbps = 50,
		latitude = 55.76, longitude = 37.62 WHERE host = 'near.example'`)
	mustExec(t, s, `UPDATE proxies SET is_working = 1, latency_ms = 200, download_mbps = 5,
		latitude = 40.71, longitude = -74.0 WHERE host = 'far.example'`)

	lat, lon := 55.75, 37.62

	got, err := s.GetProxies(models.ProxyFilter{SortBy: "distance", SortDir: "asc", Limit: 10, OriginLat: &lat, OriginLon: &lon})
	if err != nil {
		t.Fatalf("GetProxies(distance): %v", err)
	}
	if len(got) != 2 || got[0].Host != "near.example" {
		t.Fatalf("сортировка по расстоянию: ожидался near первым, получено %v", proxyHosts(got))
	}
	if got[0].Quality == nil || got[1].Quality == nil {
		t.Fatalf("quality не вычислен: %+v", got)
	}
	if *got[0].Quality <= *got[1].Quality {
		t.Fatalf("качество near.example должно быть выше: %v vs %v", *got[0].Quality, *got[1].Quality)
	}

	gotQ, err := s.GetProxies(models.ProxyFilter{SortBy: "quality", SortDir: "desc", Limit: 10, OriginLat: &lat, OriginLon: &lon})
	if err != nil {
		t.Fatalf("GetProxies(quality): %v", err)
	}
	if len(gotQ) != 2 || gotQ[0].Host != "near.example" {
		t.Fatalf("сортировка по качеству: ожидался near первым, получено %v", proxyHosts(gotQ))
	}

	// Без координат пользователя сортировка по расстоянию не должна падать.
	if _, err := s.GetProxies(models.ProxyFilter{SortBy: "distance", Limit: 10}); err != nil {
		t.Fatalf("GetProxies(distance) без origin: %v", err)
	}
}

// TestMTProtoDistanceSort проверяет сортировку MTProto по расстоянию.
func TestMTProtoDistanceSort(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.InsertMTProto([]models.ParsedMTProto{
		{Host: "nearm.example", Port: 443, Type: "mtproto"},
		{Host: "farm.example", Port: 443, Type: "mtproto"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, "UPDATE mtproto_proxies SET latitude = 55.76, longitude = 37.62 WHERE host = 'nearm.example'")
	mustExec(t, s, "UPDATE mtproto_proxies SET latitude = 40.71, longitude = -74.0 WHERE host = 'farm.example'")

	lat, lon := 55.75, 37.62
	got, err := s.GetMTProto(models.MTProtoFilter{SortBy: "distance", SortDir: "asc", Limit: 10, OriginLat: &lat, OriginLon: &lon})
	if err != nil {
		t.Fatalf("GetMTProto(distance): %v", err)
	}
	if len(got) != 2 || got[0].Host != "nearm.example" {
		t.Fatalf("сортировка MTProto по расстоянию: ожидался nearm первым, получено %d записей", len(got))
	}
}
