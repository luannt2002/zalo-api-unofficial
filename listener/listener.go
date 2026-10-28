package listener

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/logger"
	"github.com/luannt2002/zalo-api-unofficial/internal/websocketx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

const (
	ZaloManualClosure       int = 1000
	ZaloAbnormalClosure     int = 1006
	ZaloDuplicateConnection int = 3000
	ZaloKickConnection      int = 3003
)

type CloseInfo = websocketx.CloseInfo

type Listener interface {
	Start(ctx context.Context, retryOnClose bool) error
	Stop()

	// Channels
	Connected() <-chan struct{}
	Disconnected() <-chan websocketx.CloseInfo
	Closed() <-chan websocketx.CloseInfo
	Error() <-chan error
	Message() <-chan model.Message
	OldMessages() <-chan model.OldMessages
	Reaction() <-chan model.Reaction
	OldReactions() <-chan model.OldReactions
	Typing() <-chan model.Typing
	// Zalo may sent duplicate delivered messages, based on number of online devices of recipients
	DeliveredMessages() <-chan []model.DeliveredMessage
	SeenMessages() <-chan []model.SeenMessage
	Undo() <-chan model.Undo
	UploadAttachment() <-chan model.UploadAttachment
	Friend() <-chan model.FriendEvent
	Group() <-chan model.GroupEvent
	CipherKey() <-chan string
	// Conversations read on another device of this account (cmd 504/524).
	UnreadCleared() <-chan []model.ClearUnread
	// Conversations pinned/unpinned in the sidebar (cmd 601 "pinned_convers").
	PinConversations() <-chan model.PinConversationsUpdate
	// Conversations marked/unmarked as unread on another device (cmd 601 "mark_unread").
	UnreadMark() <-chan model.UnreadMarkUpdate
	// Call signaling, raw (cmd 601 "voip").
	Voip() <-chan model.VoipEvent
	// Frames/controls with no handler (diagnostics; full channel drops, see UnhandledFrames).
	Unhandled() <-chan RawFrame

	SendWS(ctx context.Context, payload WSPayload, requireID bool) error

	RequestOldMessages(ctx context.Context, threadType model.ThreadType, lastMsgID *string) error
	RequestOldReactions(ctx context.Context, threadType model.ThreadType, lastMsgID *string) error
}

type listener struct {
	mu sync.RWMutex

	ch    channels
	reqID uint64

	client websocketx.Client
	sc     session.MutableContext

	urls      []string
	wsURL     string
	userAgent string

	retryStates map[string]*retryState
	// fallbackRetries counts consecutive reconnect attempts per close code for
	// codes the server settings carry no schedule for (H10). Reset alongside
	// retryStates on every healthy connection.
	fallbackRetries map[int]int
	rotateCount     int
	// stableAfter: a connection must stay up this long after the cipher key
	// before the retry counters are cleared.
	stableAfter time.Duration

	cipherKey string

	selfListen  bool
	pingStopper *func()

	// rootCtx is the caller's context captured on the FIRST Start. Every
	// reconnect derives its per-attempt context from rootCtx (not from the
	// dying attempt's context), so the chain never grows unbounded and a new
	// attempt's context is never a child of the one we are about to cancel.
	rootCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

var _ Listener = (*listener)(nil)

func New(sc session.MutableContext, urls []string) (*listener, error) {
	if err := validateInputs(sc, urls); err != nil {
		return nil, err
	}

	wsURL, err := buildWebSocketURL(sc, urls[0])
	if err != nil {
		return nil, err
	}

	retryStates := buildRetryStates(sc)
	channels := initializeChannels()

	return &listener{
		sc: sc,

		ch:    channels,
		reqID: 0,

		urls:      urls,
		wsURL:     wsURL,
		userAgent: sc.UserAgent(),

		retryStates:     retryStates,
		fallbackRetries: make(map[int]int),
		rotateCount:     0,
		stableAfter:     30 * time.Second,

		cipherKey:   "",
		selfListen:  sc.Options().SelfListen,
		pingStopper: nil,
	}, nil
}

func (ln *listener) Start(ctx context.Context, retryOnClose bool) error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.client != nil {
		return errs.NewZCA("Already started", "listener.Start")
	}
	if ctx.Err() != nil {
		err := ctx.Err()
		return errs.WrapZCA("context cancelled", "listener.Start", err)
	}

	// Capture the caller's context once, on the first successful entry. Every
	// per-attempt context (this one and every reconnect) is derived from
	// rootCtx so the cancel-func chain never grows and we can safely cancel a
	// previous attempt's context without affecting the new one.
	if ln.rootCtx == nil {
		ln.rootCtx = ctx
	}

	// Cancel the PREVIOUS attempt's context before installing the new one.
	// This is safe because lctx below derives from ln.rootCtx, not from the
	// context we are cancelling here — so there is no use-after-cancel and no
	// leaked cancel func on reconnect. On the very first Start ln.cancel is
	// nil, making this a no-op.
	if ln.cancel != nil {
		ln.cancel()
		ln.cancel = nil
	}

	lctx, cancel := context.WithCancel(ln.rootCtx)
	ln.cancel = cancel

	client, err := ln.createWebSocketConnection(lctx)
	if err != nil {
		cancel()
		ln.cancel = nil
		return err
	}

	ln.client = client

	select {
	case ln.ch.Connected <- struct{}{}:
	default:
	}

	ln.wg.Add(1)
	go ln.run(lctx, retryOnClose)

	return nil
}

func (ln *listener) createWebSocketConnection(ctx context.Context) (websocketx.Client, error) {
	u, err := url.Parse(ln.wsURL)
	if err != nil {
		return nil, errs.WrapZCA("parse websocket URL failed", "listener.createWebSocketConnection", err)
	}

	// DIAG QUYẾT ĐỊNH (zpw_sek): trong giao thức Zalo Web, `zpw_sek` là QUERY PARAM nằm SẴN trong
	// URL `zpw_ws` server trả lúc login — KHÔNG phải cookie. Log TÊN các query param (không log
	// giá trị) + có zpw_sek trong query hay không. Khi chạy bot THẬT:
	//   has_zpw_sek_query=true  → URL đúng; 600 = phiên bị đá/hết hạn (dual-login) → vấn đề vận hành.
	//   has_zpw_sek_query=false → zpw_sek KHÔNG tới WS qua query → cần bổ sung (field JSON riêng /
	//                             parse khác zca-js); việc gom-cookie zpw_sek là CHỮA SAI BỆNH.
	{
		q := u.Query()
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		logger.Log(ln.sc).Infof("ws-dial url: has_zpw_sek_query=%v query_keys=%v host=%s",
			q.Has("zpw_sek"), keys, u.Host)
	}

	h := make(http.Header)
	// KHÔNG gửi Accept-Encoding trên request upgrade WS — browser thật không gửi
	// header này khi upgrade; gửi vào ⇒ edge Zalo có thể nén CẢ STREAM sau upgrade
	// ⇒ frame WS nằm trong dữ liệu nén → reader desync ("unexpected rsv bits" đủ
	// tổ hợp + "unknown opcode" + close 1011 lặp vô hạn).
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Cache-Control", "no-cache")
	h.Set("Host", u.Host)
	h.Set("Origin", config.DefaultURL.String())
	h.Set("Pragma", "no-cache")
	h.Set("User-Agent", ln.userAgent)

	// FIX (Zalo): set Cookie header TƯỜNG MINH như zca-js ("cookie: this.cookie").
	// zpw_sek nằm trong cookie set ở chat.zalo.me lúc login; nếu endpoint WS khác
	// domain thì cookie jar (lọc theo domain) KHÔNG gửi → Zalo trả error_code 600
	// "zpw_sek not found or invalid" → đóng WS → close 1011 → reconnect lặp. Gom
	// cookie của CẢ host WS lẫn chat.zalo.me, dedupe theo tên, set thẳng vào header.
	if jar := ln.sc.CookieJar(); jar != nil {
		seen := make(map[string]bool)
		var parts []string
		add := func(cks []*http.Cookie) {
			for _, ck := range cks {
				if seen[ck.Name] {
					continue
				}
				seen[ck.Name] = true
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
		base := config.DefaultURL
		add(jar.Cookies(u))     // host WS hiện tại (ln.wsURL)
		add(jar.Cookies(&base)) // chat.zalo.me (nơi login set zpw_sek)
		// MỞ RỘNG: gom cookie cho MỌI host ws ứng viên (zpw_ws có thể trả nhiều endpoint khác
		// domain/subdomain; jar lọc theo domain nên cookie set ở host khác sẽ thiếu nếu chỉ lấy
		// host[0]). Bao phủ trường hợp zpw_sek set với Domain riêng của endpoint ws.
		for _, raw := range ln.urls {
			if pu, perr := url.Parse(raw); perr == nil {
				add(jar.Cookies(pu))
			}
		}
		if len(parts) > 0 {
			h.Set("Cookie", strings.Join(parts, "; "))
		}
		// DIAG (giúp xác nhận root-cause trên BOT THẬT): server WS Zalo trả error_code 600
		// "zpw_sek not found or invalid" khi cookie phiên không tới. Log rõ zpw_sek CÓ trong
		// header gửi đi hay không — thiếu ⇒ vấn đề ở tầng login/jar; CÓ mà vẫn 600 ⇒ phiên bị
		// đá (dual-login) / hết hạn. KHÔNG log giá trị (nhạy cảm), chỉ log present/absent.
		logger.Log(ln.sc).Infof("ws-dial cookie: zpw_sek_present=%v total_cookies=%d host=%s",
			seen["zpw_sek"], len(parts), u.Host)
	}

	client, err := websocketx.Dial(ctx, ln.wsURL, &websocketx.Options{
		Header:     h,
		HTTPClient: ln.sc.Client(),
		// Server WS Zalo nén permessage-deflate vô điều kiện nhưng không echo
		// extension trong handshake (đã xác nhận bằng production log) — phải ép
		// giải nén chiều đọc, nếu không listener chết vòng lặp "unexpected rsv
		// bits" → close 1011 → reconnect. Xem internal/coderws (PATCH FORK).
		ForceDeflate: true,
		Logf: func(f string, a ...any) {
			logger.Log(ln.sc).Infof(f, a...)
		},
	})
	if err != nil {
		return nil, errs.WrapZCA("websocket dial failed", "listener.createWebSocketConnection", err)
	}

	return client, nil
}

func (ln *listener) run(ctx context.Context, retryOnClose bool) {
	defer ln.wg.Done()

	// A malformed frame from the network must never kill the host process:
	// convert any panic in the read/route path into an Error event plus a clean
	// close instead of crashing.
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		const reason = "panic recovered in listener run loop"
		ln.emitError(ctx, errs.NewZCA(fmt.Sprintf("%s: %v", reason, r), "listener.run"))

		if cl := ln.getClient(); cl != nil {
			cl.Close(ZaloAbnormalClosure, reason)
		}
		ln.reset()
		ln.emitClosed(ctx, CloseInfo{Code: ZaloAbnormalClosure, Reason: reason})
		ln.cancelActiveContext()
	}()

	cl := ln.getClient()
	if cl == nil {
		return
	}

	errsCh := cl.Errors()
	msgsCh := cl.Messages()
	closCh := cl.Closed()
	open := 3

	for open > 0 {
		select {
		case <-ctx.Done():
			return

		case err, ok := <-errsCh:
			if !ok {
				errsCh = nil
				open--
				continue
			}
			ln.emitError(ctx, err)

		case msg, ok := <-msgsCh:
			if !ok {
				msgsCh = nil
				open--
				continue
			}
			ln.handleWebSocketMessage(ctx, msg)

		case ci, ok := <-closCh:
			if !ok {
				closCh = nil
				open--
				continue
			}
			ln.handleConnectionClose(ctx, ci, retryOnClose)
			return
		}
	}
}

func (ln *listener) handleConnectionClose(ctx context.Context, ci websocketx.CloseInfo, retryOnClose bool) {
	ln.reset()

	select {
	case ln.ch.Disconnected <- ci:
	case <-ctx.Done():
		return
	default:
	}

	if delay, ok := ln.shouldRetryConnection(ctx, ci, retryOnClose); ok {
		if err := ln.scheduleReconnection(ctx, ci, delay); err != nil {
			ln.emitError(ctx, errs.WrapZCA("failed to schedule reconnection:", "listener.handleConnectionClose", err))
			ln.emitClosed(ctx, ci)
			ln.cancelActiveContext()
		}
		return
	}

	ln.emitClosed(ctx, ci)
	ln.cancelActiveContext()
}

func (ln *listener) Stop() {
	client := ln.getClient()

	// Always cancel: during a retry backoff client is nil but the pending
	// reconnect waits on this context, and must not revive a stopped listener.
	ln.cancelActiveContext()
	if client == nil {
		return
	}

	client.Close(ZaloManualClosure, "")

	ln.wg.Wait()
}

func (ln *listener) reset() {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.pingStopper != nil {
		(*ln.pingStopper)()
		ln.pingStopper = nil
	}

	if ln.client != nil {
		ln.client = nil
	}

	ln.reqID = 0
	ln.cipherKey = ""
}

func (ln *listener) cancelActiveContext() {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if ln.cancel != nil {
		ln.cancel()
		ln.cancel = nil
	}
}

func (ln *listener) getClient() websocketx.Client {
	ln.mu.RLock()
	defer ln.mu.RUnlock()
	return ln.client
}

// ----------------------------------------
// Constructor helpers
// ----------------------------------------

func validateInputs(sc session.MutableContext, urls []string) error {
	if sc == nil {
		return errs.NewZCA("context is nil", "listener.validateInputs")
	}
	if sc.CookieJar() == nil {
		return errs.NewZCA("cookie jar is not available", "listener.validateInputs")
	}
	if ua := sc.UserAgent(); ua == "" {
		return errs.NewZCA("user-agent is not available", "listener.validateInputs")
	}
	if len(urls) == 0 || urls[0] == "" {
		return errs.NewZCA("websocket URL list is empty", "listener.validateInputs")
	}
	return nil
}

func buildWebSocketURL(sc session.MutableContext, url string) (string, error) {
	wsURL := httpx.MakeURL(sc, url, map[string]any{
		"t": time.Now().UnixMilli(),
	}, true)
	if wsURL == "" {
		return "", errs.NewZCA("build websocket URL failed", "listener.buildWebSocketURL")
	}
	return wsURL, nil
}

func buildRetryStates(sc session.MutableContext) map[string]*retryState {
	retryStates := make(map[string]*retryState, 8)
	if s := sc.Settings(); s != nil && s.Features.Socket.Retries != nil {
		for reason, cfg := range s.Features.Socket.Retries {
			max := 0
			if cfg.Max != nil {
				max = *cfg.Max
			}

			times := cfg.Times.Slice()
			if len(times) == 0 {
				continue
			}
			retryStates[reason] = &retryState{
				count: 0,
				max:   max,
				times: append([]int(nil), times...),
			}
		}
	}
	return retryStates
}

func initializeChannels() channels {
	buf := defaultBuffers()
	return channels{
		Connected:         make(chan struct{}, buf.Connected),
		Disconnected:      make(chan websocketx.CloseInfo, buf.Disconnected),
		Closed:            make(chan websocketx.CloseInfo, buf.Closed),
		Error:             make(chan error, buf.Error),
		Message:           make(chan model.Message, buf.Message),
		OldMessages:       make(chan model.OldMessages, buf.OldMessages),
		Reaction:          make(chan model.Reaction, buf.Reaction),
		OldReactions:      make(chan model.OldReactions, buf.OldReactions),
		Typing:            make(chan model.Typing, buf.Typing),
		DeliveredMessages: make(chan []model.DeliveredMessage, buf.DeliveredMessages),
		SeenMessages:      make(chan []model.SeenMessage, buf.SeenMessages),
		Undo:              make(chan model.Undo, buf.Undo),
		UploadAttachment:  make(chan model.UploadAttachment, buf.UploadAttachment),
		Friend:            make(chan model.FriendEvent, buf.Friend),
		Group:             make(chan model.GroupEvent, buf.Group),
		CipherKey:         make(chan string, buf.CipherKey),
		UnreadCleared:     make(chan []model.ClearUnread, buf.UnreadCleared),
		PinConversations:  make(chan model.PinConversationsUpdate, buf.PinConversations),
		UnreadMark:        make(chan model.UnreadMarkUpdate, buf.UnreadMark),
		Voip:              make(chan model.VoipEvent, buf.Voip),
		Unhandled:         make(chan RawFrame, buf.Unhandled),
	}
}
