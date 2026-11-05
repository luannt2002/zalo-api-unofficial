package format

import "testing"

func TestColorConversion(t *testing.T) {
	// In zca-js: hexToNegativeColor('#00FF00') -> -16711936
	neg := HexToNegativeColor("#00FF00")
	if neg != -16711936 {
		t.Fatalf("HexToNegativeColor(#00FF00) = %d; expected -16711936", neg)
	}

	hexBack := NegativeColorToHex(neg)
	if hexBack != "#00FF00" {
		t.Fatalf("NegativeColorToHex(%d) = %s; expected #00FF00", neg, hexBack)
	}

	// Red #FF0000 -> 0xFFFF0000 -> -65536
	negRed := HexToNegativeColor("FF0000")
	if negRed != -65536 {
		t.Fatalf("HexToNegativeColor(FF0000) = %d; expected -65536", negRed)
	}
	if NegativeColorToHex(negRed) != "#FF0000" {
		t.Fatalf("NegativeColorToHex(%d) = %s; expected #FF0000", negRed, NegativeColorToHex(negRed))
	}
}
