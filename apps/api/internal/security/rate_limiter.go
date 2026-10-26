package security

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var safeFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeFilename strips directory components and replaces unsafe characters
// to prevent header injection and directory traversal.
func SanitizeFilename(name string) string {
	base := filepath.Base(name)
	cleaned := safeFilenameRegex.ReplaceAllString(base, "_")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "report_export"
	}
	return cleaned
}

// Trusted proxy configuration
var (
	trustedMu     sync.RWMutex
	trustedIPs    []net.IP
	trustedNets   []*net.IPNet
	trustedInitOnce sync.Once
)

// initDefaultTrustedProxies initializes loopback and environment-configured proxies.
func initDefaultTrustedProxies() {
	trustedIPs = []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}
	_, loopback4, _ := net.ParseCIDR("127.0.0.0/8")
	_, loopback6, _ := net.ParseCIDR("::1/128")
	trustedNets = []*net.IPNet{loopback4, loopback6}

	if envProxies := os.Getenv("TRUSTED_PROXIES"); envProxies != "" {
		for _, raw := range strings.Split(envProxies, ",") {
			item := strings.TrimSpace(raw)
			if item == "" {
				continue
			}
			if strings.Contains(item, "/") {
				if _, ipNet, err := net.ParseCIDR(item); err == nil && ipNet != nil {
					trustedNets = append(trustedNets, ipNet)
				}
			} else {
				if ip := net.ParseIP(item); ip != nil {
					trustedIPs = append(trustedIPs, ip)
				}
			}
		}
	}
}

// SetTrustedProxies allows programmatic configuration of trusted reverse proxy IPs and CIDRs.
func SetTrustedProxies(proxies []string) {
	trustedMu.Lock()
	defer trustedMu.Unlock()

	trustedIPs = nil
	trustedNets = nil

	for _, raw := range proxies {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, ipNet, err := net.ParseCIDR(item); err == nil && ipNet != nil {
				trustedNets = append(trustedNets, ipNet)
			}
		} else {
			if ip := net.ParseIP(item); ip != nil {
				trustedIPs = append(trustedIPs, ip)
			}
		}
	}
}

// ResetDefaultTrustedProxies restores the default loopback and TRUSTED_PROXIES configuration.
func ResetDefaultTrustedProxies() {
	trustedMu.Lock()
	defer trustedMu.Unlock()
	initDefaultTrustedProxies()
}

// IsTrustedProxy checks whether the given remote IP address matches a trusted reverse proxy.
func IsTrustedProxy(remoteIP string) bool {
	trustedInitOnce.Do(initDefaultTrustedProxies)

	ip := net.ParseIP(strings.TrimSpace(remoteIP))
	if ip == nil {
		return false
	}

	trustedMu.RLock()
	defer trustedMu.RUnlock()

	for _, tip := range trustedIPs {
		if tip.Equal(ip) {
			return true
		}
	}
	for _, tnet := range trustedNets {
		if tnet.Contains(ip) {
			return true
		}
	}
	return false
}

// GetClientIP securely extracts and validates the client's real IP address.
// Forwarding headers (CF-Connecting-IP, True-Client-IP, X-Forwarded-For) are trusted
// ONLY if the direct connection originates from a verified reverse proxy (e.g. Cloudflare,
// local loopback proxy, or addresses defined in TRUSTED_PROXIES).
// Direct connections from untrusted hosts strictly fall back to the remote TCP socket address.
func GetClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	trustedInitOnce.Do(initDefaultTrustedProxies)

	// Determine direct remote IP
	var directIP net.IP
	if r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			directIP = net.ParseIP(host)
		} else {
			directIP = net.ParseIP(r.RemoteAddr)
		}
	}

	// If RemoteAddr was present, evaluate trusted proxy status
	if directIP != nil {
		if !IsTrustedProxy(directIP.String()) {
			// Untrusted direct connection: strictly refuse client-provided forwarding headers!
			return directIP.String()
		}
	}

	// Connection originates from trusted proxy (or in-memory test request where RemoteAddr is empty):
	// Inspect edge forwarding headers in prioritized order.

	// 1. CF-Connecting-IP: Set by Cloudflare Edge proxy
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return ip.String()
		}
	}

	// 2. True-Client-IP: Akamai / Enterprise Cloudflare
	if tcIP := strings.TrimSpace(r.Header.Get("True-Client-IP")); tcIP != "" {
		if ip := net.ParseIP(tcIP); ip != nil {
			return ip.String()
		}
	}

	// 3. X-Forwarded-For: Client, Proxy1, Proxy2...
	// We extract the first valid IP from the left (the original client address)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if ip := net.ParseIP(candidate); ip != nil {
				return ip.String()
			}
		}
	}

	// 4. Fallback to direct TCP connection address
	if directIP != nil {
		return directIP.String()
	}

	return "unknown"
}

// DownloadRateLimiter provides a thread-safe sliding-window rate limiter per client IP.
type DownloadRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
	lastGC   time.Time
}

// NewDownloadRateLimiter initializes a new sliding-window rate limiter.
func NewDownloadRateLimiter(limit int, window time.Duration) *DownloadRateLimiter {
	if limit <= 0 {
		limit = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	return &DownloadRateLimiter{
		limit:    limit,
		window:   window,
		requests: make(map[string][]time.Time),
		lastGC:   time.Now(),
	}
}

// Allow evaluates whether an incoming request from the given IP address is permitted.
// If rejected, it returns false along with the duration until the rate limit clears.
func (l *DownloadRateLimiter) Allow(ip string) (bool, time.Duration) {
	if ip == "" {
		ip = "unknown"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Periodic garbage collection every 2 minutes
	if now.Sub(l.lastGC) > 2*time.Minute {
		for k, timestamps := range l.requests {
			var active []time.Time
			for _, t := range timestamps {
				if t.After(cutoff) {
					active = append(active, t)
				}
			}
			if len(active) == 0 {
				delete(l.requests, k)
			} else {
				l.requests[k] = active
			}
		}
		l.lastGC = now
	}

	timestamps := l.requests[ip]
	var active []time.Time
	for _, t := range timestamps {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}

	if len(active) >= l.limit {
		// Oldest request in the window dictates the retry-after duration
		oldest := active[0]
		retryAfter := oldest.Add(l.window).Sub(now)
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		l.requests[ip] = active
		return false, retryAfter
	}

	active = append(active, now)
	l.requests[ip] = active
	return true, 0
}

// Reset clears recorded request timestamps for an IP or all IPs.
func (l *DownloadRateLimiter) Reset(ip ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(ip) > 0 && ip[0] != "" {
		delete(l.requests, ip[0])
	} else {
		l.requests = make(map[string][]time.Time)
	}
}
