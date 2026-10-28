package listener

import (
	"context"
	"sync/atomic"

	"github.com/luannt2002/zalo-api-unofficial/internal/websocketx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type Buffers struct {
	Connected         int
	Disconnected      int
	Closed            int
	Error             int
	Message           int
	OldMessages       int
	Typing            int
	SeenMessages      int
	DeliveredMessages int
	Reaction          int
	OldReactions      int
	Undo              int
	UploadAttachment  int
	Friend            int
	Group             int
	CipherKey         int
	UnreadCleared     int
	PinConversations  int
	UnreadMark        int
}

func defaultBuffers() Buffers {
	// Kênh BỀN (Message/Reaction/Undo/Friend/Group/UploadAttachment) mang dữ liệu KHÔNG được
	// mất: emit() drop-oldest khi đầy ⇒ buffer nhỏ + consumer chậm (publish backpressure ~120ms)
	// = RỚT TIN inbound trong burst. Nới rộng để burst thực tế (1 tài khoản) không chạm trần.
	// Kênh EPHEMERAL (Typing/Seen/Delivered) giữ nhỏ — drop-oldest vô hại.
	return Buffers{
		Connected:         1,
		Disconnected:      4,
		Closed:            1,
		Error:             16,
		Message:           1024,
		OldMessages:       256,
		Typing:            8,
		SeenMessages:      16,
		DeliveredMessages: 16,
		Reaction:          512,
		OldReactions:      256,
		Undo:              256,
		UploadAttachment:  128,
		Friend:            256,
		Group:             256,
		CipherKey:         4,
		UnreadCleared:     64,
		PinConversations:  64,
		UnreadMark:        64,
	}
}

type channels struct {
	Connected         chan struct{}
	Disconnected      chan websocketx.CloseInfo
	Closed            chan websocketx.CloseInfo
	Error             chan error
	Message           chan model.Message
	OldMessages       chan model.OldMessages
	Reaction          chan model.Reaction
	OldReactions      chan model.OldReactions
	Typing            chan model.Typing
	DeliveredMessages chan []model.DeliveredMessage
	SeenMessages      chan []model.SeenMessage
	Undo              chan model.Undo
	UploadAttachment  chan model.UploadAttachment
	Friend            chan model.FriendEvent
	Group             chan model.GroupEvent
	CipherKey         chan string
	UnreadCleared     chan []model.ClearUnread
	PinConversations  chan model.PinConversationsUpdate
	UnreadMark        chan model.UnreadMarkUpdate
}

func (ln *listener) Connected() <-chan struct{}                { return ln.ch.Connected }
func (ln *listener) Disconnected() <-chan websocketx.CloseInfo { return ln.ch.Disconnected }
func (ln *listener) Closed() <-chan websocketx.CloseInfo       { return ln.ch.Closed }
func (ln *listener) Error() <-chan error                       { return ln.ch.Error }
func (ln *listener) Message() <-chan model.Message             { return ln.ch.Message }
func (ln *listener) OldMessages() <-chan model.OldMessages     { return ln.ch.OldMessages }
func (ln *listener) Reaction() <-chan model.Reaction           { return ln.ch.Reaction }
func (ln *listener) OldReactions() <-chan model.OldReactions   { return ln.ch.OldReactions }
func (ln *listener) Typing() <-chan model.Typing               { return ln.ch.Typing }

func (ln *listener) DeliveredMessages() <-chan []model.DeliveredMessage {
	return ln.ch.DeliveredMessages
}
func (ln *listener) SeenMessages() <-chan []model.SeenMessage        { return ln.ch.SeenMessages }
func (ln *listener) Undo() <-chan model.Undo                         { return ln.ch.Undo }
func (ln *listener) UploadAttachment() <-chan model.UploadAttachment { return ln.ch.UploadAttachment }
func (ln *listener) Friend() <-chan model.FriendEvent                { return ln.ch.Friend }
func (ln *listener) Group() <-chan model.GroupEvent                  { return ln.ch.Group }
func (ln *listener) CipherKey() <-chan string                        { return ln.ch.CipherKey }
func (ln *listener) UnreadCleared() <-chan []model.ClearUnread       { return ln.ch.UnreadCleared }
func (ln *listener) PinConversations() <-chan model.PinConversationsUpdate {
	return ln.ch.PinConversations
}
func (ln *listener) UnreadMark() <-chan model.UnreadMarkUpdate { return ln.ch.UnreadMark }

func (ln *listener) emitError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case ln.ch.Error <- err:
	default:
	}
}

func (ln *listener) emitClosed(ctx context.Context, ci websocketx.CloseInfo) {
	select {
	case ln.ch.Closed <- ci:
	case <-ctx.Done():
		return
	default:
	}
}

// emit attempts to deliver obj into the provided channel without blocking.
//
// Behavior:
//   - If the channel has buffer space, obj is sent immediately.
//   - If the channel is full, the oldest value in the channel is dropped
//     (non-blocking receive) and emit retries once to enqueue obj.
//   - If the second attempt also fails (channel still full), obj is dropped.
//
// This policy ensures that slow or absent receivers do not block the sender,
// at the cost of possibly overwriting or dropping messages.
func emit[T any](ctx context.Context, ch chan T, obj T) {
	select {
	case <-ctx.Done():
		return
	case ch <- obj:
	default:
		// BỎ TIN CŨ NHẤT ⇒ ĐÓ LÀ MỘT TIN BỊ MẤT, phải đếm.
		//
		// Trước đây nhánh này vứt tin không đếm, nên bộ đếm chỉ nhích khi kênh VẪN đầy sau
		// khi đã vứt — tức gần như không bao giờ. Đo được: bơm 100 tin vào kênh buffer 2,
		// consumer nhận 2, mất 98, DroppedEvents() báo 0.
		//
		// Đó là loại hỏng nguy hơn không có metric: người trực nhìn thấy 0 và tin rằng
		// không mất tin nào, trong khi đang mất 98%.
		select {
		case <-ch:
			droppedEvents.Add(1)
		default:
		}
		select {
		case ch <- obj:
		default:
			// Kênh VẪN đầy sau khi bỏ oldest ⇒ obj bị RƠI. Đếm để không mất-tin ÂM THẦM
			// (consumer chậm kéo dài / burst vượt buffer) — kiểm qua DroppedEvents().
			droppedEvents.Add(1)
		}
	}
}

// droppedEvents đếm TỔNG số sự kiện bị emit() bỏ khi kênh đầy (mọi listener). Quan sát để
// phát hiện consumer chậm/burst vượt buffer thay vì mất tin lặng lẽ.
var droppedEvents atomic.Int64

// DroppedEvents trả tổng số sự kiện đã bị bỏ do kênh đầy (giám sát rớt tin inbound).
func DroppedEvents() int64 { return droppedEvents.Load() }
