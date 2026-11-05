package cryptox

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
)

// EncryptPIN băm mã PIN 4 chữ số thành chuỗi MD5 hex 32 ký tự theo chuẩn Zalo Web.
func EncryptPIN(pin string) string {
	h := md5.Sum([]byte(pin))
	return hex.EncodeToString(h[:])
}

// ValidatePIN xác thực mã PIN với chuỗi MD5 hex bằng thời gian không đổi (constant-time).
func ValidatePIN(encryptedPIN, pin string) bool {
	computed := EncryptPIN(pin)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(encryptedPIN)) == 1
}
