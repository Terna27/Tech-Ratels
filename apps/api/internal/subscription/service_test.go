package subscription

import (
	"errors"
	"ratels-api/internal/payment"
	"testing"
	"time"
)

func TestPrices(t *testing.T) {
	for _, tt := range []struct {
		plan, interval string
		amount         int64
	}{{"basic", "monthly", 200000}, {"basic", "yearly", 2000000}, {"business", "monthly", 500000}, {"business", "yearly", 5000000}} {
		got, err := Price(tt.plan, tt.interval)
		if err != nil || got != tt.amount {
			t.Fatalf("%+v: %d %v", tt, got, err)
		}
	}
	if _, err := Price("free", "monthly"); err == nil {
		t.Fatal("invalid plan accepted")
	}
	if _, err := Price("basic", "daily"); err == nil {
		t.Fatal("invalid interval accepted")
	}
}
func TestCalendarPeriods(t *testing.T) {
	for _, tt := range []struct{ start, interval, end string }{{"2027-01-31T12:15:00Z", "monthly", "2027-02-28T12:15:00Z"}, {"2028-01-31T12:15:00Z", "monthly", "2028-02-29T12:15:00Z"}, {"2028-02-29T12:15:00Z", "yearly", "2029-02-28T12:15:00Z"}, {"2026-12-31T12:15:00Z", "monthly", "2027-01-31T12:15:00Z"}} {
		start, _ := time.Parse(time.RFC3339, tt.start)
		if got := PeriodEnd(start, tt.interval).Format(time.RFC3339); got != tt.end {
			t.Errorf("%+v got %s", tt, got)
		}
	}
}
func TestPaymentValidation(t *testing.T) {
	p := Payment{Reference: "sub_test", Amount: 200000}
	for _, tt := range []struct {
		name string
		v    payment.VerifyPaymentOutput
		err  error
	}{{"valid", payment.VerifyPaymentOutput{PaymentReference: p.Reference, AmountKobo: p.Amount, Currency: "NGN", Status: "successful"}, nil}, {"amount", payment.VerifyPaymentOutput{PaymentReference: p.Reference, AmountKobo: 1, Currency: "NGN", Status: "successful"}, payment.ErrAmountMismatch}, {"currency", payment.VerifyPaymentOutput{PaymentReference: p.Reference, AmountKobo: p.Amount, Currency: "USD", Status: "successful"}, payment.ErrCurrencyMismatch}, {"reference", payment.VerifyPaymentOutput{PaymentReference: "order", AmountKobo: p.Amount, Currency: "NGN", Status: "successful"}, payment.ErrReferenceMismatch}, {"unknown status", payment.VerifyPaymentOutput{PaymentReference: p.Reference, AmountKobo: p.Amount, Currency: "NGN", Status: "approved"}, payment.ErrMalformedResponse}} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePayment(p, &tt.v); !errors.Is(err, tt.err) {
				t.Fatalf("got %v expected %v", err, tt.err)
			}
		})
	}
}

func TestLivePaymentSafeguards(t *testing.T) {
	for _, tt := range []struct {
		key          string
		allow, ready bool
	}{{"", false, false}, {"invalid", true, false}, {"sk_test_example", false, true}, {"sk_live_example", false, false}, {"sk_live_example", true, true}} {
		if got := CheckoutKeyAllowed(tt.key, tt.allow); got != tt.ready {
			t.Fatalf("allowLive=%v got %v want %v", tt.allow, got, tt.ready)
		}
	}
	t.Setenv("STORE_SUBSCRIPTIONS_ENABLED", "")
	t.Setenv("STORE_SUBSCRIPTIONS_PAYSTACK_SECRET_KEY", "sk_test_example")
	t.Setenv("STORE_SUBSCRIPTIONS_PAYSTACK_TEST_SECRET_KEY", "")
	t.Setenv("STORE_SUBSCRIPTIONS_ALLOW_LIVE_PAYMENTS", "")
	s := FromEnvironment(nil, nil, "https://ratels.test")
	if s.Enabled || s.CheckoutEnabled {
		t.Fatal("subscriptions enabled by default")
	}
	t.Setenv("STORE_SUBSCRIPTIONS_ENABLED", "true")
	t.Setenv("STORE_SUBSCRIPTIONS_PAYSTACK_SECRET_KEY", "sk_live_example")
	s = FromEnvironment(nil, nil, "https://ratels.test")
	if s.CheckoutEnabled {
		t.Fatal("live key enabled without explicit opt-in")
	}
}
