package security

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestParseJWTRequiresSessionClaimsAndHS256(t *testing.T) {
	secret := "regression-test-secret"
	valid := jwt.MapClaims{"sub": "user", "jti": "session", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()}
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		remove string
	}{
		{"missing expiry", jwt.SigningMethodHS256, "exp"},
		{"missing subject", jwt.SigningMethodHS256, "sub"},
		{"missing session", jwt.SigningMethodHS256, "jti"},
		{"alternate algorithm", jwt.SigningMethodHS512, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{}
			for k, v := range valid {
				claims[k] = v
			}
			delete(claims, tc.remove)
			raw, err := jwt.NewWithClaims(tc.method, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseJWT(raw, secret); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
	raw, _, err := GenerateJWT("user", "user@example.org", "member", "session", secret, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJWT(raw, secret); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}
