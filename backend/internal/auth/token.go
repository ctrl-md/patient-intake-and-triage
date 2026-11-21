package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims is what we keep inside the token.
type Claims struct {
	Subject   string `json:"sub"`  // staff id
	Role      string `json:"role"` // nurse, doctor, admin
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// TokenManager makes and checks JWTs signed with HMAC-SHA256 (HS256).
//
// It only accepts tokens that it made itself: the header must be exactly the
// one we write. That blocks tricks like alg "none" or switching algorithms.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}
}

var b64 = base64.RawURLEncoding

// header is {"alg":"HS256","typ":"JWT"}, encoded once.
var header = b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Issue makes a signed token for a staff member and says when it expires.
func (m *TokenManager) Issue(staffID, role string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)

	payload, err := json.Marshal(Claims{
		Subject:   staffID,
		Role:      role,
		IssuedAt:  now.Unix(),
		ExpiresAt: exp.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	signingInput := header + "." + b64.EncodeToString(payload)
	return signingInput + "." + m.sign(signingInput), exp, nil
}

// Parse checks the signature and the expiry, then returns the claims.
func (m *TokenManager) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != header {
		return Claims{}, ErrInvalidToken
	}

	got, err := b64.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	want, _ := b64.DecodeString(m.sign(parts[0] + "." + parts[1]))
	if !hmac.Equal(got, want) {
		return Claims{}, ErrInvalidToken
	}

	raw, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil || c.Subject == "" {
		return Claims{}, ErrInvalidToken
	}
	if m.now().Unix() >= c.ExpiresAt {
		return Claims{}, ErrExpiredToken
	}
	return c, nil
}

func (m *TokenManager) sign(input string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(input))
	return b64.EncodeToString(mac.Sum(nil))
}
