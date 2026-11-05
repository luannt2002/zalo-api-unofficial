package httpx_test

import (
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
)

func TestZCIDRoundTrip(t *testing.T) {
	apiType := uint(30)
	imei := "test-imei-uuid-12345"
	flt := time.Now().UnixMilli()

	pe, err := httpx.NewParamsEncryptor(apiType, imei, flt)
	if err != nil {
		t.Fatalf("NewParamsEncryptor error: %v", err)
	}

	params := pe.GetParams()
	if params == nil || params.ZCID == "" {
		t.Fatal("expected non-empty zcid in params")
	}

	// Giải mã zcid qua DecodeZCID
	decodedType, decodedIMEI, decodedFLT, err := httpx.DecodeZCID(params.ZCID)
	if err != nil {
		t.Fatalf("DecodeZCID error: %v", err)
	}

	if decodedType != apiType {
		t.Errorf("decodedType = %d, expected %d", decodedType, apiType)
	}
	if decodedIMEI != imei {
		t.Errorf("decodedIMEI = %q, expected %q", decodedIMEI, imei)
	}
	if decodedFLT != flt {
		t.Errorf("decodedFLT = %d, expected %d", decodedFLT, flt)
	}
}
