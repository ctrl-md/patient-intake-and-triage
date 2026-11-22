package intake

import "testing"

func TestNewToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != tokenLength {
		t.Fatalf("token length = %d, want %d", len(token), tokenLength)
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	if hash == token || hash != HashToken(token) {
		t.Fatal("hash must be a different value that HashToken reproduces")
	}

	other, _, _ := NewToken()
	if other == token {
		t.Fatal("two tokens were the same")
	}
}
