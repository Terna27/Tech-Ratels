package delivery

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid Nigerian phone number; must be 11 digits starting with 070, 080, 081, 090, 091, or international +234 format")
)

var (
	cleanRegex = regexp.MustCompile(`[\s\-\(\)\.]`)
	// Valid Nigerian mobile prefixes: 70, 80, 81, 90, 91
	nigerianMobileRegex = regexp.MustCompile(`^(?:\+?234|0)([789][01]\d{8})$`)
)

// NormalizeNigerianPhone validates and normalizes a phone number to standard E.164 (+234XXXXXXXXXX).
// Also returns the provider format without the plus sign (234XXXXXXXXXX).
func NormalizeNigerianPhone(raw string) (e164 string, providerFormat string, err error) {
	cleaned := cleanRegex.ReplaceAllString(strings.TrimSpace(raw), "")
	if cleaned == "" {
		return "", "", ErrInvalidPhoneNumber
	}

	matches := nigerianMobileRegex.FindStringSubmatch(cleaned)
	if len(matches) != 2 {
		return "", "", fmt.Errorf("%w: '%s'", ErrInvalidPhoneNumber, raw)
	}

	subscriberNumber := matches[1] // e.g. 8031234567 (10 digits)
	e164 = "+234" + subscriberNumber
	providerFormat = "234" + subscriberNumber

	return e164, providerFormat, nil
}
