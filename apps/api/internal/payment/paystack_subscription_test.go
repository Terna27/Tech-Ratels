package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaystackSubscriptionRefundReconciliation(t *testing.T) {
	for _, tt := range []struct {
		name, transaction, currency, status string
		httpStatus                          int
		wantError                           bool
		wantCount                           int
	}{{"processed", "42", "NGN", "processed", 200, false, 1}, {"pending", "42", "NGN", "pending", 200, false, 1}, {"wrong transaction", "43", "NGN", "processed", 200, true, 0}, {"wrong currency", "42", "USD", "processed", 200, true, 0}, {"upstream failure", "42", "NGN", "processed", 503, true, 0}} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("transaction") != "42" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("incorrect refund request")
				}
				w.WriteHeader(tt.httpStatus)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "data": []map[string]interface{}{{"id": 1, "transaction": json.Number(tt.transaction), "currency": tt.currency, "amount": 20000, "status": tt.status}}})
			}))
			defer srv.Close()
			g := NewPaystackGateway("test-key", "")
			g.SetBaseURL(srv.URL)
			rows, err := g.ListTransactionRefunds(context.Background(), "42")
			if (err != nil) != tt.wantError || len(rows) != tt.wantCount {
				t.Fatalf("rows %v err %v", rows, err)
			}
			if !tt.wantError && tt.status == "processed" && rows[0].Status != "successful" {
				t.Fatal(rows)
			}
		})
	}
}
