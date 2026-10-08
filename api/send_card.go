package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	// SendCardOptions — MỘT danh thiếp cho MỘT lần gọi. Cố ý giữ nguyên một UserID, không
	// mở thành danh sách, vì giao thức Zalo không nhận nhiều liên hệ trong một lệnh:
	//
	//	POST /api/message/forward (msgType=6) mang đúng MỘT msgInfo = {contactUid, qrCodeUrl}
	//	— một chuỗi JSON, không phải mảng. Không có khóa nào trong payload chứa danh sách uid.
	//
	// Bản "nhiều" của họ hàng endpoint này là mforward (xem forward_message.go): nó nhân bản
	// theo NGƯỜI NHẬN (toIds/grids), tức gửi CÙNG MỘT nội dung tới nhiều hội thoại — không
	// phải gửi NHIỀU danh thiếp vào một hội thoại. Dùng nó cho việc này là hiểu sai bài toán.
	//
	// Vậy nên gửi 9 danh thiếp vẫn là 9 lệnh HTTP. Chuyện "9 card ăn 9 suất hạn mức" được xử
	// ở tầng ứng dụng thay vì bịa endpoint gộp: zalo-be/internal/worker/sender.go coi một LÔ
	// card là MỘT đơn vị token bucket (xem mục "LÔ DANH THIẾP" ở cuối file đó).
	SendCardOptions struct {
		UserID      string
		PhoneNumber string
		TTL         int
	}
	SendCardResponse struct {
		MsgID flexID `json:"msgId"`
	}
	SendCardFn = func(ctx context.Context, threadID string, threadType model.ThreadType, options SendCardOptions) (*SendCardResponse, error)
)

func (a *api) SendCard(ctx context.Context, threadID string, threadType model.ThreadType, options SendCardOptions) (*SendCardResponse, error) {
	return a.e.SendCard(ctx, threadID, threadType, options)
}

var sendCardFactory = apiFactory[*SendCardResponse, SendCardFn]()(
	func(a *api, sc session.Context, u factoryUtils[*SendCardResponse]) (SendCardFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("file"), "")
		serviceURLs := map[model.ThreadType]string{
			model.ThreadTypeUser:  u.MakeURL(base+"/api/message/forward", nil, true),
			model.ThreadTypeGroup: u.MakeURL(base+"/api/group/forward", nil, true),
		}

		return func(ctx context.Context, threadID string, threadType model.ThreadType, options SendCardOptions) (*SendCardResponse, error) {
			data, err := a.GetQR(ctx, options.UserID)
			if err != nil {
				return nil, err
			}

			// GetQR trả map[uid]qrUrl; nếu thiếu key ⇒ qrCodeUrl rỗng ⇒ Zalo dựng card hỏng
			// mà KHÔNG báo lỗi. Chặn sớm để lỗi hiện rõ thay vì gửi card trống.
			qrURL, ok := (*data)[options.UserID]
			if !ok || qrURL == "" {
				return nil, errs.NewZCA("missing QR code url for contact", "api.SendCard")
			}

			msgInfo := map[string]any{
				"contactUid": options.UserID,
				"qrCodeUrl":  qrURL,
			}

			if options.PhoneNumber != "" {
				msgInfo["phone"] = options.PhoneNumber
			}

			payload := map[string]any{
				"ttl":      options.TTL,
				"msgType":  6,
				"clientId": strconv.FormatInt(time.Now().UnixMilli(), 10),
				"msgInfo":  jsonx.Stringify(msgInfo),
			}

			if threadType == model.ThreadTypeUser {
				payload["toId"] = threadID
				payload["imei"] = sc.IMEI()
			} else {
				payload["grid"] = threadID
				payload["visibility"] = 0
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.SendCard", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURLs[threadType], &httpx.RequestOptions{
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
