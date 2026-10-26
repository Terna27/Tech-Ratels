package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaystackGateway_VerifyPayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/transaction/verify/REF_TEST_123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Verification successful",
			"data": {
				"status": "success",
				"reference": "REF_TEST_123",
				"amount": 250000,
				"currency": "NGN",
				"paid_at": "2026-10-26T12:00:00Z",
				"channel": "card",
				"gateway_response": "Successful",
				"customer": {
					"email": "buyer@example.com"
				}
			}
		}`))
	}))
	defer ts.Close()

	gw := NewPaystackGateway("sk_test_secret", "pk_test_public")
	gw.SetBaseURL(ts.URL)

	out, err := gw.VerifyPayment(context.Background(), "REF_TEST_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "successful" {
		t.Errorf("expected status 'successful', got %s", out.Status)
	}
	if out.PaymentReference != "REF_TEST_123" {
		t.Errorf("expected reference 'REF_TEST_123', got %s", out.PaymentReference)
	}
	if out.AmountKobo != 250000 {
		t.Errorf("expected amount 250000, got %d", out.AmountKobo)
	}
	if out.Currency != "NGN" {
		t.Errorf("expected currency 'NGN', got %s", out.Currency)
	}
	if out.CustomerEmail != "buyer@example.com" {
		t.Errorf("expected email 'buyer@example.com', got %s", out.CustomerEmail)
	}
}

func TestPaystackGateway_VerifyPayment_HTTPErrorStatus(t *testing.T) {
	statuses := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError}
	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"status": false, "message": "error occurred"}`))
			}))
			defer ts.Close()

			gw := NewPaystackGateway("sk_test", "pk_test")
			gw.SetBaseURL(ts.URL)

			_, err := gw.VerifyPayment(context.Background(), "REF_FAIL")
			if err == nil {
				t.Fatalf("expected error for HTTP status %d, got nil", status)
			}
		})
	}
}

func TestPaystackGateway_VerifyPayment_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": true, "data": { broken json...`))
	}))
	defer ts.Close()

	gw := NewPaystackGateway("sk_test", "pk_test")
	gw.SetBaseURL(ts.URL)

	_, err := gw.VerifyPayment(context.Background(), "REF_BROKEN")
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("expected ErrMalformedResponse, got %v", err)
	}
}

func TestPaystackGateway_VerifyPayment_IncompleteResponses(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing reference",
			json: `{"status":true,"data":{"status":"success","reference":"","amount":1000,"currency":"NGN"}}`,
		},
		{
			name: "zero amount",
			json: `{"status":true,"data":{"status":"success","reference":"REF1","amount":0,"currency":"NGN"}}`,
		},
		{
			name: "negative amount",
			json: `{"status":true,"data":{"status":"success","reference":"REF1","amount":-500,"currency":"NGN"}}`,
		},
		{
			name: "missing currency",
			json: `{"status":true,"data":{"status":"success","reference":"REF1","amount":1000,"currency":""}}`,
		},
		{
			name: "missing status",
			json: `{"status":true,"data":{"status":"","reference":"REF1","amount":1000,"currency":"NGN"}}`,
		},
		{
			name: "status false response",
			json: `{"status":false,"message":"Transaction not found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.json))
			}))
			defer ts.Close()

			gw := NewPaystackGateway("sk_test", "pk_test")
			gw.SetBaseURL(ts.URL)

			_, err := gw.VerifyPayment(context.Background(), "REF_TEST")
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPaystackGateway_VerifyPayment_Unconfigured(t *testing.T) {
	gw := NewPaystackGateway("", "")
	_, err := gw.VerifyPayment(context.Background(), "REF1")
	if !errors.Is(err, ErrUnconfiguredProvider) {
		t.Fatalf("expected ErrUnconfiguredProvider, got %v", err)
	}
}

func TestPaystackGateway_VerifyWebhookSignature(t *testing.T) {
	secret := "sk_test_secret_key_123"
	payload := []byte(`{"event":"charge.success","data":{"reference":"REF_WH_1"}}`)

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(payload)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	gw := NewPaystackGateway(secret, "pk_test")

	// Valid signature
	if !gw.VerifyWebhookSignature(validSignature, payload) {
		t.Fatal("expected valid signature to verify successfully")
	}

	// Tampered payload
	tamperedPayload := []byte(`{"event":"charge.success","data":{"reference":"REF_WH_2"}}`)
	if gw.VerifyWebhookSignature(validSignature, tamperedPayload) {
		t.Fatal("expected tampered payload signature verification to fail")
	}

	// Invalid signature
	if gw.VerifyWebhookSignature("invalid_hex_or_signature", payload) {
		t.Fatal("expected invalid signature to fail")
	}

	// Empty signature
	if gw.VerifyWebhookSignature("", payload) {
		t.Fatal("expected empty signature to fail")
	}

	// Empty secret in gateway
	unconf := NewPaystackGateway("", "")
	if unconf.VerifyWebhookSignature(validSignature, payload) {
		t.Fatal("expected verification to fail when secret is empty")
	}
}
