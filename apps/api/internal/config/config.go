package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultDevDatabaseURL = "postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable"
	DefaultDevJWTSecret   = "ratels-secret-key-dev-only-change-in-production-min-32-chars"
	MinJWTSecretLength    = 32
)

type Config struct {
	DatabaseURL               string
	Port                      string
	JWTSecret                 string
	JWTExpiryHours            int
	Environment               string
	DevSafeDelivery           bool
	FoundationProvisionalName string
	AllowedOrigins            []string
	EmailProvider             string // "brevo", "resend", "dev"
	BrevoAPIKey               string
	ResendAPIKey              string
	EmailFromName             string
	EmailFromAddress          string
	EmailFrom                 string
	FrontendBaseURL           string
	PaymentProvider           string // "paystack", "sandbox"
	PaystackSecretKey         string
	PaystackPublicKey         string
	R2AccountID               string
	R2AccessKeyID             string
	R2SecretAccessKey         string
	R2BucketName              string
	R2PublicDomain            string
	R2Endpoint                string
	TermiiAPIKey              string
	TermiiSenderID            string
	EnableOutboxWorker        bool
	SafeguardingReportingEnabled bool
	R2SafeguardingBucketName     string
	SafeguardingEncryptionKey    string
}

func Load() *Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	isProduction := strings.ToLower(env) == "production"

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" && !isProduction {
		dbURL = DefaultDevDatabaseURL
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" && !isProduction {
		jwtSecret = DefaultDevJWTSecret
	}

	jwtExpiryHours := 24
	if expStr := os.Getenv("JWT_EXPIRY_HOURS"); expStr != "" {
		if val, err := strconv.Atoi(expStr); err == nil && val > 0 {
			jwtExpiryHours = val
		}
	}

	devDelivery := true
	if isProduction || os.Getenv("DEV_DELIVERY_SAFE_MODE") == "false" {
		devDelivery = false
	}

	foundationName := os.Getenv("FOUNDATION_PROVISIONAL_NAME")
	if foundationName == "" {
		foundationName = "Martins Vincent Otse Foundation"
	}

	// AllowedOrigins: In production, allow ONLY explicitly configured frontend origins.
	// In development/test, default to local dev server origins.
	var allowedOrigins []string
	if isProduction {
		if rawOrigins := os.Getenv("ALLOWED_ORIGINS"); rawOrigins != "" {
			for _, o := range strings.Split(rawOrigins, ",") {
				trimmed := strings.TrimSpace(o)
				if trimmed != "" {
					allowedOrigins = append(allowedOrigins, trimmed)
				}
			}
		}
	} else {
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:8000",
			"http://127.0.0.1:8000",
		}
		if rawOrigins := os.Getenv("ALLOWED_ORIGINS"); rawOrigins != "" {
			for _, o := range strings.Split(rawOrigins, ",") {
				trimmed := strings.TrimSpace(o)
				if trimmed != "" {
					allowedOrigins = append(allowedOrigins, trimmed)
				}
			}
		}
	}

	emailProvider := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")))
	brevoAPIKey := strings.TrimSpace(os.Getenv("BREVO_API_KEY"))
	resendAPIKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))

	emailFromName := strings.TrimSpace(os.Getenv("EMAIL_FROM_NAME"))
	if emailFromName == "" {
		emailFromName = "Nigerian Ratels"
	}

	emailFromAddress := strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS"))
	emailFrom := strings.TrimSpace(os.Getenv("EMAIL_FROM"))

	if emailFromAddress != "" && emailFrom == "" {
		emailFrom = fmt.Sprintf("%s <%s>", emailFromName, emailFromAddress)
	} else if emailFrom != "" && emailFromAddress == "" {
		if strings.Contains(emailFrom, "<") && strings.Contains(emailFrom, ">") {
			parts := strings.Split(emailFrom, "<")
			emailFromAddress = strings.Trim(parts[1], "> \t")
			namePart := strings.TrimSpace(parts[0])
			if namePart != "" && emailFromName == "Nigerian Ratels" {
				emailFromName = namePart
			}
		} else {
			emailFromAddress = emailFrom
		}
	} else if emailFrom == "" && emailFromAddress == "" {
		emailFromAddress = "onboarding@resend.dev"
		emailFrom = fmt.Sprintf("%s <%s>", emailFromName, emailFromAddress)
	}

	if emailProvider == "" {
		if brevoAPIKey != "" {
			emailProvider = "brevo"
		} else if resendAPIKey != "" {
			emailProvider = "resend"
		} else if isProduction {
			emailProvider = "unconfigured"
		} else {
			emailProvider = "dev"
		}
	}

	frontendBaseURL := strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))
	if frontendBaseURL == "" {
		if isProduction && len(allowedOrigins) > 0 {
			frontendBaseURL = allowedOrigins[0]
		} else {
			frontendBaseURL = "http://localhost:3000"
		}
	}
	frontendBaseURL = strings.TrimRight(frontendBaseURL, "/")

	paymentProvider := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")))
	paystackSecretKey := strings.TrimSpace(os.Getenv("PAYSTACK_SECRET_KEY"))
	paystackPublicKey := strings.TrimSpace(os.Getenv("PAYSTACK_PUBLIC_KEY"))

	if paymentProvider == "" {
		if paystackSecretKey != "" {
			paymentProvider = "paystack"
		} else if isProduction {
			paymentProvider = "unconfigured"
		} else {
			paymentProvider = "sandbox"
		}
	}

	r2AccountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	r2AccessKeyID := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	r2SecretAccessKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	r2BucketName := strings.TrimSpace(os.Getenv("R2_BUCKET_NAME"))
	r2PublicDomain := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_DOMAIN")), "/")
	if r2PublicDomain == "" {
		r2PublicDomain = "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev"
	} else if !strings.HasPrefix(r2PublicDomain, "http://") && !strings.HasPrefix(r2PublicDomain, "https://") {
		r2PublicDomain = "https://" + r2PublicDomain
	}
	r2Endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_ENDPOINT")), "/")

	termiiAPIKey := strings.TrimSpace(os.Getenv("TERMII_API_KEY"))
	termiiSenderID := strings.TrimSpace(os.Getenv("TERMII_SENDER_ID"))
	if termiiSenderID == "" {
		termiiSenderID = "RATELS"
	}
	enableWorker := true
	if os.Getenv("ENABLE_OUTBOX_WORKER") == "false" {
		enableWorker = false
	}

	safeguardingEnabled := false
	if isProduction {
		if os.Getenv("SAFEGUARDING_REPORTING_ENABLED") == "true" {
			safeguardingEnabled = true
		}
	} else {
		// Enabled by default in dev/test unless explicitly disabled
		safeguardingEnabled = true
		if os.Getenv("SAFEGUARDING_REPORTING_ENABLED") == "false" {
			safeguardingEnabled = false
		}
	}

	r2SafeguardingBucket := strings.TrimSpace(os.Getenv("R2_SAFEGUARDING_BUCKET_NAME"))
	safeguardingEncKey := strings.TrimSpace(os.Getenv("SAFEGUARDING_ENCRYPTION_KEY"))

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		JWTSecret:                 jwtSecret,
		JWTExpiryHours:            jwtExpiryHours,
		Environment:               env,
		DevSafeDelivery:           devDelivery,
		FoundationProvisionalName: foundationName,
		AllowedOrigins:            allowedOrigins,
		EmailProvider:             emailProvider,
		BrevoAPIKey:               brevoAPIKey,
		ResendAPIKey:              resendAPIKey,
		EmailFromName:             emailFromName,
		EmailFromAddress:          emailFromAddress,
		EmailFrom:                 emailFrom,
		FrontendBaseURL:           frontendBaseURL,
		PaymentProvider:           paymentProvider,
		PaystackSecretKey:         paystackSecretKey,
		PaystackPublicKey:         paystackPublicKey,
		R2AccountID:               r2AccountID,
		R2AccessKeyID:             r2AccessKeyID,
		R2SecretAccessKey:         r2SecretAccessKey,
		R2BucketName:              r2BucketName,
		R2PublicDomain:            r2PublicDomain,
		R2Endpoint:                r2Endpoint,
		TermiiAPIKey:              termiiAPIKey,
		TermiiSenderID:            termiiSenderID,
		EnableOutboxWorker:        enableWorker,
		SafeguardingReportingEnabled: safeguardingEnabled,
		R2SafeguardingBucketName:     r2SafeguardingBucket,
		SafeguardingEncryptionKey:    safeguardingEncKey,
	}
}

// Validate verifies production requirements and safety invariants.
func (c *Config) Validate() error {
	isProduction := strings.ToLower(c.Environment) == "production"

	switch c.EmailProvider {
	case "brevo", "resend", "dev", "unconfigured", "":
		// Allowed provider identifiers
	default:
		return fmt.Errorf("unsupported EMAIL_PROVIDER: %q; supported providers are 'brevo', 'resend', and 'dev'", c.EmailProvider)
	}

	if isProduction {
		if strings.TrimSpace(c.DatabaseURL) == "" {
			return errors.New("DATABASE_URL is required in production environment; no connection string provided")
		}
		if c.DatabaseURL == DefaultDevDatabaseURL {
			return errors.New("DATABASE_URL cannot use the development fallback database in production")
		}
		if strings.TrimSpace(c.JWTSecret) == "" {
			return errors.New("JWT_SECRET is required in production environment; no secret key provided")
		}
		if c.JWTSecret == DefaultDevJWTSecret {
			return errors.New("JWT_SECRET cannot use the development fallback key in production")
		}
		if len(c.JWTSecret) < MinJWTSecretLength {
			return fmt.Errorf("JWT_SECRET is too short (%d characters); minimum requirement is %d characters in production", len(c.JWTSecret), MinJWTSecretLength)
		}
		if len(c.AllowedOrigins) == 0 {
			return errors.New("ALLOWED_ORIGINS is required in production environment; specify explicit frontend origin(s)")
		}
		for _, o := range c.AllowedOrigins {
			if strings.Contains(o, "*") {
				return fmt.Errorf("wildcard origin %q is prohibited in production; specify explicit frontend origins", o)
			}
		}
		if strings.Contains(c.FrontendBaseURL, "localhost") || strings.Contains(c.FrontendBaseURL, "127.0.0.1") {
			return fmt.Errorf("FRONTEND_BASE_URL cannot point to localhost in production; current value: %q", c.FrontendBaseURL)
		}

		if c.EmailProvider == "brevo" {
			if c.BrevoAPIKey == "" {
				return errors.New("BREVO_API_KEY is required in production when EMAIL_PROVIDER is set to brevo")
			}
			if c.EmailFromAddress == "" || !strings.Contains(c.EmailFromAddress, "@") {
				return errors.New("EMAIL_FROM_ADDRESS must be a valid email address in production when EMAIL_PROVIDER is set to brevo")
			}
		} else if c.EmailProvider == "resend" {
			if c.ResendAPIKey == "" {
				return errors.New("RESEND_API_KEY is required in production when EMAIL_PROVIDER is set to resend")
			}
			if c.EmailFrom == "" {
				return errors.New("EMAIL_FROM is required in production when EMAIL_PROVIDER is set to resend")
			}
		} else if c.EmailProvider == "dev" {
			return errors.New("EMAIL_PROVIDER cannot be set to 'dev' in production; configure 'brevo' or 'resend'")
		}

		switch c.PaymentProvider {
		case "sandbox", "dev_sandbox":
			return errors.New("PAYMENT_PROVIDER cannot be set to 'sandbox' or 'dev_sandbox' in production environment; live Paystack credentials required or leave unconfigured to safely disable checkout")
		case "paystack":
			if c.PaystackSecretKey == "" {
				return errors.New("PAYSTACK_SECRET_KEY is required in production when PAYMENT_PROVIDER is set to paystack")
			}
		case "unconfigured", "":
			// Allowed in production: checkout will be safely disabled until keys are supplied
		default:
			return fmt.Errorf("unsupported PAYMENT_PROVIDER in production: %q; supported providers are 'paystack' and 'unconfigured'", c.PaymentProvider)
		}

		// Cloudflare R2: If any R2 configuration is provided in production, all required keys must be provided
		hasAnyR2 := c.R2AccessKeyID != "" || c.R2SecretAccessKey != "" || c.R2BucketName != "" || c.R2AccountID != ""
		if hasAnyR2 {
			if c.R2AccessKeyID == "" {
				return errors.New("R2_ACCESS_KEY_ID is required in production when Cloudflare R2 is configured")
			}
			if c.R2SecretAccessKey == "" {
				return errors.New("R2_SECRET_ACCESS_KEY is required in production when Cloudflare R2 is configured")
			}
			if c.R2BucketName == "" {
				return errors.New("R2_BUCKET_NAME is required in production when Cloudflare R2 is configured")
			}
			if c.R2AccountID == "" && c.R2Endpoint == "" {
				return errors.New("R2_ACCOUNT_ID (or R2_ENDPOINT) is required in production when Cloudflare R2 is configured")
			}
		}

		// Safeguarding: in production, if enabled, require encryption key and verify length >= 32
		if c.SafeguardingReportingEnabled {
			if strings.TrimSpace(c.SafeguardingEncryptionKey) == "" {
				return errors.New("SAFEGUARDING_ENCRYPTION_KEY is required in production when SAFEGUARDING_REPORTING_ENABLED is true")
			}
			if len(c.SafeguardingEncryptionKey) < 32 {
				return fmt.Errorf("SAFEGUARDING_ENCRYPTION_KEY is too short (%d bytes); minimum 32 bytes required for AES-256", len(c.SafeguardingEncryptionKey))
			}
		}
	}
	return nil
}

// LoadAndValidate loads configuration and verifies all required invariants.
func LoadAndValidate() (*Config, error) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
