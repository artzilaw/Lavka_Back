package httpapp

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type viewTracker struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

func newViewTracker(ttl time.Duration) *viewTracker {
	return &viewTracker{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// shouldCount возвращает true только если этот зритель ещё не смотрел это объявление в пределах ttl.
func (vt *viewTracker) shouldCount(viewerKey string, adID int64) bool {
	key := fmt.Sprintf("%s:%d", viewerKey, adID)
	now := time.Now()

	vt.mu.Lock()
	defer vt.mu.Unlock()

	if exp, ok := vt.entries[key]; ok && now.Before(exp) {
		return false
	}

	// Простая ленивая чистка, если записей накопилось много
	if len(vt.entries) > 10000 {
		for k, exp := range vt.entries {
			if now.After(exp) {
				delete(vt.entries, k)
			}
		}
	}

	vt.entries[key] = now.Add(vt.ttl)
	return true
}

func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
