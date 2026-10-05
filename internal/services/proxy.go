package services

import "proxy-pool-manager/internal/models"

// ProxyService — доступ фронтенда к пулу прокси (список, счётчик, удаление).
type ProxyService struct {
	storage *StorageService
}

func NewProxyService(storage *StorageService) *ProxyService {
	return &ProxyService{storage: storage}
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
