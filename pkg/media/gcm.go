package media

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

// DecryptMediaGCM giải mã AES-256-GCM toàn bộ payload media (Wave 18 & media-gcm-auth.mjs).
// Yêu cầu: key (32 bytes), iv (12 bytes), tag (16 bytes).
// Khi xác thực tag thất bại, buffer sẽ bị xóa sạch (zeroed) để tránh rò rỉ dữ liệu nhạy cảm.
func DecryptMediaGCM(ciphertext, key, iv, tag []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("media: invalid key length %d, expected 32", len(key))
	}
	if len(iv) != 12 {
		return nil, fmt.Errorf("media: invalid iv length %d, expected 12", len(iv))
	}
	if len(tag) != 16 {
		return nil, fmt.Errorf("media: invalid tag length %d, expected 16", len(tag))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("media: cipher creation failed: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("media: gcm initialization failed: %w", err)
	}

	// GCM Open yêu cầu ciphertext được ghép liền với tag ở đuôi: ciphertext || tag
	payloadWithTag := make([]byte, len(ciphertext)+len(tag))
	copy(payloadWithTag, ciphertext)
	copy(payloadWithTag[len(ciphertext):], tag)

	plaintext, err := aesGCM.Open(nil, iv, payloadWithTag, nil)
	if err != nil {
		for i := range payloadWithTag {
			payloadWithTag[i] = 0
		}
		return nil, errors.New("media: authentication failed")
	}

	return plaintext, nil
}
