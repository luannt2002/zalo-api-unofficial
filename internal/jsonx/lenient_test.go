package jsonx

import (
	"encoding/json"
	"testing"
)

// Khoan dung chỉ đổi số<->chuỗi số; rác vẫn phải báo lỗi, không được thành 0 im lặng.
func TestUnmarshalLenientKhongNuotRac(t *testing.T) {
	var v struct {
		N int    `json:"n"`
		S string `json:"s"`
	}
	for _, raw := range []string{`{"n":"abc"}`, `{"n":{"x":1}}`, `{"s":{"x":1}}`, `{"n":"1.5"}`, `{"n":"NaN"}`} {
		if err := UnmarshalLenient([]byte(raw), &v); err == nil {
			t.Errorf("UnmarshalLenient(%s) = nil, mong lỗi (v=%+v)", raw, v)
		}
	}
	if err := UnmarshalLenient([]byte(`{"n":"-12","s":12345678901234567890}`), &v); err != nil || v.N != -12 || v.S != "12345678901234567890" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	if err := UnmarshalLenient([]byte(`{"n":`), &v); err == nil {
		t.Fatal("JSON hỏng phải báo lỗi")
	}
}

func TestScalarString(t *testing.T) {
	for raw, want := range map[string]string{`"a"`: "a", `123`: "123", `null`: "", ``: "", `8070450532247928833`: "8070450532247928833"} {
		if got := ScalarString(json.RawMessage(raw)); got != want {
			t.Errorf("ScalarString(%s) = %q, mong %q", raw, got, want)
		}
	}
}
