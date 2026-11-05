package format

import "testing"

func TestNormalizeHolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Nguyễn Văn A", "NGUYEN VAN A"},
		{"Đặng Thị B", "DANG THI B"},
		{"  lê   hoàng   nam  ", "LE HOANG NAM"},
		{"abc", ""}, // < 5 chars
		{"", ""},
		{"John Doe 123", "JOHN DOE 123"},
		{"Trần-Đức#Bo!", "TRAN DUC BO"},
	}

	for _, tc := range tests {
		got := NormalizeHolderName(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeHolderName(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}
}
