package cryptox

import (
	"encoding/binary"
)

// SocketConst16 là hằng số 16-byte cố định trích xuất từ .rodata (offset 0x18030)
// của thư viện native libnative_utils.so (xác minh độc lập qua Ghidra và REA).
var SocketConst16 = []byte{
	0x48, 0xed, 0x3a, 0x1f, 0x52, 0xf9, 0x39, 0x41,
	0xe3, 0x21, 0x0d, 0x3f, 0x2e, 0x5d, 0xe4, 0x10,
}

// SocketMagic là giá trị magic 32-bit (0x305b1f1d) ghi dạng Little-Endian ở 4 byte cuối của key.
const SocketMagic uint32 = 0x305b1f1d

// DeriveSocketKey xây dựng 32-byte AES-256 key cho tầng native socket params.
// Cấu trúc 32 bytes:
//   - Byte 0..3:   sequence (int32, Big-Endian)
//   - Byte 4..11:  privateKey (int64, Big-Endian)
//   - Byte 12..27: SocketConst16 (16 bytes)
//   - Byte 28..31: SocketMagic (uint32, Little-Endian: 0x1d, 0x1f, 0x5b, 0x30)
func DeriveSocketKey(sequence int32, privateKey int64) []byte {
	key := make([]byte, 32)
	binary.BigEndian.PutUint32(key[0:4], uint32(sequence))
	binary.BigEndian.PutUint64(key[4:12], uint64(privateKey))
	copy(key[12:28], SocketConst16)
	binary.LittleEndian.PutUint32(key[28:32], SocketMagic)
	return key
}

// EncodeSocketParams mã hóa chuỗi params sử dụng AES-256-CBC với IV = 0^16 và PKCS#7 padding,
// trả về chuỗi Base64 chuẩn khớp 100% với hàm CoreUtils.encodeSocketParamsNative.
func EncodeSocketParams(sequence int32, privateKey int64, params string) (string, error) {
	key := DeriveSocketKey(sequence, privateKey)
	return EncodeAESCBC(key, params, EncryptTypeBase64)
}

// DecodeSocketParams giải mã chuỗi Base64 socket params về chuỗi plaintext ban đầu.
func DecodeSocketParams(sequence int32, privateKey int64, encoded string) (string, error) {
	key := DeriveSocketKey(sequence, privateKey)
	b, err := DecodeAESCBC(key, encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

