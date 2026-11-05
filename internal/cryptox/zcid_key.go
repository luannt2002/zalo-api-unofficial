package cryptox

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// DeriveHttpKeyFromZCID phái sinh khóa HTTP 32-byte từ chuỗi định danh zcid hex
// theo đúng thuật toán native trong libnative_utils.so / CoreUtils.a:
// 16 ký tự ở vị trí chẵn (0, 2, ..., 30) nối với 16 ký tự ở vị trí lẻ từ cuối về (31, 29, ..., 1).
func DeriveHttpKeyFromZCID(zcid string) ([]byte, error) {
	zcid = strings.TrimSpace(zcid)
	if len(zcid) < 32 {
		return nil, fmt.Errorf("cryptox: zcid must be at least 32 characters (got %d)", len(zcid))
	}

	keyText := make([]byte, 32)
	for i := 0; i < 16; i++ {
		keyText[i] = zcid[2*i]
	}
	for i := 0; i < 16; i++ {
		keyText[16+i] = zcid[len(zcid)-1-2*i]
	}

	return keyText, nil
}

// EncryptMobileHttpParams mã hóa chuỗi params dạng URL query theo chuẩn REST-sig của app mobile.
// Thuật toán: AES-256-CBC (key 32B phái sinh từ ZCID, IV=0^16, PKCS#7), xuất ra chuỗi HEX viết hoa.
func EncryptMobileHttpParams(params string, zcid string) (string, error) {
	key, err := DeriveHttpKeyFromZCID(zcid)
	if err != nil {
		return "", err
	}

	hexStr, err := EncodeAESCBC(key, params, EncryptTypeHex)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(hexStr), nil
}

// DecryptMobileHttpParams giải mã chuỗi hex uppercase của mobile params về plaintext.
func DecryptMobileHttpParams(hexCipher string, zcid string) (string, error) {
	key, err := DeriveHttpKeyFromZCID(zcid)
	if err != nil {
		return "", err
	}

	ct, err := hex.DecodeString(hexCipher)
	if err != nil {
		return "", fmt.Errorf("cryptox: hex decode: %w", err)
	}

	plain, err := DecodeAESCBCDirect(key, ct)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
