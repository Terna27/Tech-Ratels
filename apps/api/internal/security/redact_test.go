package security

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		knownSecrets []string
		mustNotHave  []string
		mustHave     []string
	}{
		{
			name:         "Postgres URL in error message",
			input:        `parse "postgres://app_user:super_secret_pw@ep-cool-db.us-east-2.aws.neon.tech:5432/ratels?sslmode=require": invalid port`,
			knownSecrets: nil,
			mustNotHave:  []string{"super_secret_pw"},
			mustHave:     []string{"[REDACTED]", "postgres://app_user:[REDACTED]@ep-cool-db.us-east-2.aws.neon.tech"},
		},
		{
			name:         "Key value connection string password",
			input:        `connection failed: host=127.0.0.1 user=admin password=SecretPassword123! dbname=ratels`,
			knownSecrets: nil,
			mustNotHave:  []string{"SecretPassword123!"},
			mustHave:     []string{"password=[REDACTED]"},
		},
		{
			name:         "Known secret URL password redacted from error description",
			input:        `failed to authenticate with token and secret: super_secret_pw`,
			knownSecrets: []string{"postgres://app_user:super_secret_pw@localhost:5432/ratels"},
			mustNotHave:  []string{"super_secret_pw"},
			mustHave:     []string{"[REDACTED]"},
		},
		{
			name:         "Known secret JWT token or key",
			input:        `fatal signature verification failure for key my_custom_jwt_secret_32_characters_long`,
			knownSecrets: []string{"my_custom_jwt_secret_32_characters_long"},
			mustNotHave:  []string{"my_custom_jwt_secret_32_characters_long"},
			mustHave:     []string{"[REDACTED]"},
		},
		{
			name:         "R2 reports secret key and access key in error message",
			input:        `S3 PutObject failed: access_key_id=AKIAR2REPORTSTOKEN123 secret_access_key=cf_super_private_secret_key_456`,
			knownSecrets: nil,
			mustNotHave:  []string{"AKIAR2REPORTSTOKEN123", "cf_super_private_secret_key_456"},
			mustHave:     []string{"access_key_id=[REDACTED]", "secret_access_key=[REDACTED]"},
		},
		{
			name:         "R2 environment variable assignment in logs",
			input:        `config error: r2_reports_secret_access_key=secret_report_token_value_999`,
			knownSecrets: nil,
			mustNotHave:  []string{"secret_report_token_value_999"},
			mustHave:     []string{"r2_reports_secret_access_key=[REDACTED]"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := RedactString(tc.input, tc.knownSecrets...)
			for _, forbidden := range tc.mustNotHave {
				if strings.Contains(res, forbidden) {
					t.Errorf("expected string to not contain %q, but got: %s", forbidden, res)
				}
			}
			for _, required := range tc.mustHave {
				if !strings.Contains(res, required) {
					t.Errorf("expected string to contain %q, but got: %s", required, res)
				}
			}
		})
	}
}

func TestRedactError(t *testing.T) {
	if RedactError(nil) != nil {
		t.Errorf("expected nil error for nil input")
	}

	rawErr := errors.New("parse error: postgres://neon_user:NeonSecretPass99@db.neon.tech/main")
	redactedErr := RedactError(rawErr)
	if strings.Contains(redactedErr.Error(), "NeonSecretPass99") {
		t.Errorf("expected password to be redacted from error, got: %s", redactedErr.Error())
	}
	if !strings.Contains(redactedErr.Error(), "[REDACTED]") {
		t.Errorf("expected error to contain [REDACTED], got: %s", redactedErr.Error())
	}
}
