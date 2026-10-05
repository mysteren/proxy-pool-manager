package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"proxy-pool-manager/internal/models"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ProxyService — доступ фронтенда к пулу прокси (список, счётчик, удаление,
// копирование в буфер и экспорт в файлы).
type ProxyService struct {
	storage  *StorageService
	settings *SettingsService
}

func NewProxyService(storage *StorageService, settings *SettingsService) *ProxyService {
	return &ProxyService{storage: storage, settings: settings}
}

// GetProxies возвращает страницу пула согласно фильтру (серверная пагинация).
func (s *ProxyService) GetProxies(f models.ProxyFilter) ([]models.Proxy, error) {
	return s.storage.GetProxies(f)
}

// CountProxies возвращает общее число прокси под фильтр.
func (s *ProxyService) CountProxies(f models.ProxyFilter) (int, error) {
	return s.storage.CountProxies(f)
}

// DeleteProxies удаляет прокси по ID.
func (s *ProxyService) DeleteProxies(ids []int64) error {
	return s.storage.DeleteProxies(ids)
}

// ClearStatus сбрасывает результат проверки у прокси под фильтр.
func (s *ProxyService) ClearStatus(f models.ProxyFilter) (int, error) {
	return s.storage.ClearStatus(f)
}

// ClearStatusByIDs сбрасывает результат проверки у указанных прокси.
func (s *ProxyService) ClearStatusByIDs(ids []int64) (int, error) {
	return s.storage.ClearStatusByIDs(ids)
}

// CopyToClipboard копирует выбранные прокси в буфер обмена (формат из настроек).
func (s *ProxyService) CopyToClipboard(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ничего не выбрано")
	}
	st, err := s.settings.Get()
	if err != nil {
		return 0, err
	}
	proxies, err := s.storage.GetProxiesByIDs(ids)
	if err != nil {
		return 0, err
	}
	var b strings.Builder
	for i, p := range proxies {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatProxy(p, st.CopyFormat))
	}
	if b.Len() == 0 {
		return 0, nil
	}
	app := application.Get()
	if app == nil || !app.Clipboard.SetText(b.String()) {
		return 0, fmt.Errorf("не удалось записать в буфер обмена")
	}
	return len(proxies), nil
}

// PickExportPath открывает диалог сохранения файла и возвращает путь.
func (s *ProxyService) PickExportPath(format string) (string, error) {
	ext, filterName := ".txt", "Текст"
	switch format {
	case "csv":
		ext, filterName = ".csv", "CSV"
	case "json":
		ext, filterName = ".json", "JSON"
	}
	return application.Get().Dialog.SaveFile().
		SetMessage("Экспорт прокси").
		SetFilename("proxies"+ext).
		AddFilter(filterName, "*"+ext).
		PromptForSingleSelection()
}

// ExportByIDs экспортирует выбранные прокси в файл.
func (s *ProxyService) ExportByIDs(ids []int64, format, path string) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ничего не выбрано")
	}
	return s.export(path, format, func(yield func(models.Proxy) bool) {
		_ = s.storage.ForEachProxyByIDs(context.Background(), ids, yield)
	})
}

// ExportByFilter экспортирует прокси по фильтру (потоково, для больших объёмов).
func (s *ProxyService) ExportByFilter(filter models.ProxyFilter, format, path string) (int, error) {
	return s.export(path, format, func(yield func(models.Proxy) bool) {
		_ = s.storage.ForEachProxy(context.Background(), filter, 1000, yield)
	})
}

func (s *ProxyService) export(path, format string, feed func(yield func(models.Proxy) bool)) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("не задан путь для экспорта")
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	sink := newProxySink(format, w)
	count := 0
	var writeErr error
	feed(func(p models.Proxy) bool {
		if err := sink.Write(p); err != nil {
			writeErr = err
			return false
		}
		count++
		return true
	})
	if writeErr != nil {
		return count, writeErr
	}
	if err := sink.Close(); err != nil {
		return count, err
	}
	if err := w.Flush(); err != nil {
		return count, err
	}
	return count, file.Sync()
}
