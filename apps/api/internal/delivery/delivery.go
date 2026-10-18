package delivery

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

var (
	ErrProductionDeliveryNotConfigured = errors.New("live SMS/email gateway is not connected in this phase; contact verification must be configured via verified external gateway in production")
	ErrDevDeliveryRejected             = errors.New("delivery rejected in development sandbox")
)

type DeliveryAdapter interface {
	SendVerificationCode(identifier, code string) error
	SendVerificationEmail(recipientEmail, recipientName, verificationURL string) error
	GetLastCode(identifier string) string
	GetLastVerificationURL(identifier string) string
}

type DevSafeDeliveryAdapter struct {
	mu           sync.RWMutex
	lastCodes    map[string]string
	lastURLs     map[string]string
	isProduction bool
	SimulateFail bool
}

func NewDevSafeDeliveryAdapter(isProduction bool) *DevSafeDeliveryAdapter {
	return &DevSafeDeliveryAdapter{
		lastCodes:    make(map[string]string),
		lastURLs:     make(map[string]string),
		isProduction: isProduction,
	}
}

func (d *DevSafeDeliveryAdapter) SendVerificationCode(identifier, code string) error {
	if d.isProduction {
		// Strict production safeguard: Never log or retain OTP in memory or console
		return ErrProductionDeliveryNotConfigured
	}
	if d.SimulateFail {
		return ErrDevDeliveryRejected
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

func (d *DevSafeDeliveryAdapter) SendVerificationEmail(recipientEmail, recipientName, verificationURL string) error {
	if d.isProduction {
		return ErrProductionDeliveryNotConfigured
	}
	if d.SimulateFail {
		return ErrDevDeliveryRejected
	}

	d.mu.Lock()
	d.lastURLs[recipientEmail] = verificationURL
	d.mu.Unlock()

	log.Printf("[DEV_DELIVERY_SAFE_MODE] Verification email for %s: Link dispatched (Development sandbox only)", recipientEmail)
	fmt.Printf("\n======================================================\n")
	fmt.Printf("[DEV SAFE DELIVERY] Verification Email for %s (%s)\n", recipientEmail, recipientName)
	fmt.Printf("Verification URL: %s\n", verificationURL)
	fmt.Printf("======================================================\n\n")
	return nil
}

func (d *DevSafeDeliveryAdapter) GetLastCode(identifier string) string {
	if d.isProduction {
		return ""
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastCodes[identifier]
}

func (d *DevSafeDeliveryAdapter) GetLastVerificationURL(identifier string) string {
	if d.isProduction {
		return ""
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastURLs[identifier]
}

type ProductionUnconfiguredAdapter struct{}

func NewProductionUnconfiguredAdapter() *ProductionUnconfiguredAdapter {
	return &ProductionUnconfiguredAdapter{}
}

func (p *ProductionUnconfiguredAdapter) SendVerificationCode(identifier, code string) error {
	return ErrProductionDeliveryNotConfigured
}

func (p *ProductionUnconfiguredAdapter) SendVerificationEmail(recipientEmail, recipientName, verificationURL string) error {
	return ErrProductionDeliveryNotConfigured
}

func (p *ProductionUnconfiguredAdapter) GetLastCode(identifier string) string {
	return ""
}

func (p *ProductionUnconfiguredAdapter) GetLastVerificationURL(identifier string) string {
	return ""
}
