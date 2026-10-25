package delivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/delivery"
)

func TestBrevoDeliveryAdapter_SendVerificationEmail(t *testing.T) {
	testKey := "xkeysib-supersecretbrevoapikey1234567890abcdef"
	fromName := "Ratels Worldwide"
	fromEmail := "verified@ratels.org.ng"

	t.Run("Success 201 Created sends correct headers and body payload", func(t *testing.T) {
		var receivedReq struct {
			Sender struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"sender"`
			To []struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"to"`
			Subject     string `json:"subject"`
			HTMLContent string `json:"htmlContent"`
			TextContent string `json:"textContent"`
		}
		var receivedAPIKey string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAPIKey = r.Header.Get("api-key")
			if r.Header.Get("content-type") != "application/json" {
				t.Errorf("expected application/json Content-Type, got %s", r.Header.Get("content-type"))
			}

			if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"messageId":"<20261018.123456@smtp-relay.brevo.com>"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)

		err := adapter.SendVerificationEmail("member@example.com", "Member Name", "https://ratels-web.onrender.com/verify-email?token=abcdef123456")
		if err != nil {
			t.Fatalf("expected nil error on HTTP 201, got: %v", err)
		}

		if receivedAPIKey != testKey {
			t.Errorf("expected api-key %q, got %q", testKey, receivedAPIKey)
		}
		if receivedReq.Sender.Email != fromEmail {
			t.Errorf("expected sender email %q, got %q", fromEmail, receivedReq.Sender.Email)
		}
		if receivedReq.Sender.Name != fromName {
			t.Errorf("expected sender name %q, got %q", fromName, receivedReq.Sender.Name)
		}
		if len(receivedReq.To) != 1 || receivedReq.To[0].Email != "member@example.com" {
			t.Errorf("expected recipient member@example.com, got %+v", receivedReq.To)
		}
		if !strings.Contains(receivedReq.HTMLContent, "https://ratels-web.onrender.com/verify-email?token=abcdef123456") {
			t.Error("expected verification URL inside HTML content")
		}
		if !strings.Contains(receivedReq.TextContent, "https://ratels-web.onrender.com/verify-email?token=abcdef123456") {
			t.Error("expected verification URL inside Text content")
		}
	})

	t.Run("HTTP 401 Unauthorized returns descriptive error without leaking key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"unauthorized","message":"Key not found"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected error on HTTP 401")
		}
		if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid, expired, or missing API key") {
			t.Errorf("unexpected error message: %v", err)
		}
		if strings.Contains(err.Error(), testKey) {
			t.Errorf("API key leaked in error: %v", err)
		}
	})

	t.Run("HTTP 403 Forbidden sender rejected or account restricted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"unauthorized_domain","message":"Sender domain is unverified"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected error on HTTP 403")
		}
		if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "unauthorized_domain") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("HTTP 429 Too Many Requests rate limit handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"too_many_requests","message":"Rate limit exceeded for transactional emails"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected error on HTTP 429")
		}
		if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "rate limit") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("HTTP 400 Bad Request parameter error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"invalid_parameter","message":"invalid recipient format"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected error on HTTP 400")
		}
		if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid_parameter") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("HTTP 500 Server Error handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"service_unavailable","message":"Internal Brevo error"}`))
		}))
		defer server.Close()

		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).WithBaseURL(server.URL)
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected error on HTTP 500")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Timeout handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Adapter with 10ms timeout client
		fastClient := &http.Client{Timeout: 10 * time.Millisecond}
		adapter := delivery.NewBrevoDeliveryAdapter(testKey, fromName, fromEmail).
			WithBaseURL(server.URL).
			WithHTTPClient(fastClient)

		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "time") {
			t.Errorf("expected timeout indicator in error, got: %v", err)
		}
	})

	t.Run("Unconfigured adapter returns ErrBrevoNotConfigured", func(t *testing.T) {
		adapter := delivery.NewBrevoDeliveryAdapter("", "", "")
		err := adapter.SendVerificationEmail("member@example.com", "Member", "https://ratels-web.onrender.com/verify-email?token=xyz")
		if err != delivery.ErrBrevoNotConfigured {
			t.Fatalf("expected ErrBrevoNotConfigured, got %v", err)
		}
	})

	t.Run("Context canceled", func(t *testing.T) {
		_ = context.Background()
	})
}
