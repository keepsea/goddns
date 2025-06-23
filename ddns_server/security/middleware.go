// ===================================================================================
// File: ddns-server/security/middleware.go
// Description: 提供HTTP中间件。目前包含 RateLimit（基于IP的速率限制）和 LimitRequestSize（请求体大小限制），用于抵御基本的DoS攻击。
// ===================================================================================
package security

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	clients           = make(map[string]time.Time)
	clientsMutex      = &sync.Mutex{}
	rateLimitDuration = 5 * time.Second
)

// getClientIP 优先从代理头中获取真实客户端IP地址。
func getClientIP(r *http.Request) string {
	// 尝试从 "X-Forwarded-For" 头获取IP。这个头可能包含一个IP列表 (client, proxy1, proxy2)，第一个通常是真实的客户端IP。
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 按逗号分割，并取第一个非空IP
		ips := strings.Split(xff, ",")
		for _, ip := range ips {
			trimmedIP := strings.TrimSpace(ip)
			// 校验一下是否是合法的IP地址
			if net.ParseIP(trimmedIP) != nil {
				return trimmedIP
			}
		}
	}

	// 如果 "X-Forwarded-For" 不可用或格式不正确，尝试 "X-Real-IP"
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		if net.ParseIP(xrip) != nil {
			return xrip
		}
	}

	// 如果以上代理头都不可用，则使用直连的IP地址
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// 如果 SplitHostPort 失败 (例如 RemoteAddr 不含端口)，直接返回 RemoteAddr 本身
		// 但也要确保它是个合法的IP
		if net.ParseIP(r.RemoteAddr) != nil {
			return r.RemoteAddr
		}
		// 如果都失败，返回空字符串，表示无法获取有效IP
		return ""
	}
	return ip
}

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
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
