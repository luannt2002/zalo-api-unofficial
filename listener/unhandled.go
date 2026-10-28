package listener

import (
	"encoding/json"
	"sync/atomic"
)

// RawFrame is a frame, or a cmd 601 control, the router has no handler for.
type RawFrame struct {
	Version, CMD, SubCMD uint
	ActType, Act         string          // set for cmd 601 controls and cmd 602 actions with no handler
	Data                 json.RawMessage // decoded payload; nil if it could not be decoded
}

var unhandledFrames atomic.Int64

// UnhandledFrames returns how many frames/controls (all listeners) had no handler.
func UnhandledFrames() int64 { return unhandledFrames.Load() }

// emitUnhandled counts f and offers it on Unhandled(), an optional channel.
func (ln *listener) emitUnhandled(f RawFrame) {
	unhandledFrames.Add(1)
	offer(ln.ch.Unhandled, f, &droppedOptional)
}
