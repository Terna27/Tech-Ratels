package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTermiiDeliveryAdapter_SendSMS_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		var req termiiSendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.To != "2348031234567" {
			t.Errorf("expected recipient 2348031234567, got %s", req.To)
		}
		if req.From != "RATELS" {
			t.Errorf("expected from RATELS, got %s", req.From)
		}
		if req.APIKey != "test-api-key" {
			t.Errorf("expected test-api-key, got %s", req.APIKey)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(termiiSuccessResponse{
			MessageID: "msg-termii-12345",
			Message:   "Successfully Sent",
			Balance:   150.0,
		})
	}))
	defer server.Close()

	adapter := NewTermiiDeliveryAdapter("test-api-key", "RATELS").WithBaseURL(server.URL)
	msgID, err := adapter.SendSMS(context.Background(), "08031234567", "Your cleanup reminder for Abuja Central")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgID != "msg-termii-12345" {
		t.Errorf("expected message ID msg-termii-12345, got %s", msgID)
	}
}

func TestTermiiDeliveryAdapter_Unconfigured(t *testing.T) {
	adapter := NewTermiiDeliveryAdapter("", "")
	_, err := adapter.SendSMS(context.Background(), "08031234567", "Test message")
	if err != ErrTermiiNotConfigured {
		t.Fatalf("expected ErrTermiiNotConfigured, got %v", err)
	}
}

func TestTermiiDeliveryAdapter_InvalidPhone(t *testing.T) {
	adapter := NewTermiiDeliveryAdapter("key", "RATELS")
	_, err := adapter.SendSMS(context.Background(), "invalid-phone", "Test message")
	if err == nil || !strings.Contains(err.Error(), "invalid Nigerian phone number") {
		t.Fatalf("expected invalid Nigerian phone number error, got %v", err)
	}
}

func TestTermiiDeliveryAdapter_ProviderErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		respBody    string
		errContains string
	}{
		{
			name:        "401 Unauthorized",
			status:      http.StatusUnauthorized,
			respBody:    `{"message":"Invalid API key"}`,
			errContains: "termii authentication failed",
		},
		{
			name:        "400 Bad Request",
			status:      http.StatusBadRequest,
			respBody:    `{"message":"Sender ID not approved"}`,
			errContains: "termii rejected parameter",
		},
		{
			name:        "429 Rate Limit",
			status:      http.StatusTooManyRequests,
			respBody:    `{"message":"Too many requests"}`,
			errContains: "termii rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.respBody))
			}))
			defer server.Close()

			adapter := NewTermiiDeliveryAdapter("secret-key", "RATELS").WithBaseURL(server.URL)
			_, err := adapter.SendSMS(context.Background(), "08031234567", "Test body")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
			}
			// Secret must be redacted
			if strings.Contains(err.Error(), "secret-key") {
				t.Errorf("secret key leaked in error message: %s", err.Error())
			}
		})
	}
}

func TestDevSafeSMSSender(t *testing.T) {
	dev := NewDevSafeSMSSender()
	msgID, err := dev.SendSMS(context.Background(), "08031234567", "Dev cleanup reminder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(msgID, "dev-sms-") {
		t.Errorf("expected dev-sms- prefix, got %s", msgID)
	}

	sent := dev.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].Phone != "2348031234567" {
		t.Errorf("expected 2348031234567, got %s", sent[0].Phone)
	}
}

func TestUnconfiguredSMSSender(t *testing.T) {
	unconf := NewUnconfiguredSMSSender()
	_, err := unconf.SendSMS(context.Background(), "08031234567", "Test")
	if err != ErrTermiiNotConfigured {
		t.Fatalf("expected ErrTermiiNotConfigured, got %v", err)
	}
}
