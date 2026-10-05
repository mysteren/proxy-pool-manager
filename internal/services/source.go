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
	rawURL = normalizeGitHubURL(rawURL)

	// Сначала загружаем и разбираем, и только потом создаём источник:
	// так при ошибке не остаётся пустой источник.
	proxies, err := s.loadURL(rawURL)
	if err != nil {
		return FetchResult{}, err
	}
	id, err := s.ensureSource(name, &rawURL, nil)
	if err != nil {
		return FetchResult{}, err
	}
	return s.ingest(id, proxies)
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
	proxies, err := parseContent(content, "")
	if err != nil {
		return FetchResult{}, err
	}
	proxies, err = requireProxies(proxies)
	if err != nil {
		return FetchResult{}, err
	}
	id, err := s.ensureSource(name, nil, &path)
	if err != nil {
		return FetchResult{}, err
	}
	return s.ingest(id, proxies)
}

// AddManual добавляет прокси из ручного ввода прямо в пул (без источника).
func (s *SourceService) AddManual(text string) (FetchResult, error) {
	proxies, err := parser.ParseText(text)
	if err != nil {
		return FetchResult{}, err
	}
	proxies, err = requireProxies(proxies)
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
		proxies, err := s.loadURL(*src.URL)
		if err != nil {
			return FetchResult{}, err
		}
		return s.ingest(id, proxies)
	case src.FilePath != nil:
		content, err := os.ReadFile(*src.FilePath)
		if err != nil {
			return FetchResult{}, err
		}
		proxies, err := parseContent(content, "")
		if err != nil {
			return FetchResult{}, err
		}
		proxies, err = requireProxies(proxies)
		if err != nil {
			return FetchResult{}, err
		}
		return s.ingest(id, proxies)
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

func (s *SourceService) loadURL(rawURL string) ([]models.ParsedProxy, error) {
	content, contentType, err := s.fetchURL(rawURL)
	if err != nil {
		return nil, err
	}
	proxies, err := parseContent(content, contentType)
	if err != nil {
		return nil, err
	}
	return requireProxies(proxies)
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

// ingest сохраняет прокси источника и обновляет метку загрузки.
func (s *SourceService) ingest(sourceID int64, proxies []models.ParsedProxy) (FetchResult, error) {
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
	if isHTML(contentType, content) {
		return nil, fmt.Errorf("источник вернул HTML-страницу, а не список прокси; для GitHub используйте ссылку raw.githubusercontent.com")
	}

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

func requireProxies(proxies []models.ParsedProxy) ([]models.ParsedProxy, error) {
	if len(proxies) == 0 {
		return nil, fmt.Errorf("в источнике не найдено ни одного прокси — проверьте, что ссылка ведёт на текстовый или JSON список")
	}
	return proxies, nil
}

func isHTML(contentType string, content []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	head := strings.ToLower(string(safeHead(content, 64)))
	return strings.HasPrefix(head, "<!doctype") || strings.HasPrefix(head, "<html")
}

func safeHead(b []byte, n int) []byte {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > n {
		return trimmed[:n]
	}
	return trimmed
}

// normalizeGitHubURL превращает ссылку на страницу GitHub в прямую raw-ссылку:
// https://github.com/<o>/<r>/blob/<ref>/<path> → https://raw.githubusercontent.com/<o>/<r>/<ref>/<path>
func normalizeGitHubURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host != "github.com" {
		return rawURL
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 5 || parts[2] != "blob" {
		return rawURL
	}
	rawParts := append([]string{}, parts[:2]...)
	rawParts = append(rawParts, parts[3:]...)
	u.Host = "raw.githubusercontent.com"
	u.Path = "/" + strings.Join(rawParts, "/")
	u.RawQuery = ""
	return u.String()
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

// ensureSource находит существующий источник по URL/пути или создаёт новый:
// повторное добавление того же источника обновляет его, а не плодит дубликаты.
func (s *SourceService) ensureSource(name string, url, filePath *string) (int64, error) {
	if url != nil {
		existing, err := s.storage.FindSourceByURL(*url)
		if err != nil {
			return 0, err
		}
		if existing != nil {
			return existing.ID, nil
		}
		return s.storage.CreateSource(defaultSourceName(name, defaultNameFromURL(*url)), url, nil)
	}
	if filePath != nil {
		existing, err := s.storage.FindSourceByFilePath(*filePath)
		if err != nil {
			return 0, err
		}
		if existing != nil {
			return existing.ID, nil
		}
		return s.storage.CreateSource(defaultSourceName(name, filepath.Base(*filePath)), nil, filePath)
	}
	return 0, fmt.Errorf("не задан ни URL, ни путь к файлу")
}

func defaultSourceName(name, fallback string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return fallback
}
