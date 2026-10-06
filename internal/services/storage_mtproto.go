package services

import (
	"context"
	"database/sql"
	"strings"

	"proxy-pool-manager/internal/models"
)

const mtprotoColumns = `SELECT id, host, port, secret, tg_type, ping_ms, method, is_working, last_checked, source_id
	FROM mtproto_proxies`

// InsertMTProto вставляет Telegram-прокси одной транзакцией.
func (s *StorageService) InsertMTProto(proxies []models.ParsedMTProto, sourceID *int64) (int, error) {
	if len(proxies) == 0 {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO mtproto_proxies (host, port, secret, tg_type, source_id)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	added := 0
	for _, p := range proxies {
		res, err := stmt.Exec(p.Host, p.Port, p.Secret, p.Type, sourceID)
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

func buildMTProtoWhere(f models.MTProtoFilter) (string, []any) {
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
	if f.Type != nil && *f.Type != "" {
		query += " AND tg_type = ?"
		args = append(args, *f.Type)
	}
	if f.Search != nil && *f.Search != "" {
		query += " AND host LIKE ?"
		args = append(args, "%"+*f.Search+"%")
	}
	return query, args
}

var mtprotoSortColumns = map[string]string{
	"ping":        "ping_ms",
	"host":        "host",
	"port":        "port",
	"lastChecked": "last_checked",
	"id":          "id",
}

func mtprotoOrder(f models.MTProtoFilter) string {
	column, ok := mtprotoSortColumns[f.SortBy]
	if !ok {
		column = "host"
	}
	dir := "ASC"
	if strings.EqualFold(f.SortDir, "desc") {
		dir = "DESC"
	}
	if column == "ping_ms" {
		return " ORDER BY ping_ms IS NULL, ping_ms " + dir + ", id ASC"
	}
	return " ORDER BY " + column + " " + dir + ", id ASC"
}

func scanMTProtoRows(rows *sql.Rows) ([]models.MTProtoProxy, error) {
	out := make([]models.MTProtoProxy, 0)
	for rows.Next() {
		var (
			p         models.MTProtoProxy
			method    sql.NullString
			ping      sql.NullInt64
			lastStr   sql.NullString
			isWorking int
			sourceID  sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.Host, &p.Port, &p.Secret, &p.Type, &ping, &method,
			&isWorking, &lastStr, &sourceID); err != nil {
			return nil, err
		}
		if ping.Valid {
			v := int(ping.Int64)
			p.PingMs = &v
		}
		if method.Valid {
			p.Method = method.String
		}
		p.IsWorking = isWorking != 0
		p.LastChecked = parseTimeString(lastStr)
		if sourceID.Valid {
			p.SourceID = &sourceID.Int64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *StorageService) queryMTProto(query string, args ...any) ([]models.MTProtoProxy, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMTProtoRows(rows)
}

// GetMTProto возвращает страницу Telegram-прокси согласно фильтру.
func (s *StorageService) GetMTProto(f models.MTProtoFilter) ([]models.MTProtoProxy, error) {
	where, args := buildMTProtoWhere(f)
	query := mtprotoColumns + where + mtprotoOrder(f)
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)
	return s.queryMTProto(query, args...)
}

// CountMTProto возвращает общее число Telegram-прокси под фильтр.
func (s *StorageService) CountMTProto(f models.MTProtoFilter) (int, error) {
	where, args := buildMTProtoWhere(f)
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM mtproto_proxies"+where, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetMTProtoByID возвращает Telegram-прокси по ID (или nil).
func (s *StorageService) GetMTProtoByID(id int64) (*models.MTProtoProxy, error) {
	list, err := s.queryMTProto(mtprotoColumns+" WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// GetMTProtoByIDs возвращает Telegram-прокси по списку ID (чанками).
func (s *StorageService) GetMTProtoByIDs(ids []int64) ([]models.MTProtoProxy, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]models.MTProtoProxy, 0, len(ids))
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := mtprotoColumns + " WHERE id IN (" + placeholders(len(batch)) + ")"
		list, err := s.queryMTProto(query, toArgs(batch)...)
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
	}
	return out, nil
}

// ForEachMTProto пролистывает Telegram-прокси под фильтр страницами (keyset по id).
func (s *StorageService) ForEachMTProto(ctx context.Context, f models.MTProtoFilter, pageSize int, fn func(models.MTProtoProxy) bool) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	where, baseArgs := buildMTProtoWhere(f)
	lastID := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		query := mtprotoColumns + where
		args := append([]any{}, baseArgs...)
		if lastID > 0 {
			query += " AND id > ?"
			args = append(args, lastID)
		}
		query += " ORDER BY id ASC LIMIT ?"
		args = append(args, pageSize)

		list, err := s.queryMTProto(query, args...)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		for _, p := range list {
			if !fn(p) {
				return nil
			}
		}
		lastID = list[len(list)-1].ID
	}
}

// ForEachMTProtoByIDs пролистывает Telegram-прокси по списку ID чанками.
func (s *StorageService) ForEachMTProtoByIDs(ctx context.Context, ids []int64, fn func(models.MTProtoProxy) bool) error {
	for start := 0; start < len(ids); start += idChunkSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+idChunkSize, len(ids))
		list, err := s.GetMTProtoByIDs(ids[start:end])
		if err != nil {
			return err
		}
		for _, p := range list {
			if !fn(p) {
				return nil
			}
		}
	}
	return nil
}

// DeleteMTProto удаляет Telegram-прокси по ID (чанками).
func (s *StorageService) DeleteMTProto(ids []int64) error {
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := "DELETE FROM mtproto_proxies WHERE id IN (" + placeholders(len(batch)) + ")"
		if _, err := s.db.Exec(query, toArgs(batch)...); err != nil {
			return err
		}
	}
	return nil
}

// UpdateMTProtoResult сохраняет результат проверки Telegram-прокси.
func (s *StorageService) UpdateMTProtoResult(r MTProtoResult) error {
	_, err := s.db.Exec(`UPDATE mtproto_proxies
		SET ping_ms = ?, method = ?, is_working = ?, last_checked = CURRENT_TIMESTAMP
		WHERE id = ?`, r.PingMs, r.Method, r.IsWorking, r.ProxyID)
	return err
}

// ClearMTProtoStatus сбрасывает результат проверки у Telegram-прокси под фильтр.
func (s *StorageService) ClearMTProtoStatus(f models.MTProtoFilter) (int, error) {
	where, args := buildMTProtoWhere(f)
	res, err := s.db.Exec("UPDATE mtproto_proxies SET ping_ms = NULL, method = NULL, last_checked = NULL, is_working = 0"+where, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ClearMTProtoStatusByIDs сбрасывает результат проверки у указанных прокси.
func (s *StorageService) ClearMTProtoStatusByIDs(ids []int64) (int, error) {
	total := 0
	for start := 0; start < len(ids); start += idChunkSize {
		end := min(start+idChunkSize, len(ids))
		batch := ids[start:end]
		query := "UPDATE mtproto_proxies SET ping_ms = NULL, method = NULL, last_checked = NULL, is_working = 0 WHERE id IN (" + placeholders(len(batch)) + ")"
		res, err := s.db.Exec(query, toArgs(batch)...)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, nil
}
