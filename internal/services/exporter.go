package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"proxy-pool-manager/internal/models"
)

// proxyURI возвращает "protocol://host:port".
func proxyURI(p models.Proxy) string {
	return p.Protocol + "://" + p.Host + ":" + strconv.Itoa(p.Port)
}

// proxyHostPort возвращает "host:port".
func proxyHostPort(p models.Proxy) string {
	return p.Host + ":" + strconv.Itoa(p.Port)
}

// formatProxy форматирует прокси для буфера обмена.
func formatProxy(p models.Proxy, format string) string {
	if format == "hostport" {
		return proxyHostPort(p)
	}
	return proxyURI(p)
}

// proxySink потоково пишет прокси в выбранном формате.
type proxySink interface {
	Write(p models.Proxy) error
	Close() error
}

func newProxySink(format string, w io.Writer) proxySink {
	switch format {
	case "csv":
		return &csvSink{w: csv.NewWriter(w)}
	case "json":
		return &jsonSink{w: w}
	default:
		return txtSink{w: w}
	}
}

// txtSink — по одному "protocol://host:port" на строку.
type txtSink struct{ w io.Writer }

func (s txtSink) Write(p models.Proxy) error {
	_, err := fmt.Fprintln(s.w, proxyURI(p))
	return err
}

func (s txtSink) Close() error { return nil }

// csvSink — host,port,protocol,latency_ms,download_mbps,country.
type csvSink struct {
	w       *csv.Writer
	started bool
}

func (s *csvSink) Write(p models.Proxy) error {
	if !s.started {
		if err := s.w.Write([]string{"host", "port", "protocol", "latency_ms", "download_mbps", "country"}); err != nil {
			return err
		}
		s.started = true
	}
	latency := ""
	if p.LatencyMs != nil {
		latency = strconv.Itoa(*p.LatencyMs)
	}
	mbps := ""
	if p.DownloadMbps != nil {
		mbps = strconv.FormatFloat(*p.DownloadMbps, 'f', -1, 64)
	}
	country := ""
	if p.Country != nil {
		country = *p.Country
	}
	return s.w.Write([]string{p.Host, strconv.Itoa(p.Port), p.Protocol, latency, mbps, country})
}

func (s *csvSink) Close() error {
	s.w.Flush()
	return s.w.Error()
}

// jsonSink — массив объектов в camelCase.
type jsonSink struct {
	w     io.Writer
	count int
}

type exportProxy struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Protocol     string   `json:"protocol"`
	LatencyMs    *int     `json:"latencyMs,omitempty"`
	DownloadMbps *float64 `json:"downloadMbps,omitempty"`
	Country      *string  `json:"country,omitempty"`
}

func (s *jsonSink) Write(p models.Proxy) error {
	separator := ",\n"
	if s.count == 0 {
		separator = "[\n"
	}
	if _, err := io.WriteString(s.w, separator); err != nil {
		return err
	}
	s.count++

	item := exportProxy{
		Host:         p.Host,
		Port:         p.Port,
		Protocol:     p.Protocol,
		LatencyMs:    p.LatencyMs,
		DownloadMbps: p.DownloadMbps,
		Country:      p.Country,
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = s.w.Write(data)
	return err
}

func (s *jsonSink) Close() error {
	if s.count == 0 {
		_, err := io.WriteString(s.w, "[]\n")
		return err
	}
	_, err := io.WriteString(s.w, "\n]\n")
	return err
}
