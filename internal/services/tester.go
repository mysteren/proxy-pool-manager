package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxy-pool-manager/internal/models"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/semaphore"
)

const (
	// cloudflareMetaURL возвращает IP/страну/город/координаты клиента (exit-узла прокси).
	cloudflareMetaURL = "https://speed.cloudflare.com/meta"
	// cloudflareSpeedURLFormat — endpoint замера скорости.
	cloudflareSpeedURLFormat = "https://speed.cloudflare.com/__down?bytes=%d"
)

// builtinGeoURLs — независимые источники IP/гео для консенсусной проверки.
// Запросы идут через сам прокси, поэтому со стороны приложения нет общего
// лимита, а согласие нескольких источников отсекает ложные срабатывания.
var builtinGeoURLs = []string{
	cloudflareMetaURL,
	"https://ipwho.is/",
	"http://ip-api.com/json/?fields=status,country,countryCode,city,lat,lon,query",
}

// TestResult — результат проверки одного прокси.
type TestResult struct {
	ProxyID      int64    `json:"proxyId"`
	LatencyMs    *int     `json:"latencyMs,omitempty"`
	DownloadMbps *float64 `json:"downloadMbps,omitempty"`
	Country      *string  `json:"country,omitempty"`
	City         *string  `json:"city,omitempty"`
	ExitIP       *string  `json:"exitIp,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	IsWorking    bool     `json:"isWorking"`
	Error        string   `json:"error,omitempty"`
}

// TestProgress — прогресс массовой проверки.
type TestProgress struct {
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Current   string `json:"current"`
}

// TestCompleted — итог массовой проверки.
type TestCompleted struct {
	Cancelled bool `json:"cancelled"`
	Tested    int  `json:"tested"`
	Working   int  `json:"working"`
}

// MyLocation — местоположение пользователя (для расчёта расстояния до прокси).
type MyLocation struct {
	IP        string   `json:"ip"`
	Country   string   `json:"country"`
	City      string   `json:"city,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// TesterService проверяет прокси: TCP-connect, HTTP через прокси (с гео) и скорость.
type TesterService struct {
	storage  *StorageService
	settings *SettingsService

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewTesterService(storage *StorageService, settings *SettingsService) *TesterService {
	return &TesterService{storage: storage, settings: settings}
}

type testConfig struct {
	timeoutMs       int
	concurrency     int
	validateViaHTTP bool
	validationURL   string
	geoConsensus    bool
	speedBytes      int
	speedURL        string
	measureSpeed    bool
}

type proxyFeed func(ctx context.Context, yield func(models.Proxy) bool)
type proxyWorker func(ctx context.Context, p models.Proxy, cfg testConfig) TestResult

func boolPtr(b bool) *bool { return &b }

func strPtr(s string) *string { return &s }

// socks5Protocol — приложение работает только с SOCKS5 (плюс отдельно MTProto).
const socks5Protocol = "socks5"

// TestProxy проверяет один прокси синхронно и сохраняет результат.
func (s *TesterService) TestProxy(id int64) (TestResult, error) {
	p, err := s.storage.GetProxyByID(id)
	if err != nil {
		return TestResult{}, err
	}
	if p == nil {
		return TestResult{}, fmt.Errorf("прокси %d не найден", id)
	}
	cfg, err := s.testConfig()
	if err != nil {
		return TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.timeoutMs)*time.Millisecond+5*time.Second)
	defer cancel()

	result := s.testOne(ctx, *p, cfg)
	if err := s.storage.UpdateTestResult(result); err != nil {
		return result, err
	}
	return result, nil
}

// TestProxies запускает проверку указанных прокси (асинхронно).
func (s *TesterService) TestProxies(ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("не выбрано ни одного прокси")
	}
	feed := func(ctx context.Context, yield func(models.Proxy) bool) {
		_ = s.storage.ForEachProxyByIDs(ctx, ids, yield)
	}
	return s.startBatch(len(ids), feed, s.testOne, s.storage.UpdateTestResult, 0)
}

// TestAll проверяет весь пул (только SOCKS5).
func (s *TesterService) TestAll() error {
	return s.startFilterBatch(models.ProxyFilter{Protocol: strPtr(socks5Protocol)})
}

// TestNonWorking проверяет проверенные, но нерабочие прокси.
func (s *TesterService) TestNonWorking() error {
	return s.startFilterBatch(models.ProxyFilter{Protocol: strPtr(socks5Protocol), OnlyWorking: boolPtr(false), Unchecked: boolPtr(false)})
}

// TestUnchecked проверяет прокси без статуса (ещё не проверенные).
func (s *TesterService) TestUnchecked() error {
	return s.startFilterBatch(models.ProxyFilter{Protocol: strPtr(socks5Protocol), Unchecked: boolPtr(true)})
}

// GetMyLocation определяет местоположение пользователя (без прокси),
// опрашивая несколько источников и согласуя результат.
func (s *TesterService) GetMyLocation() (MyLocation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second}

	results := make([]geoResponse, len(builtinGeoURLs))
	var wg sync.WaitGroup
	for i, u := range builtinGeoURLs {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			info, err := fetchGeoDirect(ctx, client, u)
			results[i] = geoResponse{info: info, ok: err == nil}
		}(i, u)
	}
	wg.Wait()

	merged, ok := consensusGeo(results)
	if !ok {
		return MyLocation{}, fmt.Errorf("не удалось определить местоположение")
	}
	return MyLocation{
		IP:        merged.exitIP,
		Country:   merged.country,
		City:      merged.city,
		Latitude:  merged.latitude,
		Longitude: merged.longitude,
	}, nil
}

func fetchGeoDirect(ctx context.Context, client *http.Client, target string) (validationInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return validationInfo{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return validationInfo{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return validationInfo{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return validationInfo{}, fmt.Errorf("пустой ответ")
	}
	return parseGeo(body), nil
}

// geoByIPURLs — источники гео для запроса ПО IP (без прокси). %s — сам IP.
// Нужны для MTProto: через такой прокси HTTP-запрос не провести, поэтому гео
// определяем по IP сервера.
var geoByIPURLs = []string{
	"https://ipwho.is/%s",
	"http://ip-api.com/json/%s?fields=status,country,countryCode,city,lat,lon,query",
	"https://ipinfo.io/%s/json",
}

// lookupGeoByIP определяет гео по IP через несколько источников (консенсус).
func lookupGeoByIP(ctx context.Context, client *http.Client, ip string) (validationInfo, bool) {
	results := make([]geoResponse, len(geoByIPURLs))
	var wg sync.WaitGroup
	for i, tmpl := range geoByIPURLs {
		wg.Add(1)
		go func(i int, target string) {
			defer wg.Done()
			info, err := fetchGeoDirect(ctx, client, target)
			results[i] = geoResponse{info: info, ok: err == nil}
		}(i, fmt.Sprintf(tmpl, ip))
	}
	wg.Wait()
	return consensusGeo(results)
}

// resolveServerIP возвращает IP сервера прокси (из кэша или через DNS),
// предпочитая IPv4.
func resolveServerIP(ctx context.Context, p models.MTProtoProxy) string {
	if p.ServerIP != nil && *p.ServerIP != "" {
		return *p.ServerIP
	}
	if ip := net.ParseIP(p.Host); ip != nil {
		return ip.String()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(lookupCtx, p.Host)
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			return a
		}
	}
	if len(addrs) > 0 {
		return addrs[0]
	}
	return ""
}

// Cancel останавливает активную массовую проверку.
func (s *TesterService) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *TesterService) startFilterBatch(filter models.ProxyFilter) error {
	total, err := s.storage.CountProxies(filter)
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("нет прокси для проверки")
	}
	// Порядок обхода — по настройке. Случайный порядок удобен, когда нужно
	// быстро выудить рабочие прокси из большого пула.
	st, err := s.settings.Get()
	if err != nil {
		return err
	}
	random := st.TestRandomOrder
	feed := func(ctx context.Context, yield func(models.Proxy) bool) {
		if random {
			_ = s.storage.ForEachProxyRandom(ctx, filter, yield)
			return
		}
		_ = s.storage.ForEachProxy(ctx, filter, 500, yield)
	}
	return s.startBatch(total, feed, s.testOne, s.storage.UpdateTestResult, 0)
}

func (s *TesterService) startBatch(total int, feed proxyFeed, work proxyWorker, apply func(TestResult) error, concurrency int) error {
	cfg, err := s.testConfig()
	if err != nil {
		return err
	}
	if concurrency <= 0 {
		concurrency = cfg.concurrency
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("проверка уже выполняется")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()

	go s.runBatch(ctx, total, feed, cfg, work, apply, concurrency)
	return nil
}

func (s *TesterService) runBatch(ctx context.Context, total int, feed proxyFeed, cfg testConfig, work proxyWorker, apply func(TestResult) error, concurrency int) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	sem := semaphore.NewWeighted(int64(concurrency))
	var (
		wg        sync.WaitGroup
		statMu    sync.Mutex
		completed int
		working   int
		lastEmit  time.Time
	)

	// feed блокируется на семафоре при заполнении — это обратное давление на
	// чтение из БД: в памяти держится не более ~concurrency прокси.
	feed(ctx, func(p models.Proxy) bool {
		if ctx.Err() != nil {
			return false
		}
		if err := sem.Acquire(ctx, 1); err != nil {
			return false
		}
		wg.Add(1)
		go func(p models.Proxy) {
			defer wg.Done()
			defer sem.Release(1)

			result := work(ctx, p, cfg)
			if err := apply(result); err != nil {
				result.IsWorking = false
			}

			statMu.Lock()
			completed++
			if result.IsWorking {
				working++
			}
			if time.Since(lastEmit) >= 100*time.Millisecond || completed == total {
				lastEmit = time.Now()
				emitTestProgress(TestProgress{
					Total:     total,
					Completed: completed,
					Current:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
				})
			}
			statMu.Unlock()
		}(p)
		return true
	})
	wg.Wait()

	emitTestProgress(TestProgress{Total: total, Completed: completed})
	emitTestCompleted(TestCompleted{Cancelled: ctx.Err() != nil, Tested: completed, Working: working})
}

func (s *TesterService) testConfig() (testConfig, error) {
	st, err := s.settings.Get()
	if err != nil {
		return testConfig{}, err
	}
	return testConfig{
		timeoutMs:       st.LatencyTimeoutMs,
		concurrency:     st.TestConcurrency,
		validateViaHTTP: st.ValidateViaHTTP,
		validationURL:   st.HTTPValidationURL,
		geoConsensus:    st.GeoConsensus,
		speedBytes:      st.SpeedDownloadBytes,
		speedURL:        fmt.Sprintf(cloudflareSpeedURLFormat, st.SpeedDownloadBytes),
		measureSpeed:    st.SpeedTest,
	}, nil
}

func (s *TesterService) testOne(ctx context.Context, p models.Proxy, cfg testConfig) TestResult {
	result := TestResult{ProxyID: p.ID}
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	timeout := time.Duration(cfg.timeoutMs) * time.Millisecond

	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	latencyMs := int(time.Since(start).Milliseconds())

	if cfg.validateViaHTTP {
		info, err := s.validateHTTP(ctx, p, cfg, timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if info.exitIP != "" {
			result.ExitIP = &info.exitIP
		}
		if info.country != "" {
			result.Country = &info.country
		}
		if info.city != "" {
			result.City = &info.city
		}
		result.Latitude = info.latitude
		result.Longitude = info.longitude
	}

	if cfg.measureSpeed {
		mbps, err := s.measureSpeed(ctx, p, cfg, timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.DownloadMbps = mbps
	}

	result.IsWorking = true
	result.LatencyMs = &latencyMs
	return result
}

// measureSpeed измеряет скорость скачивания через прокси и возвращает Мбит/с.
// Нулевой размер и ошибки — это ошибка (прокси не пропускает данные).
func (s *TesterService) measureSpeed(ctx context.Context, p models.Proxy, cfg testConfig, timeout time.Duration) (*float64, error) {
	transport, err := proxyTransport(p.Protocol, net.JoinHostPort(p.Host, strconv.Itoa(p.Port)), timeout)
	if err != nil {
		return nil, err
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := cfg.speedURL
	if url == "" {
		url = fmt.Sprintf(cloudflareSpeedURLFormat, cfg.speedBytes)
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	start := time.Now()
	n, err := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	if err != nil && n == 0 {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("нулевая скорость")
	}
	if elapsed <= 0 {
		elapsed = 0.001
	}
	mbps := float64(n) * 8 / elapsed / 1e6
	return &mbps, nil
}

type validationInfo struct {
	exitIP    string
	country   string
	city      string
	latitude  *float64
	longitude *float64
}

// geoResponse — ответ одного источника (ok=false, если источник не ответил).
type geoResponse struct {
	info validationInfo
	ok   bool
}

// validationURLs собирает список источников: настроенный URL плюс встроенные,
// если включён консенсус (дубликаты убираются).
func validationURLs(cfg testConfig) []string {
	urls := make([]string, 0, len(builtinGeoURLs)+1)
	if u := strings.TrimSpace(cfg.validationURL); u != "" {
		urls = append(urls, u)
	}
	if cfg.geoConsensus {
		for _, u := range builtinGeoURLs {
			if !containsString(urls, u) {
				urls = append(urls, u)
			}
		}
	}
	if len(urls) == 0 {
		urls = append(urls, cloudflareMetaURL)
	}
	return urls
}

// validateHTTP проверяет прокси через несколько независимых источников и
// возвращает согласованный (консенсусный) результат. Прокси считается рабочим,
// если ответил хотя бы один источник.
func (s *TesterService) validateHTTP(ctx context.Context, p models.Proxy, cfg testConfig, timeout time.Duration) (validationInfo, error) {
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	urls := validationURLs(cfg)
	results := make([]geoResponse, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			info, err := fetchGeoThroughProxy(ctx, p.Protocol, addr, u, timeout)
			results[i] = geoResponse{info: info, ok: err == nil}
		}(i, u)
	}
	wg.Wait()

	merged, ok := consensusGeo(results)
	if !ok {
		return validationInfo{}, fmt.Errorf("прокси не ответил ни на один из %d источников", len(urls))
	}
	return merged, nil
}

func fetchGeoThroughProxy(ctx context.Context, protocol, addr, target string, timeout time.Duration) (validationInfo, error) {
	transport, err := proxyTransport(protocol, addr, timeout)
	if err != nil {
		return validationInfo{}, err
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: timeout}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return validationInfo{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return validationInfo{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return validationInfo{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return validationInfo{}, fmt.Errorf("пустой ответ прокси")
	}
	return parseGeo(body), nil
}

// consensusGeo сводит ответы источников: exit-IP/страна/город — по большинству,
// координаты — от источника, чей exit-IP совпал с консенсусным (иначе — первый
// ответивший с координатами).
func consensusGeo(results []geoResponse) (validationInfo, bool) {
	ipCounts := map[string]int{}
	countryCounts := map[string]int{}
	cityCounts := map[string]int{}
	responding := 0
	for _, r := range results {
		if !r.ok {
			continue
		}
		responding++
		if r.info.exitIP != "" {
			ipCounts[r.info.exitIP]++
		}
		if r.info.country != "" {
			countryCounts[r.info.country]++
		}
		if r.info.city != "" {
			cityCounts[r.info.city]++
		}
	}
	if responding == 0 {
		return validationInfo{}, false
	}
	merged := validationInfo{
		exitIP:  mostCommon(ipCounts),
		country: mostCommon(countryCounts),
		city:    mostCommon(cityCounts),
	}
	coordsFrom := func(matchIP bool) *validationInfo {
		for i := range results {
			r := &results[i]
			if !r.ok || r.info.latitude == nil || r.info.longitude == nil {
				continue
			}
			if matchIP && merged.exitIP != "" && r.info.exitIP != merged.exitIP {
				continue
			}
			return &r.info
		}
		return nil
	}
	if info := coordsFrom(true); info != nil {
		merged.latitude, merged.longitude = info.latitude, info.longitude
	} else if info := coordsFrom(false); info != nil {
		merged.latitude, merged.longitude = info.latitude, info.longitude
	}
	return merged, true
}

// parseGeo вытаскивает exit-IP, страну, город и координаты из ответа источника.
// Поддерживаются JSON (Cloudflare /meta, ip-api, ipwho.is, ipinfo) и trace
// (key=value). Разбор устойчив: координаты-строки тоже принимаются, а битое
// поле не обнуляет остальные.
func parseGeo(body []byte) validationInfo {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if info, ok := parseGeoJSON(trimmed); ok {
			return info
		}
	}
	return parseGeoKeyValue(trimmed)
}

func parseGeoJSON(body []byte) (validationInfo, bool) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return validationInfo{}, false
	}
	info := validationInfo{
		exitIP:  firstString(m, "clientIp", "ip", "query"),
		country: normalizeCountry(firstString(m, "countryCode", "country_code", "country")),
		city:    firstString(m, "city"),
	}
	info.latitude = firstFloat(m, "latitude", "lat")
	info.longitude = firstFloat(m, "longitude", "lon", "lng")
	if info.latitude == nil || info.longitude == nil {
		if lat, lon, ok := splitCoords(firstString(m, "loc")); ok {
			info.latitude, info.longitude = lat, lon
		}
	}
	return info, true
}

func parseGeoKeyValue(body []byte) validationInfo {
	var info validationInfo
	for _, line := range strings.Split(string(body), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ip":
			info.exitIP = strings.TrimSpace(value)
		case "loc":
			info.country = normalizeCountry(value)
		}
	}
	return info
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

// firstFloat принимает число или строку — источники присылают координаты и так, и так.
func firstFloat(m map[string]any, keys ...string) *float64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return &v
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return &f
			}
		}
	}
	return nil
}

func splitCoords(s string) (*float64, *float64, bool) {
	latStr, lonStr, ok := strings.Cut(s, ",")
	if !ok {
		return nil, nil, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(latStr), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(lonStr), 64)
	if err1 != nil || err2 != nil {
		return nil, nil, false
	}
	return &lat, &lon, true
}

func normalizeCountry(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// mostCommon возвращает самое частое значение; при равенстве — лексикографически
// меньшее (детерминированно).
func mostCommon(counts map[string]int) string {
	best := ""
	bestN := 0
	for k, n := range counts {
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}
	return best
}

func containsString(items []string, v string) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}

func proxyTransport(protocol, addr string, timeout time.Duration) (*http.Transport, error) {
	switch protocol {
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", addr, nil, &net.Dialer{Timeout: timeout})
		if err != nil {
			return nil, err
		}
		return &http.Transport{DialContext: contextDialFunc(dialer)}, nil
	default: // http
		return &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: addr})}, nil
	}
}

// contextDialFunc превращает proxy.Dialer в функцию с поддержкой context.
func contextDialFunc(d proxy.Dialer) func(ctx context.Context, network, address string) (net.Conn, error) {
	if cd, ok := d.(proxy.ContextDialer); ok {
		return cd.DialContext
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		type outcome struct {
			conn net.Conn
			err  error
		}
		ch := make(chan outcome, 1)
		go func() {
			conn, err := d.Dial(network, address)
			ch <- outcome{conn, err}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-ch:
			return res.conn, res.err
		}
	}
}

func emitTestProgress(p TestProgress) {
	if app := application.Get(); app != nil {
		app.Event.Emit("test:progress", p)
	}
}

func emitTestCompleted(c TestCompleted) {
	if app := application.Get(); app != nil {
		app.Event.Emit("test:completed", c)
	}
}

func emitGeoProgress(p TestProgress) {
	if app := application.Get(); app != nil {
		app.Event.Emit("geo:progress", p)
	}
}

func emitGeoCompleted(c TestCompleted) {
	if app := application.Get(); app != nil {
		app.Event.Emit("geo:completed", c)
	}
}
