package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"proxy-pool-manager/internal/models"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/semaphore"
)

// TestResult — результат проверки одного прокси.
type TestResult struct {
	ProxyID      int64    `json:"proxyId"`
	LatencyMs    *int     `json:"latencyMs,omitempty"`
	DownloadMbps *float64 `json:"downloadMbps,omitempty"`
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

// TesterService проверяет прокси: TCP-connect (latency) + HTTP через сам прокси.
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
}

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

// TestProxies запускает массовую проверку указанных прокси (асинхронно).
func (s *TesterService) TestProxies(ids []int64) error {
	proxies, err := s.storage.GetProxiesByIDs(ids)
	if err != nil {
		return err
	}
	return s.startBatch(proxies)
}

// TestNonWorking запускает проверку всех нерабочих прокси (асинхронно).
func (s *TesterService) TestNonWorking() error {
	notWorking := false
	ids, err := s.storage.ListProxyIDs(models.ProxyFilter{OnlyWorking: &notWorking})
	if err != nil {
		return err
	}
	proxies, err := s.storage.GetProxiesByIDs(ids)
	if err != nil {
		return err
	}
	return s.startBatch(proxies)
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

func (s *TesterService) startBatch(proxies []models.Proxy) error {
	cfg, err := s.testConfig()
	if err != nil {
		return err
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

	go s.runBatch(ctx, proxies, cfg)
	return nil
}

func (s *TesterService) runBatch(ctx context.Context, proxies []models.Proxy, cfg testConfig) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	total := len(proxies)
	sem := semaphore.NewWeighted(int64(cfg.concurrency))

	var (
		wg        sync.WaitGroup
		statMu    sync.Mutex
		completed int
		working   int
		lastEmit  time.Time
	)

	for i := range proxies {
		if ctx.Err() != nil {
			break
		}
		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}
		wg.Add(1)
		go func(p models.Proxy) {
			defer wg.Done()
			defer sem.Release(1)

			result := s.testOne(ctx, p, cfg)
			if err := s.storage.UpdateTestResult(result); err != nil {
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
		}(proxies[i])
	}
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
		if err := s.validateHTTP(ctx, p, cfg, timeout); err != nil {
			result.Error = err.Error()
			return result
		}
	}

	result.IsWorking = true
	result.LatencyMs = &latencyMs
	return result
}

func (s *TesterService) validateHTTP(ctx context.Context, p models.Proxy, cfg testConfig, timeout time.Duration) error {
	transport, err := proxyTransport(p.Protocol, net.JoinHostPort(p.Host, strconv.Itoa(p.Port)), timeout)
	if err != nil {
		return err
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: timeout}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.validationURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
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
