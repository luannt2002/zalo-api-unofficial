package listener

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/luannt2002/zalo-api-unofficial/listener/events"
)

const maxRawLen = 512

// DecodeError reports an event the listener received but could not decode. The
// event is dropped; the rest of its batch is still delivered.
type DecodeError struct {
	Op    string // handler, e.g. "listener.handleGroupMessages"
	MsgID string // "" when the item carries none
	Raw   string // raw item JSON, cut to maxRawLen bytes
	Err   error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s: dropped undecodable event msgId=%q: %v; raw=%s", e.Op, e.MsgID, e.Err, e.Raw)
}

func (e *DecodeError) Unwrap() error { return e.Err }

var decodeFailures atomic.Int64

// DecodeFailures returns how many events or frames (all listeners) were dropped
// because they could not be decoded.
func DecodeFailures() int64 { return decodeFailures.Load() }

// emitDecodeFailure counts a dropped frame or item and reports it on Error().
func (ln *listener) emitDecodeFailure(ctx context.Context, err error) {
	decodeFailures.Add(1)
	ln.emitError(ctx, err)
}

func (ln *listener) emitItemErrors(ctx context.Context, op string, items []events.ItemError) {
	for _, it := range items {
		raw := string(it.Raw)
		if len(raw) > maxRawLen {
			raw = strings.ToValidUTF8(raw[:maxRawLen], "") + "…"
		}
		ln.emitDecodeFailure(ctx, &DecodeError{Op: op, MsgID: it.MsgID, Raw: raw, Err: it.Err})
	}
}
