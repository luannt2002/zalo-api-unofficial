package media

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

func TestDecryptMediaGCM(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("hello zalo media authenticated gcm payload")

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt: result contains ciphertext || tag
	sealed := gcm.Seal(nil, iv, plaintext, nil)
	ciphertext := sealed[:len(sealed)-16]
	tag := sealed[len(sealed)-16:]

	// 1. Decrypt success
	decrypted, err := DecryptMediaGCM(ciphertext, key, iv, tag)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %s != plaintext %s", string(decrypted), string(plaintext))
	}

	// 2. Tampered tag fails
	badTag := make([]byte, 16)
	copy(badTag, tag)
	badTag[0] ^= 0xFF
	_, err = DecryptMediaGCM(ciphertext, key, iv, badTag)
	if err == nil {
		t.Fatalf("expected error on tampered tag, got nil")
	}

	// 3. Invalid key/iv length
	_, err = DecryptMediaGCM(ciphertext, key[:16], iv, tag)
	if err == nil {
		t.Fatalf("expected error on invalid key length")
	}
	_, err = DecryptMediaGCM(ciphertext, key, iv[:8], tag)
	if err == nil {
		t.Fatalf("expected error on invalid iv length")
	}
}

