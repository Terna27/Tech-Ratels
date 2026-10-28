package subscription

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"ratels-api/internal/delivery"
	"ratels-api/internal/payment"
	"strings"
)

// CheckoutKeyAllowed requires an explicit second opt-in for live charges. The
// rollout flag alone cannot turn on real-money checkout.
func CheckoutKeyAllowed(key string, allowLive bool) bool {
	return strings.HasPrefix(key, "sk_test_") || (allowLive && strings.HasPrefix(key, "sk_live_"))
}
func FromEnvironment(pool *pgxpool.Pool, email delivery.EmailSender, frontend string) *Service {
	enabled := os.Getenv("STORE_SUBSCRIPTIONS_ENABLED") == "true"
	key := strings.TrimSpace(os.Getenv("STORE_SUBSCRIPTIONS_PAYSTACK_SECRET_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("STORE_SUBSCRIPTIONS_PAYSTACK_TEST_SECRET_KEY"))
	}
	allowed := CheckoutKeyAllowed(key, os.Getenv("STORE_SUBSCRIPTIONS_ALLOW_LIVE_PAYMENTS") == "true")
	if !allowed {
		key = ""
	}
	return New(pool, payment.NewPaystackGateway(key, ""), email, enabled, enabled && allowed, frontend)
}
