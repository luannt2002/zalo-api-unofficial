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

// Chuỗi rỗng / chỉ khoảng trắng ở trường số thành 0, như Number("") / Number("  ") của JS (zca-js
// đọc các trường này không ép kiểu, Zalo có lúc gửi ""). Khoảng trắng hai đầu số bị bỏ như Number(" 12 ").
func TestUnmarshalLenientChuoiRongThanh0(t *testing.T) {
	for raw, want := range map[string]int{`{"n":""}`: 0, `{"n":"   "}`: 0, `{"n":"\t\n"}`: 0, `{"n":" 12 "}`: 12} {
		v := struct {
			N int `json:"n"`
		}{N: -1}
		if err := UnmarshalLenient([]byte(raw), &v); err != nil || v.N != want {
			t.Errorf("UnmarshalLenient(%s) = (%d, %v), mong (%d, nil)", raw, v.N, err, want)
		}
	}
	var f struct {
		F float64 `json:"f"`
	}
	if err := UnmarshalLenient([]byte(`{"f":""}`), &f); err != nil || f.F != 0 {
		t.Errorf("float rỗng = (%v, %v), mong (0, nil)", f.F, err)
	}
}
