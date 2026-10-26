package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SchedulerResponse matches the JSON structure returned by POST /api/v1/internal/cron/reports
type SchedulerResponse struct {
	Status           string   `json:"status"`
	ReportsGenerated int      `json:"reports_generated"`
	ReportsSkipped   int      `json:"reports_skipped"`
	ReportsFailed    int      `json:"reports_failed"`
	Errors           []string `json:"errors"`
}

// Config holds runtime parameters for the reporting cron runner.
type Config struct {
	Secret     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// LoadConfigFromEnv reads and validates configuration from environment variables.
func LoadConfigFromEnv() (*Config, error) {
	secret := strings.TrimSpace(os.Getenv("REPORT_CRON_SECRET"))
	if secret == "" {
		return nil, errors.New("REPORT_CRON_SECRET is missing or empty")
	}

	rawURL := strings.TrimRight(strings.TrimSpace(os.Getenv("RATELS_API_URL")), "/")
	if rawURL == "" {
		return nil, errors.New("RATELS_API_URL is missing or empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid RATELS_API_URL: %q", rawURL)
	}

	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" {
		appEnv = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	isProduction := appEnv == "production"

	hostname := strings.ToLower(parsedURL.Hostname())
	isLoopback := hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "[::1]"

	// Enforce HTTPS in production environments
	if isProduction && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("RATELS_API_URL must use HTTPS in production (got %q)", parsedURL.Scheme)
	}

	// Reject remote HTTP URLs regardless of environment
	if parsedURL.Scheme != "https" && !isLoopback {
		return nil, fmt.Errorf("RATELS_API_URL must use HTTPS for remote hosts (got scheme %q for %s)", parsedURL.Scheme, hostname)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return nil, fmt.Errorf("RATELS_API_URL must use http or https scheme (got %q)", parsedURL.Scheme)
	}

	return &Config{
		Secret:  secret,
		BaseURL: rawURL,
		Timeout: 5 * time.Minute,
	}, nil
}

// Run executes the automated reporting cron request against the Ratels API.
func Run(ctx context.Context, cfg *Config) error {
	if cfg == nil {
		return errors.New("nil configuration provided")
	}
	if cfg.Secret == "" {
		return errors.New("REPORT_CRON_SECRET is missing")
	}
	if cfg.BaseURL == "" {
		return errors.New("RATELS_API_URL is missing")
	}

	endpointURL := cfg.BaseURL + "/api/v1/internal/cron/reports?period_type=monthly"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, nil)
	if err != nil {
		return fmt.Errorf("failed creating HTTP request: %w", err)
	}

	req.Header.Set("X-Cron-Secret", cfg.Secret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Ratels-Report-Cron/1.0")

	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}
		client = &http.Client{
			Timeout: timeout,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("scheduler request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read up to 1MB response
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("reading scheduler response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("scheduler authentication failed: HTTP 401 Unauthorized")
		case http.StatusConflict:
			return fmt.Errorf("scheduler lock conflict: HTTP 409 Conflict")
		default:
			return fmt.Errorf("scheduler returned HTTP %d", resp.StatusCode)
		}
	}

	var result SchedulerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("invalid scheduler JSON: %w", err)
	}

	if result.Status != "completed" {
		return fmt.Errorf("scheduler finished with status %q", result.Status)
	}

	if result.ReportsFailed != 0 || len(result.Errors) != 0 {
		return fmt.Errorf("scheduler reported %d failures with %d errors", result.ReportsFailed, len(result.Errors))
	}

	log.Printf(
		"[REPORT-CRON] Reporting completed successfully: generated=%d, skipped=%d, failed=%d",
		result.ReportsGenerated,
		result.ReportsSkipped,
		result.ReportsFailed,
	)

	return nil
}

func run(ctx context.Context) error {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return err
	}
	return Run(ctx, cfg)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Printf("[REPORT-CRON] Execution failed: %v", err)
		os.Exit(1)
	}
}
