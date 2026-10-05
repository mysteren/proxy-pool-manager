package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"proxy-pool-manager/internal/models"
)

// StorageService — CRUD над SQLite. Не регистрируется как Wails-сервис:
// используется другими сервисами (SourceService, позже ProxyService/TesterService).
type StorageService struct {
	db *sql.DB
}

func NewStorageService(db *sql.DB) *StorageService {
	return &StorageService{db: db}
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
}

func parseTimeString(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, ns.String); err == nil {
			return &t
		}
	}
	return nil
}

// InsertProxies вставляет прокси одной транзакцией и возвращает число реально
// добавленных строк (дубликаты по host+port+protocol игнорируются).
func (s *StorageService) InsertProxies(proxies []models.ParsedProxy, sourceID *int64) (int, error) {
	if len(proxies) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO proxies (host, port, protocol, country, source_id)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	added := 0
	for _, p := range proxies {
		res, err := stmt.Exec(p.Host, p.Port, p.Protocol, p.Country, sourceID)
		if err != nil {
			return added, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
	}
	if err := tx.Commit(); err != nil {
		return added, err
	}
	return added, nil
}

var proxySortColumns = map[string]string{
	"latency":     "latency_ms",
	"host":        "host",
	"port":        "port",
	"protocol":    "protocol",
	"lastChecked": "last_checked",
	"id":          "id",
}

func buildProxyWhere(f models.ProxyFilter) (string, []any) {
	query := " WHERE 1=1"
	args := []any{}
	if f.OnlyWorking != nil && *f.OnlyWorking {
		query += " AND is_working = 1"
	}
	if f.Protocol != nil && *f.Protocol != "" {
		query += " AND protocol = ?"
		args = append(args, *f.Protocol)
	}
	if f.MaxLatency != nil {
		query += " AND latency_ms IS NOT NULL AND latency_ms <= ?"
		args = append(args, *f.MaxLatency)
	}
	if f.Country != nil && *f.Country != "" {
		query += " AND country = ?"
		args = append(args, *f.Country)
	}
	if f.Search != nil && *f.Search != "" {
		query += " AND host LIKE ?"
		args = append(args, "%"+*f.Search+"%")
	}
	return query, args
}

func proxyOrder(f models.ProxyFilter) string {
	column, ok := proxySortColumns[f.SortBy]
	if !ok {
		column = "host"
	}
	dir := "ASC"
	if strings.EqualFold(f.SortDir, "desc") {
		dir = "DESC"
	}
	// NULL latency всегда в конце, независимо от направления.
	if column == "latency_ms" {
		return fmt.Sprintf(" ORDER BY latency_ms IS NULL, latency_ms %s", dir)
	}
	return fmt.Sprintf(" ORDER BY %s %s", column, dir)
}

// GetProxies возвращает страницу пула согласно фильтру.
func (s *StorageService) GetProxies(f models.ProxyFilter) ([]models.Proxy, error) {
	where, args := buildProxyWhere(f)
	query := `SELECT id, host, port, protocol, country, latency_ms, download_mbps,
		last_checked, is_working, source_id FROM proxies` + where + proxyOrder(f)

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	return s.queryProxies(query, args...)
}

// scanProxyRows читает строки выборки прокси.
func scanProxyRows(rows *sql.Rows) ([]models.Proxy, error) {
	proxies := make([]models.Proxy, 0)
	for rows.Next() {
		var (
			p         models.Proxy
			country   sql.NullString
			latency   sql.NullInt64
			mbps      sql.NullFloat64
			lastStr   sql.NullString
			isWorking int
			sourceID  sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.Host, &p.Port, &p.Protocol, &country, &latency,
			&mbps, &lastStr, &isWorking, &sourceID); err != nil {
			return nil, err
		}
		if country.Valid {
			p.Country = &country.String
		}
		if latency.Valid {
			v := int(latency.Int64)
			p.LatencyMs = &v
		}
		if mbps.Valid {
			v := mbps.Float64
			p.DownloadMbps = &v
		}
		p.LastChecked = parseTimeString(lastStr)
		p.IsWorking = isWorking != 0
		if sourceID.Valid {
			p.SourceID = &sourceID.Int64
		}
		proxies = append(proxies, p)
	}
	return proxies, rows.Err()
}

// queryProxies выполняет запрос выборки прокси с общим сканированием.
func (s *StorageService) queryProxies(query string, args ...any) ([]models.Proxy, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProxyRows(rows)
}

// CountProxies возвращает общее число строк под фильтр (без пагинации).
func (s *StorageService) CountProxies(f models.ProxyFilter) (int, error) {
	where, args := buildProxyWhere(f)
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM proxies"+where, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteProxies удаляет прокси по ID.
func (s *StorageService) DeleteProxies(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "DELETE FROM proxies WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.Exec(query, args...)
	return err
}

// UpdateTestResult сохраняет результат проверки прокси.
func (s *StorageService) UpdateTestResult(r TestResult) error {
	_, err := s.db.Exec(`UPDATE proxies
		SET latency_ms = ?, download_mbps = ?, last_checked = CURRENT_TIMESTAMP, is_working = ?
		WHERE id = ?`, r.LatencyMs, r.DownloadMbps, r.IsWorking, r.ProxyID)
	return err
}

const proxyColumns = `SELECT id, host, port, protocol, country, latency_ms, download_mbps,
	last_checked, is_working, source_id FROM proxies`

// GetProxyByID возвращает прокси по ID (или nil).
func (s *StorageService) GetProxyByID(id int64) (*models.Proxy, error) {
	proxies, err := s.queryProxies(proxyColumns+" WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(proxies) == 0 {
		return nil, nil
	}
	return &proxies[0], nil
}

// GetProxiesByIDs возвращает прокси по списку ID.
func (s *StorageService) GetProxiesByIDs(ids []int64) ([]models.Proxy, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := proxyColumns + " WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	return s.queryProxies(query, args...)
}

// ListProxyIDs возвращает ID прокси под фильтр (для массовой проверки).
func (s *StorageService) ListProxyIDs(f models.ProxyFilter) ([]int64, error) {
	where, args := buildProxyWhere(f)
	rows, err := s.db.Query("SELECT id FROM proxies"+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

const sourceSelect = `SELECT s.id, s.name, s.url, s.file_path, s.last_fetched, s.created_at,
	(SELECT COUNT(*) FROM proxies p WHERE p.source_id = s.id) AS proxy_count
	FROM sources s`

func scanSource(scan func(dest ...any) error) (models.Source, error) {
	var (
		src        models.Source
		urlStr     sql.NullString
		pathStr    sql.NullString
		lastStr    sql.NullString
		createdStr sql.NullString
	)
	if err := scan(&src.ID, &src.Name, &urlStr, &pathStr, &lastStr, &createdStr, &src.ProxyCount); err != nil {
		return models.Source{}, err
	}
	if urlStr.Valid {
		src.URL = &urlStr.String
	}
	if pathStr.Valid {
		src.FilePath = &pathStr.String
	}
	src.LastFetched = parseTimeString(lastStr)
	if t := parseTimeString(createdStr); t != nil {
		src.CreatedAt = *t
	}
	return src, nil
}

// ListSources возвращает источники с посчитанным количеством прокси.
func (s *StorageService) ListSources() ([]models.Source, error) {
	rows, err := s.db.Query(sourceSelect + " ORDER BY s.created_at DESC, s.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]models.Source, 0)
	for rows.Next() {
		src, err := scanSource(rows.Scan)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// GetSource возвращает источник по ID.
func (s *StorageService) GetSource(id int64) (*models.Source, error) {
	row := s.db.QueryRow(sourceSelect+" WHERE s.id = ?", id)
	src, err := scanSource(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &src, nil
}

// FindSourceByURL возвращает источник с таким URL или nil.
func (s *StorageService) FindSourceByURL(url string) (*models.Source, error) {
	row := s.db.QueryRow(sourceSelect+" WHERE s.url = ? LIMIT 1", url)
	src, err := scanSource(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}

// FindSourceByFilePath возвращает источник с таким путём к файлу или nil.
func (s *StorageService) FindSourceByFilePath(path string) (*models.Source, error) {
	row := s.db.QueryRow(sourceSelect+" WHERE s.file_path = ? LIMIT 1", path)
	src, err := scanSource(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}

// CreateSource создаёт источник и возвращает его ID.
func (s *StorageService) CreateSource(name string, url, filePath *string) (int64, error) {
	res, err := s.db.Exec("INSERT INTO sources (name, url, file_path) VALUES (?, ?, ?)", name, url, filePath)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateSourceLastFetched отмечает время последней загрузки источника.
func (s *StorageService) UpdateSourceLastFetched(id int64) error {
	_, err := s.db.Exec("UPDATE sources SET last_fetched = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// DeleteSource удаляет источник; у связанных прокси source_id обнуляется (ON DELETE SET NULL).
func (s *StorageService) DeleteSource(id int64) error {
	_, err := s.db.Exec("DELETE FROM sources WHERE id = ?", id)
	return err
}

// GetSetting возвращает значение настройки (пустая строка, если не задана).
func (s *StorageService) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting сохраняет настройку.
func (s *StorageService) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// AllSettings возвращает все настройки как map.
func (s *StorageService) AllSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, rows.Err()
}
