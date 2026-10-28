package listener

import (
	"encoding/json"
	"sync/atomic"
)

// RawFrame is a frame, or a cmd 601 control, the router has no handler for.
type RawFrame struct {
	Version, CMD, SubCMD uint
	ActType              string          // set for an unknown cmd 601 act_type
	Data                 json.RawMessage // decoded payload; nil if it could not be decoded
}

var unhandledFrames atomic.Int64

// UnhandledFrames returns how many frames/controls (all listeners) had no handler.
func UnhandledFrames() int64 { return unhandledFrames.Load() }

// emitUnhandled counts f and offers it on Unhandled(). That channel is optional
// diagnostics, so a full one drops f without touching DroppedEvents.
func (ln *listener) emitUnhandled(f RawFrame) {
	unhandledFrames.Add(1)
	select {
	case ln.ch.Unhandled <- f:
	default:
	}
}
