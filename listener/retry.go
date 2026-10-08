package listener

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/logger"
	"github.com/luannt2002/zalo-api-unofficial/internal/websocketx"
)

// Fallback reconnect policy used when the server settings carry no retry
// schedule for a close code: exponential backoff with full jitter, capped at
// fallbackRetryMaxDelayMS per attempt and unlimited attempts. Counters are
// reset on every healthy connection (cipher key received), so the budget can
// never be exhausted by transient blips accumulated over weeks.
const (
	fallbackRetryBaseDelayMS = 1_000  // 1s
	fallbackRetryMaxDelayMS  = 60_000 // 60s
)

type retryState struct {
	count int
	max   int
	times []int
}

// next advances the server-provided schedule and returns the base delay (ms)
// for this attempt, or false when the schedule is exhausted or unusable.
func (st *retryState) next() (int, bool) {
	if st == nil || st.max == 0 || len(st.times) == 0 || st.count >= st.max {
		return 0, false
	}

	idx := st.count
	st.count++

	if idx < len(st.times) {
		return st.times[idx], true
	}
	return st.times[len(st.times)-1], true
}

func (ln *listener) shouldRetryConnection(ctx context.Context, ci websocketx.CloseInfo, retryOnClose bool) (int, bool) {
	if !retryOnClose || ctx.Err() != nil {
		return 0, false
	}

	return ln.canRetry(ci.Code)
}

func (ln *listener) scheduleReconnection(ctx context.Context, ci websocketx.CloseInfo, delay int) error {
	if ln.shouldRotate(ci.Code) {
		if err := ln.rotateEndpoint(); err != nil {
			return err
		}
	}

	time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		// ctx is the DYING attempt's context. Honoring it here means a Stop()
		// (which cancels the active context) during the backoff window aborts
		// the pending reconnect instead of resurrecting a stopped listener.
		// Start itself re-derives the new attempt's context from ln.rootCtx,
		// so passing ctx only gates the start; it is never the parent of the
		// new context.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := ln.Start(ctx, true); err != nil {
			select {
			case ln.ch.Closed <- ci:
			default:
			}
		}
	})

	return nil
}

func (ln *listener) canRetry(code int) (int, bool) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	// Server-provided schedule for this close code, with jitter on top so a
	// fleet of bots does not reconnect in lockstep.
	if st, ok := ln.retryStates[fmt.Sprint(code)]; ok && st != nil {
		base, retry := st.next()
		if !retry {
			return 0, false
		}
		delay := equalJitterMS(base)

		logger.Log(ln.sc).Verbosef(
			"Retry for code %d in %dms (%d/%d)",
			code, delay, st.count, st.max,
		)
		return delay, true
	}

	// Fallback: the server returned no usable retry schedule for this code.
	// Reconnect anyway with default exponential backoff + full jitter, so the
	// listener never depends 100% on server settings to stay alive.
	var closeAndRetry []int
	if s := ln.sc.Settings(); s != nil {
		closeAndRetry = s.Features.Socket.CloseAndRetry
	}
	if !fallbackRetryable(code, closeAndRetry) {
		return 0, false
	}

	attempt := ln.fallbackRetries[code]
	ln.fallbackRetries[code]++
	delay := fullJitterMS(attempt, fallbackRetryBaseDelayMS, fallbackRetryMaxDelayMS)

	logger.Log(ln.sc).Verbosef(
		"Fallback retry for code %d in %dms (attempt %d)",
		code, delay, attempt+1,
	)
	return delay, true
}

// resetRetryCounters clears every accumulated retry/rotation counter. Called
// once a connection is confirmed healthy (cipher key received), so the retry
// budget only ever applies to consecutive failures, never to blips spread
// across the whole process lifetime.
func (ln *listener) resetRetryCounters() {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	for _, st := range ln.retryStates {
		if st != nil {
			st.count = 0
		}
	}
	clear(ln.fallbackRetries)
	ln.rotateCount = 0
}

// fallbackRetryable reports whether a close code should be retried when the
// server settings provide no retry schedule for it. Deliberate closures
// (manual stop, duplicate connection, kick) are never retried; abnormal
// closure (network blip) always is. For other codes the server CloseAndRetry
// list is honored when present, otherwise retry by default.
func fallbackRetryable(code int, closeAndRetry []int) bool {
	switch code {
	case ZaloManualClosure, ZaloDuplicateConnection, ZaloKickConnection:
		return false
	case ZaloAbnormalClosure:
		return true
	}

	if len(closeAndRetry) > 0 {
		return slices.Contains(closeAndRetry, code)
	}
	return true
}

// fullJitterMS returns a delay in (0, min(capMS, baseMS<<attempt)] —
// exponential backoff with full jitter.
func fullJitterMS(attempt, baseMS, capMS int) int {
	if baseMS <= 0 {
		baseMS = fallbackRetryBaseDelayMS
	}
	if capMS < baseMS {
		capMS = baseMS
	}
	if attempt < 0 {
		attempt = 0
	}
	// Clamp the shift so the doubling can never overflow; 1s<<20 (~17min)
	// already exceeds any sane cap.
	if attempt > 20 {
		attempt = 20
	}

	d := baseMS << attempt
	if d <= 0 || d > capMS {
		d = capMS
	}
	return rand.IntN(d) + 1
}

// equalJitterMS spreads a configured delay over [d/2, d] so simultaneous
// reconnects desynchronize while still honoring the server-provided pacing.
func equalJitterMS(d int) int {
	if d <= 1 {
		return d
	}
	half := d / 2
	return half + rand.IntN(d-half+1)
}

func (ln *listener) shouldRotate(code int) bool {
	s := ln.sc.Settings()
	if s == nil || s.Features.Socket.RotateErrorCodes == nil {
		return false
	}

	if slices.Contains(s.Features.Socket.RotateErrorCodes, code) {
		return ln.rotateCount < len(ln.urls)-1
	}

	return false
}

func (ln *listener) rotateEndpoint() error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.rotateCount++
	wsURL := httpx.MakeURL(ln.sc, ln.urls[ln.rotateCount], map[string]any{
		"t": time.Now().UnixMilli(),
	}, true)
	if wsURL == "" {
		return errs.NewZCA("build websocket URL failed", "listener.rotateEndpoint")
	}
	ln.wsURL = wsURL
	logger.Log(ln.sc).Verbosef(`Rotating endpoint to %s`, ln.wsURL)

	return nil
}
