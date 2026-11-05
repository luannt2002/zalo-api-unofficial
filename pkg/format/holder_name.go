package format

import (
	"strings"
	"unicode"
)

// NormalizeHolderName chuẩn hoá tên chủ tài khoản ngân hàng:
// bỏ dấu tiếng Việt, đ/Đ -> D, chữ in hoa, chỉ giữ A-Z0-9 và khoảng trắng.
// Trả về rỗng nếu độ dài sau chuẩn hoá < 5.
func NormalizeHolderName(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	var sb strings.Builder
	for _, r := range input {
		switch r {
		case 'à', 'á', 'ả', 'ã', 'ạ', 'â', 'ầ', 'ấ', 'ẩ', 'ẫ', 'ậ', 'ă', 'ằ', 'ắ', 'ẳ', 'ẵ', 'ặ',
			'À', 'Á', 'Ả', 'Ã', 'Ạ', 'Â', 'Ầ', 'Ấ', 'Ẩ', 'Ẫ', 'Ậ', 'Ă', 'Ằ', 'Ắ', 'Ẳ', 'Ẵ', 'Ặ':
			sb.WriteRune('A')
		case 'è', 'é', 'ẻ', 'ẽ', 'ẹ', 'ê', 'ề', 'ế', 'ể', 'ễ', 'ệ',
			'È', 'É', 'Ẻ', 'Ẽ', 'Ẹ', 'Ê', 'Ề', 'Ế', 'Ể', 'Ễ', 'Ệ':
			sb.WriteRune('E')
		case 'ì', 'í', 'ỉ', 'ĩ', 'ị',
			'Ì', 'Í', 'Ỉ', 'Ĩ', 'Ị':
			sb.WriteRune('I')
		case 'ò', 'ó', 'ỏ', 'õ', 'ọ', 'ô', 'ồ', 'ố', 'ổ', 'ỗ', 'ộ', 'ơ', 'ờ', 'ớ', 'ở', 'ỡ', 'ợ',
			'Ò', 'Ó', 'Ỏ', 'Õ', 'Ọ', 'Ô', 'Ồ', 'Ố', 'Ổ', 'Ỗ', 'Ộ', 'Ơ', 'Ờ', 'Ớ', 'Ở', 'Ỡ', 'Ợ':
			sb.WriteRune('O')
		case 'ù', 'ú', 'ủ', 'ũ', 'ụ', 'ư', 'ừ', 'ứ', 'ử', 'ữ', 'ự',
			'Ù', 'Ú', 'Ủ', 'Ũ', 'Ụ', 'Ư', 'Ừ', 'Ứ', 'Ử', 'Ữ', 'Ự':
			sb.WriteRune('U')
		case 'ỳ', 'ý', 'ỷ', 'ỹ', 'ỵ',
			'Ỳ', 'Ý', 'Ỷ', 'Ỹ', 'Ỵ':
			sb.WriteRune('Y')
		case 'đ', 'Đ':
			sb.WriteRune('D')
		default:
			if r >= 0x0300 && r <= 0x036F {
				continue
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				sb.WriteRune(unicode.ToUpper(r))
			} else {
				sb.WriteRune(' ')
			}
		}
	}

	fields := strings.Fields(sb.String())
	res := strings.Join(fields, " ")
	if len(res) < 5 {
		return ""
	}
	return res
}
