package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
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

func TestPaystackGateway_RefundPayment_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/refund" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Refund queued successfully",
			"data": {
				"status": "pending",
				"reference": "REFUND_PAYSTACK_123",
				"refund_reference": "REFUND_PAYSTACK_123",
				"amount": 50000,
				"currency": "NGN"
			}
		}`))
	}))
	defer ts.Close()

	gw := NewPaystackGateway("sk_test_secret", "pk_test_public")
	gw.SetBaseURL(ts.URL)

	out, err := gw.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "ORIG_PAY_123",
		AmountKobo:       50000,
		Currency:         "NGN",
		MerchantNote:     "Damaged product refund",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.RefundReference != "REFUND_PAYSTACK_123" {
		t.Errorf("expected refund reference 'REFUND_PAYSTACK_123', got %s", out.RefundReference)
	}
	if out.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", out.Status)
	}
	if out.AmountKobo != 50000 {
		t.Errorf("expected amount 50000, got %d", out.AmountKobo)
	}
	if out.Currency != "NGN" {
		t.Errorf("expected currency 'NGN', got %s", out.Currency)
	}
}

func TestPaystackGateway_RefundPayment_HTTPErrorStatus(t *testing.T) {
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

			_, err := gw.RefundPayment(context.Background(), RefundPaymentInput{
				PaymentReference: "ORIG_PAY_123",
				AmountKobo:       50000,
				Currency:         "NGN",
			})
			if err == nil {
				t.Fatalf("expected error for HTTP status %d, got nil", status)
			}
		})
	}
}

func TestPaystackGateway_RefundPayment_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": true, "data": { broken json...`))
	}))
	defer ts.Close()

	gw := NewPaystackGateway("sk_test", "pk_test")
	gw.SetBaseURL(ts.URL)

	_, err := gw.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "ORIG_PAY_123",
		AmountKobo:       50000,
		Currency:         "NGN",
	})
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("expected ErrMalformedResponse, got %v", err)
	}
}

func TestPaystackGateway_RefundPayment_ValidationAndUnconfigured(t *testing.T) {
	gwUnconf := NewPaystackGateway("", "")
	_, err := gwUnconf.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "REF1",
		AmountKobo:       1000,
	})
	if !errors.Is(err, ErrUnconfiguredProvider) {
		t.Fatalf("expected ErrUnconfiguredProvider, got %v", err)
	}

	gw := NewPaystackGateway("sk_test", "pk_test")

	// Missing reference
	_, err = gw.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "",
		AmountKobo:       1000,
	})
	if err == nil {
		t.Fatal("expected error for empty payment reference, got nil")
	}

	// Zero or negative amount
	_, err = gw.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "REF1",
		AmountKobo:       0,
	})
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}

	_, err = gw.RefundPayment(context.Background(), RefundPaymentInput{
		PaymentReference: "REF1",
		AmountKobo:       -500,
	})
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

func TestPaystackGateway_VerifyRefund_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/refund/REFUND_123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Refund retrieved",
			"data": {
				"status": "processed",
				"reference": "REFUND_123",
				"refund_reference": "REFUND_123",
				"transaction_reference": "TXN_ORIG_456",
				"amount": 75000,
				"currency": "NGN"
			}
		}`))
	}))
	defer ts.Close()

	gw := NewPaystackGateway("sk_test_secret", "pk_test")
	gw.SetBaseURL(ts.URL)

	out, err := gw.VerifyRefund(context.Background(), "REFUND_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "successful" {
		t.Errorf("expected status 'successful', got %s", out.Status)
	}
	if out.RefundReference != "REFUND_123" {
		t.Errorf("expected refund ref 'REFUND_123', got %s", out.RefundReference)
	}
	if out.TransactionReference != "TXN_ORIG_456" {
		t.Errorf("expected txn ref 'TXN_ORIG_456', got %s", out.TransactionReference)
	}
	if out.AmountKobo != 75000 {
		t.Errorf("expected amount 75000, got %d", out.AmountKobo)
	}
	if out.Currency != "NGN" {
		t.Errorf("expected currency 'NGN', got %s", out.Currency)
	}
}

func TestPaystackGateway_VerifyRefund_PendingAndFailed(t *testing.T) {
	tests := []struct {
		name           string
		providerStatus string
		expectedStatus string
	}{
		{name: "pending", providerStatus: "pending", expectedStatus: "pending"},
		{name: "processing", providerStatus: "processing", expectedStatus: "pending"},
		{name: "failed", providerStatus: "failed", expectedStatus: "failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(fmt.Sprintf(`{
					"status": true,
					"message": "Refund status",
					"data": {
						"status": "%s",
						"reference": "REF_TEST",
						"amount": 5000
					}
				}`, tc.providerStatus)))
			}))
			defer ts.Close()

			gw := NewPaystackGateway("sk_test", "pk_test")
			gw.SetBaseURL(ts.URL)

			out, err := gw.VerifyRefund(context.Background(), "REF_TEST")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Status != tc.expectedStatus {
				t.Errorf("expected status '%s', got '%s'", tc.expectedStatus, out.Status)
			}
		})
	}
}

func TestPaystackGateway_VerifyRefund_ErrorsAndValidation(t *testing.T) {
	// Unconfigured
	unconf := NewPaystackGateway("", "")
	_, err := unconf.VerifyRefund(context.Background(), "REF1")
	if !errors.Is(err, ErrUnconfiguredProvider) {
		t.Fatalf("expected ErrUnconfiguredProvider, got %v", err)
	}

	// Empty reference
	gw := NewPaystackGateway("sk_test", "pk_test")
	_, err = gw.VerifyRefund(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty reference, got nil")
	}

	// HTTP 404
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status": false, "message": "Refund not found"}`))
	}))
	defer ts.Close()

	gw.SetBaseURL(ts.URL)
	_, err = gw.VerifyRefund(context.Background(), "REF_NOT_FOUND")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}
