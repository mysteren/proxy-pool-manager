package services

import (
	"database/sql"
	"time"
)

// GetGeoCache возвращает гео по IP из кэша.
func (s *StorageService) GetGeoCache(ip string) (validationInfo, time.Time, bool, error) {
	var (
		country, city sql.NullString
		lat, lon      sql.NullFloat64
		fetchedStr    sql.NullString
	)
	err := s.db.QueryRow("SELECT country, city, latitude, longitude, fetched_at FROM geo_cache WHERE ip = ?", ip).
		Scan(&country, &city, &lat, &lon, &fetchedStr)
	if err == sql.ErrNoRows {
		return validationInfo{}, time.Time{}, false, nil
	}
	if err != nil {
		return validationInfo{}, time.Time{}, false, err
	}

	var info validationInfo
	if country.Valid {
		info.country = country.String
	}
	if city.Valid {
		info.city = city.String
	}
	if lat.Valid {
		v := lat.Float64
		info.latitude = &v
	}
	if lon.Valid {
		v := lon.Float64
		info.longitude = &v
	}
	fetchedAt := time.Time{}
	if t := parseTimeString(fetchedStr); t != nil {
		fetchedAt = *t
	}
	return info, fetchedAt, true, nil
}

// PutGeoCache сохраняет/обновляет гео по IP.
func (s *StorageService) PutGeoCache(ip string, info validationInfo) error {
	_, err := s.db.Exec(`INSERT INTO geo_cache (ip, country, city, latitude, longitude, fetched_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(ip) DO UPDATE SET
			country = excluded.country,
			city = excluded.city,
			latitude = excluded.latitude,
			longitude = excluded.longitude,
			fetched_at = CURRENT_TIMESTAMP`,
		ip, strOrNil(info.country), strOrNil(info.city), info.latitude, info.longitude)
	return err
}

// ClearGeoCache очищает кэш гео, возвращает число удалённых строк.
func (s *StorageService) ClearGeoCache() (int, error) {
	res, err := s.db.Exec("DELETE FROM geo_cache")
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
