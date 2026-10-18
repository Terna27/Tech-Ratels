package delivery_test

import (
	"testing"

	"ratels-api/internal/delivery"
)

func TestDevSafeDeliveryAdapter(t *testing.T) {
	t.Run("Development mode stores code and succeeds", func(t *testing.T) {
		dev := delivery.NewDevSafeDeliveryAdapter(false)
		err := dev.SendVerificationCode("user@example.org", "123456")
		if err != nil {
			t.Fatalf("Expected nil error in dev mode, got: %v", err)
		}

		code := dev.GetLastCode("user@example.org")
		if code != "123456" {
			t.Errorf("Expected code '123456', got: '%s'", code)
		}
	})

	t.Run("Production mode rejects delivery and refuses in-memory storage", func(t *testing.T) {
		prod := delivery.NewDevSafeDeliveryAdapter(true)
		err := prod.SendVerificationCode("user@example.org", "123456")
		if err != delivery.ErrProductionDeliveryNotConfigured {
			t.Fatalf("Expected ErrProductionDeliveryNotConfigured in prod, got: %v", err)
		}

		code := prod.GetLastCode("user@example.org")
		if code != "" {
			t.Errorf("Expected empty code in production mode, got: '%s'", code)
		}
	})
}
