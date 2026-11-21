package auth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestTokenRoundTrip(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Hour)

	token, exp, err := tm.Issue("staff-1", "doctor")
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 59*time.Minute {
		t.Fatalf("expiry too early: %v", exp)
	}

	c, err := tm.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "staff-1" || c.Role != "doctor" {
		t.Fatalf("wrong claims: %+v", c)
	}
}

func TestTokenRejectsBadTokens(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Hour)
	good, _, _ := tm.Issue("staff-1", "nurse")
	parts := strings.Split(good, ".")

	other := NewTokenManager("a-completely-different-secret-value!!", time.Hour)
	otherToken, _, _ := other.Issue("staff-1", "nurse")

	// A token that claims to be admin but keeps the old signature.
	forged, _, _ := tm.Issue("staff-1", "admin")
	forgedParts := strings.Split(forged, ".")

	tests := map[string]string{
		"empty":             "",
		"garbage":           "not-a-token",
		"two parts":         parts[0] + "." + parts[1],
		"wrong secret":      otherToken,
		"tampered payload":  parts[0] + "." + forgedParts[1] + "." + parts[2],
		"empty signature":   parts[0] + "." + parts[1] + ".",
		"alg none":          b64.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + parts[1] + ".",
		"different header":  b64.EncodeToString([]byte(`{"alg":"HS512","typ":"JWT"}`)) + "." + parts[1] + "." + parts[2],
		"signature garbage": parts[0] + "." + parts[1] + ".!!!",
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tm.Parse(token); err != ErrInvalidToken {
				t.Fatalf("got %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestTokenExpires(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Hour)
	now := time.Now()
	tm.now = func() time.Time { return now }

	token, _, _ := tm.Issue("staff-1", "nurse")

	tm.now = func() time.Time { return now.Add(59 * time.Minute) }
	if _, err := tm.Parse(token); err != nil {
		t.Fatalf("should still be valid: %v", err)
	}

	tm.now = func() time.Time { return now.Add(61 * time.Minute) }
	if _, err := tm.Parse(token); err != ErrExpiredToken {
		t.Fatalf("got %v, want ErrExpiredToken", err)
	}
}
