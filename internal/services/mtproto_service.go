package services

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"proxy-pool-manager/internal/models"
	"proxy-pool-manager/internal/mtproto"
	"proxy-pool-manager/internal/parser"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/semaphore"
)

// MTProtoResult — результат проверки одного Telegram-прокси.
type MTProtoResult struct {
	ProxyID   int64    `json:"proxyId"`
	PingMs    *int     `json:"pingMs,omitempty"`
	JitterMs  *int     `json:"jitterMs,omitempty"`
	Successes int      `json:"successes"`
	Attempts  int      `json:"attempts"`
	Score     *float64 `json:"score,omitempty"`
	IsWorking bool     `json:"isWorking"`
	Method    string   `json:"method,omitempty"`
}

// mtprotoAttempts — сколько раз проверяем каждый прокси (для оценки надёжности и джиттера).
const mtprotoAttempts = 3

// geoLookupInterval — пауза между запросами к гео-сервисам, чтобы не превышать
// бесплатные лимиты (у ip-api — 45 запросов/мин с одного IP).
const geoLookupInterval = 1500 * time.Millisecond

// MTProtoService — список, проверка, копирование и экспорт Telegram-прокси.
type MTProtoService struct {
	storage  *StorageService
	settings *SettingsService

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool

	geoMu      sync.Mutex
	geoCancel  context.CancelFunc
	geoRunning bool
}

func NewMTProtoService(storage *StorageService, settings *SettingsService) *MTProtoService {
	return &MTProtoService{storage: storage, settings: settings}
}

// GetProxies возвращает страницу Telegram-прокси согласно фильтру.
func (s *MTProtoService) GetProxies(f models.MTProtoFilter) ([]models.MTProtoProxy, error) {
	return s.storage.GetMTProto(f)
}

// Count возвращает общее число Telegram-прокси под фильтр.
func (s *MTProtoService) Count(f models.MTProtoFilter) (int, error) {
	return s.storage.CountMTProto(f)
}

// Delete удаляет Telegram-прокси по ID.
func (s *MTProtoService) Delete(ids []int64) error {
	return s.storage.DeleteMTProto(ids)
}

// DeleteByFilter удаляет Telegram-прокси под фильтр.
func (s *MTProtoService) DeleteByFilter(f models.MTProtoFilter) (int, error) {
	return s.storage.DeleteMTProtoByFilter(f)
}

// ClearStatus сбрасывает статус проверки под фильтр.
func (s *MTProtoService) ClearStatus(f models.MTProtoFilter) (int, error) {
	return s.storage.ClearMTProtoStatus(f)
}

// ClearStatusByIDs сбрасывает статус у указанных прокси.
func (s *MTProtoService) ClearStatusByIDs(ids []int64) (int, error) {
	return s.storage.ClearMTProtoStatusByIDs(ids)
}

// CopyToClipboard копирует ссылки tg:// выбранных прокси.
func (s *MTProtoService) CopyToClipboard(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ничего не выбрано")
	}
	list, err := s.storage.GetMTProtoByIDs(ids)
	if err != nil {
		return 0, err
	}
	lines := make([]string, 0, len(list))
	for _, p := range list {
		lines = append(lines, parser.MTProtoLink(p))
	}
	if len(lines) == 0 {
		return 0, nil
	}
	app := application.Get()
	if app == nil || !app.Clipboard.SetText(strings.Join(lines, "\n")) {
		return 0, fmt.Errorf("не удалось записать в буфер обмена")
	}
	return len(lines), nil
}

// PickExportPath открывает диалог сохранения файла.
func (s *MTProtoService) PickExportPath(format string) (string, error) {
	ext, filterName := ".txt", "Текст"
	switch format {
	case "csv":
		ext, filterName = ".csv", "CSV"
	case "json":
		ext, filterName = ".json", "JSON"
	}
	return application.Get().Dialog.SaveFile().
		SetMessage("Экспорт Telegram-прокси").
		SetFilename("mtproto"+ext).
		AddFilter(filterName, "*"+ext).
		PromptForSingleSelection()
}

// ExportByIDs экспортирует выбранные Telegram-прокси в файл.
func (s *MTProtoService) ExportByIDs(ids []int64, format, path string) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ничего не выбрано")
	}
	return s.export(path, format, func(yield func(models.MTProtoProxy) bool) {
		_ = s.storage.ForEachMTProtoByIDs(context.Background(), ids, yield)
	})
}

// ExportByFilter экспортирует Telegram-прокси по фильтру (потоково).
func (s *MTProtoService) ExportByFilter(f models.MTProtoFilter, format, path string) (int, error) {
	return s.export(path, format, func(yield func(models.MTProtoProxy) bool) {
		_ = s.storage.ForEachMTProto(context.Background(), f, 1000, yield)
	})
}

func (s *MTProtoService) export(path, format string, feed func(yield func(models.MTProtoProxy) bool)) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("не задан путь для экспорта")
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	count := 0
	var writeErr error

	write := func(p models.MTProtoProxy) error { return nil }
	finalize := func() error { return nil }
	switch format {
	case "csv":
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"host", "port", "secret", "type", "ping_ms", "is_working"})
		write = func(p models.MTProtoProxy) error {
			ping := ""
			if p.PingMs != nil {
				ping = strconv.Itoa(*p.PingMs)
			}
			return cw.Write([]string{p.Host, strconv.Itoa(p.Port), p.Secret, p.Type, ping, strconv.FormatBool(p.IsWorking)})
		}
		finalize = func() error {
			cw.Flush()
			return cw.Error()
		}
	case "json":
		first := true
		write = func(p models.MTProtoProxy) error {
			sep := ",\n"
			if first {
				sep = "[\n"
				first = false
			}
			if _, err := w.WriteString(sep); err != nil {
				return err
			}
			data, err := json.Marshal(map[string]any{
				"host": p.Host, "port": p.Port, "secret": p.Secret,
				"type": p.Type, "pingMs": p.PingMs, "isWorking": p.IsWorking,
			})
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}
		finalize = func() error {
			if first {
				_, err := w.WriteString("[]\n")
				return err
			}
			_, err := w.WriteString("\n]\n")
			return err
		}
	default: // txt
		write = func(p models.MTProtoProxy) error {
			_, err := w.WriteString(parser.MTProtoLink(p) + "\n")
			return err
		}
	}

	feed(func(p models.MTProtoProxy) bool {
		if err := write(p); err != nil {
			writeErr = err
			return false
		}
		count++
		return true
	})
	if writeErr != nil {
		return count, writeErr
	}
	if err := finalize(); err != nil {
		return count, err
	}
	if err := w.Flush(); err != nil {
		return count, err
	}
	return count, file.Sync()
}

// TestProxies запускает проверку указанных Telegram-прокси.
func (s *MTProtoService) TestProxies(ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("не выбрано ни одного прокси")
	}
	feed := func(ctx context.Context, yield func(models.MTProtoProxy) bool) {
		_ = s.storage.ForEachMTProtoByIDs(ctx, ids, yield)
	}
	return s.startBatch(len(ids), feed)
}

// TestAll проверяет все Telegram-прокси.
func (s *MTProtoService) TestAll() error {
	return s.startFilterBatch(models.MTProtoFilter{})
}

// TestNonWorking проверяет проверенные, но нерабочие прокси.
func (s *MTProtoService) TestNonWorking() error {
	return s.startFilterBatch(models.MTProtoFilter{OnlyWorking: boolPtr(false), Unchecked: boolPtr(false)})
}

// TestUnchecked проверяет прокси без статуса.
func (s *MTProtoService) TestUnchecked() error {
	return s.startFilterBatch(models.MTProtoFilter{Unchecked: boolPtr(true)})
}

// Cancel останавливает активную проверку.
func (s *MTProtoService) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// LookupGeo определяет страну/город/координаты Telegram-прокси по IP сервера.
// Идёт фоном с ограничением частоты запросов к гео-сервисам.
func (s *MTProtoService) LookupGeo() error {
	total, err := s.storage.CountMTProtoMissingGeo()
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("нет прокси без гео")
	}
	s.geoMu.Lock()
	if s.geoRunning {
		s.geoMu.Unlock()
		return fmt.Errorf("определение гео уже выполняется")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.geoCancel = cancel
	s.geoRunning = true
	s.geoMu.Unlock()

	go s.runGeo(ctx, total)
	return nil
}

// CancelGeo останавливает определение гео.
func (s *MTProtoService) CancelGeo() {
	s.geoMu.Lock()
	cancel := s.geoCancel
	s.geoMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *MTProtoService) runGeo(ctx context.Context, total int) {
	defer func() {
		s.geoMu.Lock()
		s.geoRunning = false
		s.geoCancel = nil
		s.geoMu.Unlock()
	}()

	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{TLSClientConfig: testTLSConfig(tlsSkipVerify(s.settings))},
	}
	completed, resolved := 0, 0
	last := time.Time{}
	ttl, _ := s.settings.geoCacheTTL()

	_ = s.storage.ForEachMTProtoMissingGeo(ctx, func(p models.MTProtoProxy) bool {
		if ctx.Err() != nil {
			return false
		}
		// Троттлинг: бережём бесплатные лимиты гео-API.
		if wait := geoLookupInterval - time.Since(last); wait > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(wait):
			}
		}
		last = time.Now()

		if ip := resolveServerIP(ctx, p); ip != "" {
			// Сначала кэш по IP — гео меняется редко.
			info, ok := geoCacheLookup(s.storage, ip, ttl)
			if !ok {
				info, ok = lookupGeoByIP(ctx, client, ip)
				if ok {
					geoCacheStore(s.storage, ip, info)
				}
			}
			if ok {
				_ = s.storage.UpdateMTProtoGeo(p.ID, ip, info)
				resolved++
			}
		}
		completed++
		emitGeoProgress(TestProgress{Total: total, Completed: completed, Current: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))})
		return true
	})

	emitGeoProgress(TestProgress{Total: total, Completed: completed})
	emitGeoCompleted(TestCompleted{Cancelled: ctx.Err() != nil, Tested: completed, Working: resolved})
}

func (s *MTProtoService) startFilterBatch(f models.MTProtoFilter) error {
	total, err := s.storage.CountMTProto(f)
	if err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("нет прокси для проверки")
	}
	feed := func(ctx context.Context, yield func(models.MTProtoProxy) bool) {
		_ = s.storage.ForEachMTProto(ctx, f, 500, yield)
	}
	return s.startBatch(total, feed)
}

func (s *MTProtoService) startBatch(total int, feed func(ctx context.Context, yield func(models.MTProtoProxy) bool)) error {
	st, err := s.settings.Get()
	if err != nil {
		return err
	}
	timeout := time.Duration(st.LatencyTimeoutMs) * time.Millisecond
	concurrency := st.TestConcurrency
	if concurrency <= 0 {
		concurrency = 50
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

	go s.runBatch(ctx, total, feed, timeout, concurrency)
	return nil
}

func (s *MTProtoService) runBatch(ctx context.Context, total int, feed func(ctx context.Context, yield func(models.MTProtoProxy) bool), timeout time.Duration, concurrency int) {
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

	feed(ctx, func(p models.MTProtoProxy) bool {
		if ctx.Err() != nil {
			return false
		}
		if err := sem.Acquire(ctx, 1); err != nil {
			return false
		}
		wg.Add(1)
		go func(p models.MTProtoProxy) {
			defer wg.Done()
			defer sem.Release(1)

			result := s.probe(ctx, p, timeout)
			_ = s.storage.UpdateMTProtoResult(result)

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

// probe делает несколько попыток и оценивает качество канала:
// ping — средний RTT успешных рукопожатий, jitter — разброс (стабильность),
// successes/attempts — надёжность. score объединяет три метрики (больше — лучше).
func (s *MTProtoService) probe(ctx context.Context, p models.MTProtoProxy, timeout time.Duration) MTProtoResult {
	out := MTProtoResult{ProxyID: p.ID}
	pings := make([]int, 0, mtprotoAttempts)
	executed := 0
	for i := 0; i < mtprotoAttempts; i++ {
		if ctx.Err() != nil {
			break
		}
		executed++
		r := mtproto.Check(ctx, p.Host, p.Port, p.Secret, p.Type, timeout)
		if r.Working {
			out.Successes++
			pings = append(pings, r.PingMs)
			if out.Method == "" {
				out.Method = r.Method
			}
		}
	}
	out.Attempts = executed
	if len(pings) == 0 {
		return out
	}

	minPing, maxPing, sum := pings[0], pings[0], 0
	for _, v := range pings {
		if v < minPing {
			minPing = v
		}
		if v > maxPing {
			maxPing = v
		}
		sum += v
	}
	avg := sum / len(pings)
	jitter := maxPing - minPing
	out.PingMs = &avg
	out.JitterMs = &jitter
	out.IsWorking = true

	score := float64(out.Successes)/float64(executed)*1000 - float64(avg) - float64(jitter)
	out.Score = &score
	return out
}
