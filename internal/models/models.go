package models

import "time"

// Proxy — запись пула прокси.
type Proxy struct {
	ID           int64      `json:"id"`
	Host         string     `json:"host"`
	Port         int        `json:"port"`
	Protocol     string     `json:"protocol"`
	Country      *string    `json:"country,omitempty"`
	City         *string    `json:"city,omitempty"`
	ExitIP       *string    `json:"exitIp,omitempty"`
	Latitude     *float64   `json:"latitude,omitempty"`
	Longitude    *float64   `json:"longitude,omitempty"`
	LatencyMs    *int       `json:"latencyMs,omitempty"`
	DownloadMbps *float64   `json:"downloadMbps,omitempty"`
	LastChecked  *time.Time `json:"lastChecked,omitempty"`
	IsWorking    bool       `json:"isWorking"`
	SourceID     *int64     `json:"sourceId,omitempty"`
}

// Source — источник списка прокси (URL, файл или ручной ввод без источника).
type Source struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	URL         *string    `json:"url,omitempty"`
	FilePath    *string    `json:"filePath,omitempty"`
	LastFetched *time.Time `json:"lastFetched,omitempty"`
	// ProxyCount не хранится в БД, а вычисляется через COUNT(*) по source_id.
	ProxyCount int       `json:"proxyCount"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ParsedProxy — прокси, полученный парсером до вставки в БД.
type ParsedProxy struct {
	Host     string
	Port     int
	Protocol string
	Country  *string
}

// ProxyFilter — фильтр, сортировка и пагинация для выборки пула.
// Пагинация серверная; общее число строк возвращает CountProxies.
type ProxyFilter struct {
	OnlyWorking *bool   `json:"onlyWorking,omitempty"`
	Unchecked   *bool   `json:"unchecked,omitempty"` // true: last_checked IS NULL; false: IS NOT NULL
	Protocol    *string `json:"protocol,omitempty"`
	MaxLatency  *int    `json:"maxLatency,omitempty"`
	Country     *string `json:"country,omitempty"`
	Search      *string `json:"search,omitempty"` // поиск по host
	SortBy      string  `json:"sortBy"`           // latency|host|lastChecked
	SortDir     string  `json:"sortDir"`          // asc|desc
	Limit       int     `json:"limit"`
	Offset      int     `json:"offset"`
}
