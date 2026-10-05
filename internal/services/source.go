package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"proxy-pool-manager/internal/models"
	"proxy-pool-manager/internal/parser"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const maxSourceBytes = 20 << 20 // 20 МБ

// FetchResult — итог загрузки/разбора источника.
type FetchResult struct {
	SourceID int64    `json:"sourceId"`
	Fetched  int      `json:"fetched"`
	Added    int      `json:"added"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// SourceService загружает списки прокси из URL, файлов и ручного ввода.
type SourceService struct {
	storage *StorageService
	client  *http.Client
}

func NewSourceService(storage *StorageService) *SourceService {
	return &SourceService{
		storage: storage,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// AddFromURL создаёт источник и загружает прокси по URL.
func (s *SourceService) AddFromURL(name, rawURL string) (FetchResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return FetchResult{}, fmt.Errorf("URL должен начинаться с http:// или https://")
	}
	if strings.TrimSpace(name) == "" {
		name = defaultNameFromURL(rawURL)
	}
	id, err := s.storage.CreateSource(name, &rawURL, nil)
	if err != nil {
		return FetchResult{}, err
	}
	content, contentType, err := s.fetchURL(rawURL)
	if err != nil {
		return FetchResult{}, err
	}
	return s.ingest(id, content, contentType)
}

// AddFromFile создаёт источник из локального файла.
func (s *SourceService) AddFromFile(name, path string) (FetchResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return FetchResult{}, fmt.Errorf("путь к файлу не задан")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return FetchResult{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(path)
	}
	id, err := s.storage.CreateSource(name, nil, &path)
	if err != nil {
		return FetchResult{}, err
	}
	return s.ingest(id, content, "")
}

// AddManual добавляет прокси из ручного ввода прямо в пул (без источника).
func (s *SourceService) AddManual(text string) (FetchResult, error) {
	proxies, err := parser.ParseText(text)
	if err != nil {
		return FetchResult{}, err
	}
	added, err := s.storage.InsertProxies(proxies, nil)
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{
		Fetched: len(proxies),
		Added:   added,
		Skipped: len(proxies) - added,
	}, nil
}

// RefreshSource повторно загружает источник (URL или файл).
func (s *SourceService) RefreshSource(id int64) (FetchResult, error) {
	src, err := s.storage.GetSource(id)
	if err != nil {
		return FetchResult{}, err
	}
	if src == nil {
		return FetchResult{}, fmt.Errorf("источник %d не найден", id)
	}
	switch {
	case src.URL != nil:
		content, contentType, err := s.fetchURL(*src.URL)
		if err != nil {
			return FetchResult{}, err
		}
		return s.ingest(id, content, contentType)
	case src.FilePath != nil:
		content, err := os.ReadFile(*src.FilePath)
		if err != nil {
			return FetchResult{}, err
		}
		return s.ingest(id, content, "")
	default:
		return FetchResult{}, fmt.Errorf("у источника нет URL или файла")
	}
}

// ListSources возвращает список источников.
func (s *SourceService) ListSources() ([]models.Source, error) {
	return s.storage.ListSources()
}

// DeleteSource удаляет источник (прокси остаются в пуле).
func (s *SourceService) DeleteSource(id int64) error {
	return s.storage.DeleteSource(id)
}

// PickProxyFile открывает системный диалог выбора файла и возвращает путь.
// Пустая строка означает, что пользователь отменил выбор.
func (s *SourceService) PickProxyFile() (string, error) {
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Выберите файл со списком прокси").
		AddFilter("Текст", "*.txt").
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (s *SourceService) fetchURL(rawURL string) ([]byte, string, error) {
	resp, err := s.client.Get(rawURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// ingest разбирает содержимое, сохраняет прокси и обновляет метку источника.
func (s *SourceService) ingest(sourceID int64, content []byte, contentType string) (FetchResult, error) {
	proxies, err := parseContent(content, contentType)
	if err != nil {
		return FetchResult{}, err
	}
	added, err := s.storage.InsertProxies(proxies, &sourceID)
	if err != nil {
		return FetchResult{}, err
	}
	if err := s.storage.UpdateSourceLastFetched(sourceID); err != nil {
		return FetchResult{}, err
	}

	result := FetchResult{
		SourceID: sourceID,
		Fetched:  len(proxies),
		Added:    added,
		Skipped:  len(proxies) - added,
	}
	emitSourceFetched(result)
	return result, nil
}

func parseContent(content []byte, contentType string) ([]models.ParsedProxy, error) {
	ct := strings.ToLower(contentType)
	trimmed := bytes.TrimSpace(content)
	looksJSON := len(trimmed) > 0 && (trimmed[0] == '[' || trimmed[0] == '{')

	if strings.Contains(ct, "json") {
		return parser.ParseJSON(content)
	}
	if looksJSON {
		if proxies, err := parser.ParseJSON(content); err == nil {
			return proxies, nil
		}
	}
	return parser.ParseText(string(content))
}

func emitSourceFetched(result FetchResult) {
	if app := application.Get(); app != nil {
		app.Event.Emit("source:fetched", result)
	}
}

func defaultNameFromURL(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	return rawURL
}
