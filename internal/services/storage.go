package services

import (
	"context"
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
	if f.OnlyWorking != nil {
		if *f.OnlyWorking {
			query += " AND is_working = 1"
		} else {
			query += " AND is_working = 0"
		}
	}
	if f.Unchecked != nil {
		if *f.Unchecked {
			query += " AND last_checked IS NULL"
		} else {
			query += " AND last_checked IS NOT NULL"
		}
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
	query := proxyColumns + where + proxyOrder(f)

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
			city      sql.NullString
			exitIP    sql.NullString
			latitude  sql.NullFloat64
			longitude sql.NullFloat64
			latency   sql.NullInt64
			mbps      sql.NullFloat64
			lastStr   sql.NullString
			isWorking int
			sourceID  sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.Host, &p.Port, &p.Protocol, &country, &city, &exitIP,
			&latitude, &longitude, &latency, &mbps, &lastStr, &isWorking, &sourceID); err != nil {
			return nil, err
		}
		if country.Valid {
			p.Country = &country.String
		}
		if city.Valid {
			p.City = &city.String
		}
		if exitIP.Valid {
			p.ExitIP = &exitIP.String
		}
		if latitude.Valid {
			v := latitude.Float64
			p.Latitude = &v
		}
		if longitude.Valid {
			v := longitude.Float64
			p.Longitude = &v
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

// DeleteProxies удаляет прокси по ID (чанками, чтобы не упереться в лимит переменных SQLite).
func (s *StorageService) DeleteProxies(ids []int64) error {
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := "DELETE FROM proxies WHERE id IN (" + placeholders(len(batch)) + ")"
		if _, err := s.db.Exec(query, toArgs(batch)...); err != nil {
			return err
		}
	}
	return nil
}

// idChunkSize — размер порции для IN (...) и постраничной выборки.
const idChunkSize = 400

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func toArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

// UpdateTestResult сохраняет результат проверки (latency + гео). download_mbps
// не трогается — за него отвечает отдельный тест скорости.
func (s *StorageService) UpdateTestResult(r TestResult) error {
	_, err := s.db.Exec(`UPDATE proxies
		SET latency_ms = ?,
		    country = COALESCE(?, country),
		    city = COALESCE(?, city),
		    exit_ip = COALESCE(?, exit_ip),
		    latitude = COALESCE(?, latitude),
		    longitude = COALESCE(?, longitude),
		    last_checked = CURRENT_TIMESTAMP,
		    is_working = ?
		WHERE id = ?`,
		r.LatencyMs, r.Country, r.City, r.ExitIP, r.Latitude, r.Longitude, r.IsWorking, r.ProxyID)
	return err
}

// UpdateSpeedResult сохраняет только результат теста скорости.
func (s *StorageService) UpdateSpeedResult(r TestResult) error {
	_, err := s.db.Exec(`UPDATE proxies
		SET download_mbps = ?, last_checked = CURRENT_TIMESTAMP
		WHERE id = ?`, r.DownloadMbps, r.ProxyID)
	return err
}

const proxyColumns = `SELECT id, host, port, protocol, country, city, exit_ip, latitude, longitude,
	latency_ms, download_mbps, last_checked, is_working, source_id FROM proxies`

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

// GetProxiesByIDs возвращает прокси по списку ID (чанками).
func (s *StorageService) GetProxiesByIDs(ids []int64) ([]models.Proxy, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]models.Proxy, 0, len(ids))
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := proxyColumns + " WHERE id IN (" + placeholders(len(batch)) + ")"
		proxies, err := s.queryProxies(query, toArgs(batch)...)
		if err != nil {
			return nil, err
		}
		out = append(out, proxies...)
	}
	return out, nil
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

// ForEachProxy пролистывает прокси под фильтр страницами (keyset по id),
// не загружая весь пул в память. fn возвращает false для остановки.
func (s *StorageService) ForEachProxy(ctx context.Context, f models.ProxyFilter, pageSize int, fn func(models.Proxy) bool) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	where, baseArgs := buildProxyWhere(f)
	lastID := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		query := proxyColumns + where
		args := append([]any{}, baseArgs...)
		if lastID > 0 {
			query += " AND id > ?"
			args = append(args, lastID)
		}
		query += " ORDER BY id ASC LIMIT ?"
		args = append(args, pageSize)

		proxies, err := s.queryProxies(query, args...)
		if err != nil {
			return err
		}
		if len(proxies) == 0 {
			return nil
		}
		for _, p := range proxies {
			if !fn(p) {
				return nil
			}
		}
		lastID = proxies[len(proxies)-1].ID
	}
}

// ForEachProxyByIDs пролистывает прокси по списку ID чанками.
func (s *StorageService) ForEachProxyByIDs(ctx context.Context, ids []int64, fn func(models.Proxy) bool) error {
	for start := 0; start < len(ids); start += idChunkSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+idChunkSize, len(ids))
		proxies, err := s.GetProxiesByIDs(ids[start:end])
		if err != nil {
			return err
		}
		for _, p := range proxies {
			if !fn(p) {
				return nil
			}
		}
	}
	return nil
}

// ClearStatus сбрасывает результат проверки у прокси под фильтр.
func (s *StorageService) ClearStatus(f models.ProxyFilter) (int, error) {
	where, args := buildProxyWhere(f)
	res, err := s.db.Exec("UPDATE proxies SET latency_ms = NULL, download_mbps = NULL, last_checked = NULL, is_working = 0"+where, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ClearStatusByIDs сбрасывает результат проверки у указанных прокси (чанками).
func (s *StorageService) ClearStatusByIDs(ids []int64) (int, error) {
	total := 0
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := "UPDATE proxies SET latency_ms = NULL, download_mbps = NULL, last_checked = NULL, is_working = 0 WHERE id IN (" + placeholders(len(batch)) + ")"
		res, err := s.db.Exec(query, toArgs(batch)...)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, nil
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
