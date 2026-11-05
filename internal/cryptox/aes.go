package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type EncryptType string

const (
	EncryptTypeHex    EncryptType = "hex"
	EncryptTypeBase64 EncryptType = "base64"
)

func EncodeAESCBC(key []byte, data string, encType EncryptType) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cryptox: new cipher: %w", err)
	}

	plain, err := Pkcs7Padding([]byte(data), aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("cryptox: pkcs7 padding: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	ct := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, plain)

	switch encType {
	case EncryptTypeHex:
		return hex.EncodeToString(ct), nil
	default:
		return base64.StdEncoding.EncodeToString(ct), nil
	}
}

func DecodeAESCBC(key []byte, data string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("cryptox: base64 decode: %w", err)
	}

	return DecodeAESCBCDirect(key, ciphertext)
}

// DecodeAESCBCDirect giải mã trực tiếp ciphertext nhị phân sử dụng AES-CBC với IV = 0^16 và PKCS#7 unpadding.
func DecodeAESCBCDirect(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptox: new cipher: %w", err)
	}

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("cryptox: ciphertext length %d is not a multiple of block size", len(ciphertext))
	}

	iv := make([]byte, aes.BlockSize)
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)

	plain, err = Pkcs7Unpadding(plain, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("cryptox: pkcs7 unpadding: %w", err)
	}

	return plain, nil
}

func DecodeAESGCM(key, iv, aad, ct []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptox: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return nil, fmt.Errorf("cryptox: new gcm: %w", err)
	}

	plain, err := gcm.Open(nil, iv, ct, aad)
	if err != nil {
		return nil, fmt.Errorf("cryptox: gcm open: %w", err)
	}

	return plain, nil
}

// DecodeAES256GCM giải mã dữ liệu E2EE/Media Zalo sử dụng chuẩn linear AES-256-GCM.
// Yêu cầu: key 32 bytes, iv (nonce) 12 bytes, auth tag 16 bytes.
func DecodeAES256GCM(key, iv, tag, ct []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cryptox: key must be 32 bytes (got %d)", len(key))
	}
	if len(iv) != 12 {
		return nil, fmt.Errorf("cryptox: iv must be 12 bytes (got %d)", len(iv))
	}
	if len(tag) != 16 {
		return nil, fmt.Errorf("cryptox: tag must be 16 bytes (got %d)", len(tag))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptox: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptox: new gcm: %w", err)
	}

	ciphertextWithTag := make([]byte, len(ct)+len(tag))
	copy(ciphertextWithTag, ct)
	copy(ciphertextWithTag[len(ct):], tag)

	plain, err := gcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("cryptox: gcm auth verification failed: %w", err)
	}

	return plain, nil
}
