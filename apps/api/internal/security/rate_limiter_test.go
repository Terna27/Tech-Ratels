package security

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "CF-Connecting-IP takes highest priority",
			headers: map[string]string{
				"CF-Connecting-IP": "102.89.44.12",
				"X-Forwarded-For":  "197.210.65.10, 10.0.0.1",
				"True-Client-IP":   "197.210.65.20",
			},
			remoteAddr: "127.0.0.1:45678",
			expectedIP: "102.89.44.12",
		},
		{
			name: "True-Client-IP takes second priority",
			headers: map[string]string{
				"True-Client-IP":  "105.112.98.5",
				"X-Forwarded-For": "197.210.65.10, 10.0.0.1",
			},
			remoteAddr: "127.0.0.1:45678",
			expectedIP: "105.112.98.5",
		},
		{
			name: "X-Forwarded-For first valid IP",
			headers: map[string]string{
				"X-Forwarded-For": "197.210.65.10, 10.0.0.1",
			},
			remoteAddr: "127.0.0.1:45678",
			expectedIP: "197.210.65.10",
		},
		{
			name: "RemoteAddr host fallback",
			headers: map[string]string{},
			remoteAddr: "192.168.1.100:54321",
			expectedIP: "192.168.1.100",
		},
		{
			name: "IPv6 parsing in CF-Connecting-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "2001:db8::1",
			},
			remoteAddr: "127.0.0.1:45678",
			expectedIP: "2001:db8::1",
		},
		{
			name: "Invalid IP falls back to RemoteAddr",
			headers: map[string]string{
				"CF-Connecting-IP": "not-an-ip",
				"X-Forwarded-For":  "also-bad-ip",
			},
			remoteAddr: "10.0.0.5:12345",
			expectedIP: "10.0.0.5",
		},
		{
			name: "All missing or invalid returns unknown",
			headers: map[string]string{
				"CF-Connecting-IP": "invalid",
			},
			remoteAddr: "invalid-remote",
			expectedIP: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tc.remoteAddr

			got := GetClientIP(req)
			if got != tc.expectedIP {
				t.Errorf("GetClientIP() = %q; want %q", got, tc.expectedIP)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"report_2026_cleanups.pdf", "report_2026_cleanups.pdf"},
		{"../../etc/passwd", "passwd"},
		{"report\"injection\r\n.pdf", "report_injection__.pdf"},
		{"special spaces & chars!?.csv", "special_spaces___chars__.csv"},
		{"", "report_export"},
		{".", "report_export"},
		{"..", "report_export"},
	}

	for _, tc := range tests {
		got := SanitizeFilename(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeFilename(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDownloadRateLimiter(t *testing.T) {
	limiter := NewDownloadRateLimiter(3, 100*time.Millisecond)
	ip := "192.168.1.50"

	// 1st request -> allowed
	allowed, _ := limiter.Allow(ip)
	if !allowed {
		t.Errorf("expected 1st request to be allowed")
	}

	// 2nd request -> allowed
	allowed, _ = limiter.Allow(ip)
	if !allowed {
		t.Errorf("expected 2nd request to be allowed")
	}

	// 3rd request -> allowed
	allowed, _ = limiter.Allow(ip)
	if !allowed {
		t.Errorf("expected 3rd request to be allowed")
	}

	// 4th request -> blocked
	allowed, retryAfter := limiter.Allow(ip)
	if allowed {
		t.Errorf("expected 4th request to be blocked by rate limit")
	}
	if retryAfter <= 0 {
		t.Errorf("expected positive retryAfter duration, got %v", retryAfter)
	}

	// Other IP is unaffected
	otherIP := "192.168.1.51"
	allowed, _ = limiter.Allow(otherIP)
	if !allowed {
		t.Errorf("expected other IP to be allowed")
	}

	// After window expires, original IP is allowed again
	time.Sleep(120 * time.Millisecond)
	allowed, _ = limiter.Allow(ip)
	if !allowed {
		t.Errorf("expected request to be allowed after window expiration")
	}
}
