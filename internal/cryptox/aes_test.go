package cryptox_test

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/cryptox"
)

func TestDecodeAES256GCM(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("authenticated Zalo media payload for test")

	// Tạo ciphertext và tag chuẩn bằng Go standard library
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	sealed := gcm.Seal(nil, iv, plaintext, nil)
	// Trong Go GCM: sealed = ciphertext (len: len(plaintext)) + tag (len: 16)
	ct := sealed[:len(plaintext)]
	tag := sealed[len(plaintext):]

	// 1. Giải mã hợp lệ
	decrypted, err := cryptox.DecodeAES256GCM(key, iv, tag, ct)
	if err != nil {
		t.Fatalf("DecodeAES256GCM failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("decrypted = %q, expected %q", decrypted, plaintext)
	}

	// 2. Auth tag giả mạo / sai lệch
	badTag := make([]byte, 16)
	copy(badTag, tag)
	badTag[0] ^= 0xFF
	if _, err := cryptox.DecodeAES256GCM(key, iv, badTag, ct); err == nil {
		t.Fatal("expected error on tampered auth tag")
	}

	// 3. Sai độ dài tham số
	if _, err := cryptox.DecodeAES256GCM(key[:16], iv, tag, ct); err == nil {
		t.Fatal("expected error on key length != 32")
	}
	if _, err := cryptox.DecodeAES256GCM(key, iv[:8], tag, ct); err == nil {
		t.Fatal("expected error on iv length != 12")
	}
	if _, err := cryptox.DecodeAES256GCM(key, iv, tag[:10], ct); err == nil {
		t.Fatal("expected error on tag length != 16")
	}
}

func TestAESCBCRoundTrip(t *testing.T) {
	key := []byte("1234567890123456") // 16 bytes
	msg := "hello zalo encryption"

	enc, err := cryptox.EncodeAESCBC(key, msg, cryptox.EncryptTypeBase64)
	if err != nil {
		t.Fatalf("EncodeAESCBC failed: %v", err)
	}

	dec, err := cryptox.DecodeAESCBC(key, enc)
	if err != nil {
		t.Fatalf("DecodeAESCBC failed: %v", err)
	}

	if string(dec) != msg {
		t.Fatalf("decoded = %q, expected %q", dec, msg)
	}
}

