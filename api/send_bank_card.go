package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	SendBankCardData struct {
		BinBank     model.BinBankCard
		NumAccBank  string
		NameAccBank string
	}
	// SendBankCardResponse: Zalo trả `data` KHÔNG nhất quán — có bản trả STRING TRẦN (msgId),
	// có bản trả OBJECT {msgId}. Khai `= string` fail khi là object; khai struct thường fail khi
	// là string. UnmarshalJSON dưới nhận CẢ HAI (chống lỗi "unmarshal string/object into ...").
	SendBankCardResponse struct {
		MsgID flexID `json:"msgId"`
	}
	SendBankCardFn = func(ctx context.Context, threadID string, threadType model.ThreadType, data SendBankCardData) (*SendBankCardResponse, error)
)

// UnmarshalJSON nhận `data` là STRING TRẦN ("8028..") hoặc OBJECT ({"msgId":".."}).
func (r *SendBankCardResponse) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' { // string trần = msgId
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		r.MsgID = flexID(s)
		return nil
	}
	var obj struct {
		MsgID flexID `json:"msgId"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	r.MsgID = obj.MsgID
	return nil
}

func (a *api) SendBankCard(ctx context.Context, threadID string, threadType model.ThreadType, data SendBankCardData) (*SendBankCardResponse, error) {
	return a.e.SendBankCard(ctx, threadID, threadType, data)
}

var sendBankCardFactory = apiFactory[*SendBankCardResponse, SendBankCardFn]()(
	func(a *api, sc session.Context, u factoryUtils[*SendBankCardResponse]) (SendBankCardFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/card", nil, true)

		return func(ctx context.Context, threadID string, threadType model.ThreadType, data SendBankCardData) (*SendBankCardResponse, error) {
			now := time.Now().UnixMilli()
			nameAccBank := data.NameAccBank

			if len(nameAccBank) == 0 {
				nameAccBank = "---"
			}

			payload := map[string]any{
				"binBank":     data.BinBank,
				"numAccBank":  data.NumAccBank,
				"nameAccBank": strings.ToUpper(nameAccBank),
				"cliMsgId":    strconv.FormatInt(now, 10),
				"tsMsg":       now,
				"destUid":     threadID,
				"destType":    jsonx.B2I(threadType == model.ThreadTypeGroup),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.SendBankCard", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   body,
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
