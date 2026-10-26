package secret

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	sealed, err := SealWithKey(key, []byte("rahasia"))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := SealWithKey(key, []byte("rahasia"))
	if bytes.Equal(sealed, again) {
		t.Fatal("nonce reused: identical ciphertexts")
	}
	plain, err := OpenWithKey(key, sealed)
	if err != nil || string(plain) != "rahasia" {
		t.Fatalf("got %q, %v", plain, err)
	}

	sealed[len(sealed)-1] ^= 1
	if _, err := OpenWithKey(key, sealed); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
	other := make([]byte, 32)
	rand.Read(other)
	if _, err := OpenWithKey(other, again); err == nil {
		t.Fatal("wrong key was accepted")
	}
	if _, err := OpenWithKey(key, []byte{1, 2}); err == nil {
		t.Fatal("truncated data was accepted")
	}
}
