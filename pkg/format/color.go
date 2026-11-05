package format

import (
	"fmt"
	"strconv"
	"strings"
)

// HexToNegativeColor chuyển mã hex (#RRGGBB hoặc RRGGBB) sang số âm signed 32-bit (ARGB với alpha 0xFF).
func HexToNegativeColor(hexStr string) int32 {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) == 6 {
		hexStr = "FF" + hexStr
	}
	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return 0
	}
	return int32(val)
}

// NegativeColorToHex chuyển số âm signed 32-bit sang mã màu hex dạng #RRGGBB.
func NegativeColorToHex(c int32) string {
	u := uint32(c)
	rgb := u & 0x00FFFFFF
	return fmt.Sprintf("#%06X", rgb)
}
