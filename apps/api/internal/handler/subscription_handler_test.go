package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/subscription"
	"strings"
	"testing"
)

func subscriptionRouter() *chi.Mux {
	h := &SubscriptionHandler{Service: subscription.New(nil, payment.NewPaystackGateway("sk_test_fixture", ""), nil, false, false, "https://ratels.test")}
	r := chi.NewRouter()
	r.Use(CSRFProtectionMiddleware)
	r.Route("/api/v1/marketplace", h.Routes)
	return r
}
func TestSubscriptionRoutesAuthenticationAndCSRF(t *testing.T) {
	for _, tt := range []struct {
		name, method, path, role, auth string
		want                           int
	}{{"plans", "GET", "/subscriptions/plans", "", "", 200}, {"checkout auth", "POST", "/seller/businesses/store/subscription/checkout", "", "", 401}, {"admin report", "GET", "/admin/subscriptions/", "member", "", 403}, {"admin reconciliation", "POST", "/admin/subscriptions/payments/sub_test/reconcile", "member", "", 403}, {"checkout CSRF", "POST", "/seller/businesses/store/subscription/checkout", "member", "cookie", 403}} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "/api/v1/marketplace"+tt.path, strings.NewReader(`{}`))
			ctx := r.Context()
			if tt.role != "" {
				ctx = context.WithValue(ctx, UserContextKey, db.User{ID: "test", PrimaryRole: tt.role})
			}
			ctx = context.WithValue(ctx, AuthTypeContextKey, tt.auth)
			w := httptest.NewRecorder()
			subscriptionRouter().ServeHTTP(w, r.WithContext(ctx))
			if w.Code != tt.want {
				t.Fatalf("got %d want %d %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}
func TestSubscriptionWebhookHMACAndCSRFIndependence(t *testing.T) {
	for _, valid := range []bool{false, true} {
		body := `{"event":"charge.success","data":{"reference":"order_reference"}}`
		r := httptest.NewRequest("POST", "/api/v1/marketplace/subscriptions/webhooks/paystack", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), AuthTypeContextKey, "cookie"))
		signature := "bad"
		if valid {
			h := hmac.New(sha512.New, []byte("sk_test_fixture"))
			h.Write([]byte(body))
			signature = hex.EncodeToString(h.Sum(nil))
		}
		r.Header.Set("x-paystack-signature", signature)
		w := httptest.NewRecorder()
		subscriptionRouter().ServeHTTP(w, r)
		want := 401
		if valid {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("valid=%v got %d want %d %s", valid, w.Code, want, w.Body.String())
		}
	}
}
