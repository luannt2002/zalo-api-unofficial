package errs

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// Lỗi bọc phải in được nguyên nhân bên trong: listener phát lỗi giải mã dạng
// WrapZCA(WrapZCA(json error)) — nếu Error() chỉ in tầng ngoài thì người trực
// chỉ thấy "Failed to decode event data" mà không biết hỏng ở đâu.
func TestZCAErrorErrorInNguyenNhan(t *testing.T) {
	var target struct{ A int }
	jsonErr := json.Unmarshal([]byte(`{"A":"x"}`), &target)
	if jsonErr == nil {
		t.Fatal("fixture: json.Unmarshal phải lỗi")
	}

	inner := WrapZCA("failed to parse JSON data", "listener.parseJSON", jsonErr)
	outer := WrapZCA("Failed to decode event data:", "listener.handleMessages", inner)

	got := outer.Error()
	for _, want := range []string{"Failed to decode event data:", "listener.handleMessages", "failed to parse JSON data", jsonErr.Error()} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, thiếu %q", got, want)
		}
	}

	if !errors.Is(WrapZCA("read", "op", io.EOF), io.EOF) {
		t.Error("errors.Is phải đi xuyên lớp bọc tới nguyên nhân gốc")
	}
	var se *json.UnmarshalTypeError
	if !errors.As(outer, &se) {
		t.Error("errors.As phải tìm được *json.UnmarshalTypeError qua Unwrap")
	}
}

func TestZCAErrorKhongNguyenNhanGiuNguyenDang(t *testing.T) {
	tests := []struct {
		err  ZCAError
		want string
	}{
		{NewZCA("msg", "op"), "ZCAError: msg (op)"},
		{NewZCA("msg", ""), "ZCAError: msg"},
		{NewZCA("", "op"), "ZCAError: (op)"},
		{NewZCA("", ""), "ZCAError"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, mong %q", got, tt.want)
		}
	}
}

// Nguyên nhân dài (vd lỗi mang nội dung khung) bị cắt để một dòng log không
// phình vô hạn; phần cắt vẫn là UTF-8 hợp lệ.
func TestZCAErrorCatNganNguyenNhanDai(t *testing.T) {
	long := errors.New(strings.Repeat("ộ", 2000)) // 3 byte/rune
	got := WrapZCA("decode", "op", long).Error()

	if len(got) > 600 {
		t.Fatalf("len(Error()) = %d, mong <= 600", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("Error() bị cắt phải kết thúc bằng …, được %q", got[len(got)-10:])
	}
	if !strings.Contains(got, "ộộộ") {
		t.Fatal("Error() phải giữ phần đầu nguyên nhân")
	}
	if strings.ContainsRune(got, '�') {
		t.Fatal("cắt ngang giữa rune")
	}
}
