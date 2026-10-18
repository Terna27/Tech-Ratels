package delivery

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

var ErrProductionDeliveryNotConfigured = errors.New("live SMS/email gateway is not connected in this phase; contact verification must be configured via verified external gateway in production")

type DeliveryAdapter interface {
	SendVerificationCode(identifier, code string) error
	GetLastCode(identifier string) string
}

type DevSafeDeliveryAdapter struct {
	mu           sync.RWMutex
	lastCodes    map[string]string
	isProduction bool
}

func NewDevSafeDeliveryAdapter(isProduction bool) *DevSafeDeliveryAdapter {
	return &DevSafeDeliveryAdapter{
		lastCodes:    make(map[string]string),
		isProduction: isProduction,
	}
}

func (d *DevSafeDeliveryAdapter) SendVerificationCode(identifier, code string) error {
	if d.isProduction {
		// Strict production safeguard: Never log or retain OTP in memory or console
		return ErrProductionDeliveryNotConfigured
	}

	d.mu.Lock()
	d.lastCodes[identifier] = code
	d.mu.Unlock()

	log.Printf("[DEV_DELIVERY_SAFE_MODE] Contact verification code for %s is: %s (Development sandbox only; no external dispatch)", identifier, code)
	fmt.Printf("\n======================================================\n")
	fmt.Printf("[DEV SAFE DELIVERY] OTP Code for %s: %s\n", identifier, code)
	fmt.Printf("======================================================\n\n")
	return nil
}

func (d *DevSafeDeliveryAdapter) GetLastCode(identifier string) string {
	if d.isProduction {
		// Never leak in production
		return ""
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastCodes[identifier]
}
