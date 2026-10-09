package services

import (
	"strconv"
	"strings"
	"time"
)

// Ключи настроек в таблице settings.
const (
	keyTheme              = "theme"
	keyLatencyTimeoutMs   = "latency_timeout_ms"
	keySpeedDownloadBytes = "speed_download_bytes"
	keyTestConcurrency    = "test_concurrency"
	keyCopyFormat         = "copy_format"
	keyValidateViaHTTP    = "validate_via_http"
	keyHTTPValidationURL  = "http_validation_url"
	keySpeedTestEnabled   = "speed_test_enabled"
	keyTestRandomOrder    = "test_random_order"
	keyGeoConsensus       = "geo_consensus"
	keyGeoCacheTTLDays    = "geo_cache_ttl_days"
	keySpeedSamples       = "speed_samples"
)

// Settings — пользовательские настройки приложения (camelCase для фронтенда).
type Settings struct {
	Theme              string `json:"theme"`
	LatencyTimeoutMs   int    `json:"latencyTimeoutMs"`
	SpeedDownloadBytes int    `json:"speedDownloadBytes"`
	TestConcurrency    int    `json:"testConcurrency"`
	CopyFormat         string `json:"copyFormat"`
	ValidateViaHTTP    bool   `json:"validateViaHttp"`
	HTTPValidationURL  string `json:"httpValidationUrl"`
	// SpeedTest включает измерение скорости в ходе обычной проверки.
	SpeedTest bool `json:"speedTest"`
	// TestRandomOrder обходит пул в случайном порядке (удобно искать рабочие).
	TestRandomOrder bool `json:"testRandomOrder"`
	// GeoConsensus определяет IP/страну/город по нескольким источникам.
	GeoConsensus bool `json:"geoConsensus"`
	// GeoCacheTTLDays — сколько дней хранить гео в кэше (0 — бессрочно).
	GeoCacheTTLDays int `json:"geoCacheTtlDays"`
	// SpeedSamples — сколько замеров скорости делать (1..3).
	SpeedSamples int `json:"speedSamples"`
}

// DefaultSettings — значения по умолчанию (см. docs/DATA_MODEL.md).
func DefaultSettings() Settings {
	return Settings{
		Theme:              "system",
		LatencyTimeoutMs:   5000,
		SpeedDownloadBytes: 1000000,
		TestConcurrency:    50,
		CopyFormat:         "uri",
		ValidateViaHTTP:    true,
		HTTPValidationURL:  cloudflareMetaURL,
		SpeedTest:          true,
		TestRandomOrder:    true,
		GeoConsensus:       true,
		GeoCacheTTLDays:    30,
		SpeedSamples:       1,
	}
}

// SettingsService читает и сохраняет настройки.
type SettingsService struct {
	storage *StorageService
}

func NewSettingsService(storage *StorageService) *SettingsService {
	return &SettingsService{storage: storage}
}

// Get возвращает настройки с подстановкой значений по умолчанию.
func (s *SettingsService) Get() (Settings, error) {
	values, err := s.storage.AllSettings()
	if err != nil {
		return Settings{}, err
	}
	return settingsFromMap(values), nil
}

// Update валидирует и сохраняет настройки, возвращает итоговое состояние.
func (s *SettingsService) Update(in Settings) (Settings, error) {
	in = clampSettings(in)
	if err := s.persist(in); err != nil {
		return Settings{}, err
	}
	return s.Get()
}

func settingsFromMap(values map[string]string) Settings {
	d := DefaultSettings()
	if v := values[keyTheme]; v != "" {
		d.Theme = v
	}
	if v, ok := values[keyLatencyTimeoutMs]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			d.LatencyTimeoutMs = n
		}
	}
	if v, ok := values[keySpeedDownloadBytes]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			d.SpeedDownloadBytes = n
		}
	}
	if v, ok := values[keyTestConcurrency]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			d.TestConcurrency = n
		}
	}
	if v := values[keyCopyFormat]; v != "" {
		d.CopyFormat = v
	}
	if v, ok := values[keyValidateViaHTTP]; ok {
		d.ValidateViaHTTP = v != "false" && v != "0"
	}
	if v := values[keyHTTPValidationURL]; v != "" {
		d.HTTPValidationURL = v
	}
	if v, ok := values[keySpeedTestEnabled]; ok {
		d.SpeedTest = v != "false" && v != "0"
	}
	if v, ok := values[keyTestRandomOrder]; ok {
		d.TestRandomOrder = v != "false" && v != "0"
	}
	if v, ok := values[keyGeoConsensus]; ok {
		d.GeoConsensus = v != "false" && v != "0"
	}
	if v, ok := values[keyGeoCacheTTLDays]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			d.GeoCacheTTLDays = n
		}
	}
	if v, ok := values[keySpeedSamples]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			d.SpeedSamples = n
		}
	}
	return clampSettings(d)
}

// clampSettings держит числовые поля в разумных границах.
func clampSettings(s Settings) Settings {
	if s.LatencyTimeoutMs < 500 {
		s.LatencyTimeoutMs = 500
	}
	if s.LatencyTimeoutMs > 60000 {
		s.LatencyTimeoutMs = 60000
	}
	if s.TestConcurrency < 1 {
		s.TestConcurrency = 1
	}
	if s.TestConcurrency > 500 {
		s.TestConcurrency = 500
	}
	if s.SpeedDownloadBytes < 100_000 {
		s.SpeedDownloadBytes = 100_000
	}
	if s.SpeedDownloadBytes > 10_000_000 {
		s.SpeedDownloadBytes = 10_000_000
	}
	switch s.CopyFormat {
	case "uri", "hostport":
	default:
		s.CopyFormat = "uri"
	}
	if strings.TrimSpace(s.HTTPValidationURL) == "" {
		s.HTTPValidationURL = DefaultSettings().HTTPValidationURL
	}
	if s.GeoCacheTTLDays < 0 {
		s.GeoCacheTTLDays = 0
	}
	if s.GeoCacheTTLDays > 365 {
		s.GeoCacheTTLDays = 365
	}
	if s.SpeedSamples < 1 {
		s.SpeedSamples = 1
	}
	if s.SpeedSamples > 3 {
		s.SpeedSamples = 3
	}
	return s
}

// ClearGeoCache очищает кэш гео (кнопка в настройках).
func (s *SettingsService) ClearGeoCache() (int, error) {
	return s.storage.ClearGeoCache()
}

// geoCacheTTL возвращает срок жизни кэша гео (0 — бессрочно).
func (s *SettingsService) geoCacheTTL() (time.Duration, error) {
	st, err := s.Get()
	if err != nil {
		return 0, err
	}
	if st.GeoCacheTTLDays <= 0 {
		return 0, nil
	}
	return time.Duration(st.GeoCacheTTLDays) * 24 * time.Hour, nil
}

func (s *SettingsService) persist(in Settings) error {
	pairs := map[string]string{
		keyTheme:              in.Theme,
		keyLatencyTimeoutMs:   strconv.Itoa(in.LatencyTimeoutMs),
		keySpeedDownloadBytes: strconv.Itoa(in.SpeedDownloadBytes),
		keyTestConcurrency:    strconv.Itoa(in.TestConcurrency),
		keyCopyFormat:         in.CopyFormat,
		keyValidateViaHTTP:    strconv.FormatBool(in.ValidateViaHTTP),
		keyHTTPValidationURL:  in.HTTPValidationURL,
		keySpeedTestEnabled:   strconv.FormatBool(in.SpeedTest),
		keyTestRandomOrder:    strconv.FormatBool(in.TestRandomOrder),
		keyGeoConsensus:       strconv.FormatBool(in.GeoConsensus),
		keyGeoCacheTTLDays:    strconv.Itoa(in.GeoCacheTTLDays),
		keySpeedSamples:       strconv.Itoa(in.SpeedSamples),
	}
	for key, value := range pairs {
		if err := s.storage.SetSetting(key, value); err != nil {
			return err
		}
	}
	return nil
}
