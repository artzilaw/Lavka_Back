package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type metricKey struct {
	method string
	path   string
	status int
}

type durationKey struct {
	method string
	path   string
}

type durationStat struct {
	sum   float64
	count uint64
}

// MetricsRegistry собирает базовые HTTP-метрики (хиты, ошибки, тайминги) в формате Prometheus.
type MetricsRegistry struct {
	mu        sync.RWMutex
	requests  map[metricKey]uint64
	durations map[durationKey]durationStat
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		requests:  make(map[metricKey]uint64),
		durations: make(map[durationKey]durationStat),
	}
}

func normalizeMetricPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) > 0 && isAllDigits(p) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (m *MetricsRegistry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		dur := time.Since(start).Seconds()

		normPath := normalizeMetricPath(r.URL.Path)
		m.mu.Lock()
		m.requests[metricKey{method: r.Method, path: normPath, status: rec.status}]++
		dk := durationKey{method: r.Method, path: normPath}
		st := m.durations[dk]
		st.sum += dur
		st.count++
		m.durations[dk] = st
		m.mu.Unlock()
	})
}

func (m *MetricsRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var b strings.Builder
	b.WriteString("# HELP lavka_http_requests_total Total HTTP requests by method, path and status.\n")
	b.WriteString("# TYPE lavka_http_requests_total counter\n")

	reqKeys := make([]metricKey, 0, len(m.requests))
	for k := range m.requests {
		reqKeys = append(reqKeys, k)
	}
	sort.Slice(reqKeys, func(i, j int) bool {
		if reqKeys[i].path != reqKeys[j].path {
			return reqKeys[i].path < reqKeys[j].path
		}
		if reqKeys[i].method != reqKeys[j].method {
			return reqKeys[i].method < reqKeys[j].method
		}
		return reqKeys[i].status < reqKeys[j].status
	})
	for _, k := range reqKeys {
		fmt.Fprintf(&b, "lavka_http_requests_total{method=%q,path=%q,status=\"%d\"} %d\n",
			k.method, k.path, k.status, m.requests[k])
	}

	b.WriteString("# HELP lavka_http_errors_total Total HTTP 4xx/5xx error responses.\n")
	b.WriteString("# TYPE lavka_http_errors_total counter\n")
	for _, k := range reqKeys {
		if k.status >= 400 {
			fmt.Fprintf(&b, "lavka_http_errors_total{method=%q,path=%q,status=\"%d\"} %d\n",
				k.method, k.path, k.status, m.requests[k])
		}
	}

	b.WriteString("# HELP lavka_http_request_duration_seconds Request latency sum and count.\n")
	b.WriteString("# TYPE lavka_http_request_duration_seconds summary\n")
	durKeys := make([]durationKey, 0, len(m.durations))
	for k := range m.durations {
		durKeys = append(durKeys, k)
	}
	sort.Slice(durKeys, func(i, j int) bool {
		if durKeys[i].path != durKeys[j].path {
			return durKeys[i].path < durKeys[j].path
		}
		return durKeys[i].method < durKeys[j].method
	})
	for _, k := range durKeys {
		st := m.durations[k]
		fmt.Fprintf(&b, "lavka_http_request_duration_seconds_sum{method=%q,path=%q} %.6f\n",
			k.method, k.path, st.sum)
		fmt.Fprintf(&b, "lavka_http_request_duration_seconds_count{method=%q,path=%q} %d\n",
			k.method, k.path, st.count)
	}

	_, _ = w.Write([]byte(b.String()))
}
