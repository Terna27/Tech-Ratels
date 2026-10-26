package security

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	// uriCredentialsRegex matches URLs with username:password@
	// e.g. postgres://user:password@host:port/dbname
	uriCredentialsRegex = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:\s@/]+):([^@\s/]+)(@)`)

	// kvCredentialsRegex matches key-value connection parameters or config keys
	// e.g. password=secret, pwd=secret, secret=secret, token=secret, api_key=secret, access_key_id=secret, secret_access_key=secret
	kvCredentialsRegex = regexp.MustCompile(`(?i)\b(password|pwd|secret|token|api_key|apikey|api-key|brevo_api_key|resend_api_key|x-sib-api-key|access_key_id|secret_access_key|r2_access_key_id|r2_secret_access_key|r2_reports_access_key_id|r2_reports_secret_access_key)\s*[:=]\s*['"]?([^\s'";]+)['"]?`)

	// urlTokenRegex redacts raw verification tokens in URLs or query strings
	urlTokenRegex = regexp.MustCompile(`(?i)([?&]token=)[a-zA-Z0-9_\-]+`)

	// fullVerificationLinkRegex redacts entire verification links from logs
	fullVerificationLinkRegex = regexp.MustCompile(`https?://[^\s/]+/verify-email\?token=[a-zA-Z0-9_\-]+`)
)

// RedactString sanitizes a string to remove database passwords, tokens, and URIs containing credentials.
func RedactString(s string, knownSecrets ...string) string {
	if s == "" {
		return ""
	}

	// 1. Redact URI userInfo credentials
	redacted := uriCredentialsRegex.ReplaceAllString(s, "${1}:[REDACTED]${3}")

	// 2. Redact key-value pairs
	redacted = kvCredentialsRegex.ReplaceAllString(redacted, "${1}=[REDACTED]")

	// 3. Redact URL token query params
	redacted = urlTokenRegex.ReplaceAllString(redacted, "${1}[REDACTED]")

	// 4. Redact full verification URLs if present
	redacted = fullVerificationLinkRegex.ReplaceAllString(redacted, "[REDACTED_VERIFICATION_LINK]")

	// 3. Redact any known secrets if provided
	for _, sec := range knownSecrets {
		trimmed := strings.TrimSpace(sec)
		if trimmed == "" {
			continue
		}
		// If known secret is a full database URL, extract user password
		if u, err := url.Parse(trimmed); err == nil && u.User != nil {
			if pass, ok := u.User.Password(); ok && pass != "" {
				redacted = strings.ReplaceAll(redacted, pass, "[REDACTED]")
			}
		}
		// Also replace the literal secret if >= 6 chars to avoid replacing short common words
		if len(trimmed) >= 6 {
			redacted = strings.ReplaceAll(redacted, trimmed, "[REDACTED]")
		}
	}

	return redacted
}

// RedactError returns a new error where any embedded credentials or sensitive connection strings are sanitized.
func RedactError(err error, knownSecrets ...string) error {
	if err == nil {
		return nil
	}
	return errors.New(RedactString(err.Error(), knownSecrets...))
}
