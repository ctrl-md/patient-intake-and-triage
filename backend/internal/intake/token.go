package intake

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// tokenLength is the length of a link token: 32 random bytes, base64 without padding.
const tokenLength = 43

// NewToken makes a random link token and the hash we store for it.
// The token itself is never saved. It is shown once, when staff create the link.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is what we keep in the database (intakes.token_hash).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
