// ===================================================================================
// File: ddns-server/security/middleware.go
// ===================================================================================
package security

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	clients           = make(map[string]time.Time)
	clientsMutex      = &sync.Mutex{}
	rateLimitDuration = 5 * time.Second
)

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.Printf("警告: 无法解析IP地址: %v", err)
			ip = r.RemoteAddr
		}
		clientsMutex.Lock()
		lastSeen, exists := clients[ip]
		if exists && time.Since(lastSeen) < rateLimitDuration {
			clientsMutex.Unlock()
			log.Printf("速率限制: IP %s 的请求过于频繁。", ip)
			http.Error(w, "请求过于频繁，请稍后再试。", http.StatusTooManyRequests)
			return
		}
		clients[ip] = time.Now()
		clientsMutex.Unlock()
		next.ServeHTTP(w, r)
	})
}
func LimitRequestSize(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
