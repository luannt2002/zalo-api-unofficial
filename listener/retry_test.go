package listener

import (
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

func newTestListener(t *testing.T, retryStates map[string]*retryState, settings *session.Settings) *listener {
	t.Helper()

	sc := session.NewContext(session.WithLogging(false))
	if settings != nil {
		sc.SealLogin(session.Seal{Settings: settings})
	}

	return &listener{
		sc:              sc,
		retryStates:     retryStates,
		fallbackRetries: make(map[int]int),
		urls:            []string{"wss://primary.example", "wss://secondary.example"},
	}
}

func TestRetryStateNext(t *testing.T) {
	t.Parallel()

	t.Run("unusable states", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name string
			st   *retryState
		}{
			{name: "nil state", st: nil},
			{name: "max zero", st: &retryState{max: 0, times: []int{100}}},
			{name: "empty times", st: &retryState{max: 3, times: nil}},
			{name: "exhausted", st: &retryState{count: 2, max: 2, times: []int{100}}},
		}
		for _, tt := range tests {
			if d, ok := tt.st.next(); ok {
				t.Errorf("%s: next() = (%d, true), want false", tt.name, d)
			}
		}
	})

	t.Run("progression repeats last delay then exhausts", func(t *testing.T) {
		t.Parallel()
		st := &retryState{max: 4, times: []int{100, 200}}
		want := []int{100, 200, 200, 200}
		for i, w := range want {
			d, ok := st.next()
			if !ok || d != w {
				t.Fatalf("attempt %d: next() = (%d, %v), want (%d, true)", i, d, ok, w)
			}
		}
		if _, ok := st.next(); ok {
			t.Fatal("next() after max attempts should be false")
		}

		// Reset (H9) re-opens the budget.
		st.count = 0
		if d, ok := st.next(); !ok || d != 100 {
			t.Fatalf("next() after reset = (%d, %v), want (100, true)", d, ok)
		}
	})
}

func TestFallbackRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		code          int
		closeAndRetry []int
		want          bool
	}{
		{name: "manual closure never retried", code: ZaloManualClosure, want: false},
		{name: "duplicate connection never retried", code: ZaloDuplicateConnection, want: false},
		{name: "kick never retried", code: ZaloKickConnection, want: false},
		{name: "manual closure not retried even if listed", code: ZaloManualClosure, closeAndRetry: []int{1000}, want: false},
		{name: "abnormal closure always retried", code: ZaloAbnormalClosure, want: true},
		{name: "abnormal closure retried even when absent from list", code: ZaloAbnormalClosure, closeAndRetry: []int{4000}, want: true},
		{name: "listed code retried", code: 4000, closeAndRetry: []int{4000, 4001}, want: true},
		{name: "unlisted code not retried when list present", code: 4002, closeAndRetry: []int{4000, 4001}, want: false},
		{name: "unknown code retried when no list", code: 4002, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fallbackRetryable(tt.code, tt.closeAndRetry); got != tt.want {
				t.Fatalf("fallbackRetryable(%d, %v) = %v, want %v", tt.code, tt.closeAndRetry, got, tt.want)
			}
		})
	}
}

func TestFullJitterMS(t *testing.T) {
	t.Parallel()

	const iterations = 200

	tests := []struct {
		name    string
		attempt int
		base    int
		cap     int
		maxWant int // delay must be in (0, maxWant]
	}{
		{name: "attempt 0", attempt: 0, base: 1000, cap: 60000, maxWant: 1000},
		{name: "attempt 3 doubles", attempt: 3, base: 1000, cap: 60000, maxWant: 8000},
		{name: "attempt 6 hits cap", attempt: 6, base: 1000, cap: 60000, maxWant: 60000},
		{name: "huge attempt clamps to cap", attempt: 500, base: 1000, cap: 60000, maxWant: 60000},
		{name: "negative attempt treated as 0", attempt: -3, base: 1000, cap: 60000, maxWant: 1000},
		{name: "non-positive base falls back to default", attempt: 0, base: 0, cap: 60000, maxWant: fallbackRetryBaseDelayMS},
		{name: "cap below base raised to base", attempt: 0, base: 1000, cap: 10, maxWant: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for range iterations {
				d := fullJitterMS(tt.attempt, tt.base, tt.cap)
				if d < 1 || d > tt.maxWant {
					t.Fatalf("fullJitterMS(%d, %d, %d) = %d, want in [1, %d]", tt.attempt, tt.base, tt.cap, d, tt.maxWant)
				}
			}
		})
	}
}

func TestEqualJitterMS(t *testing.T) {
	t.Parallel()

	const iterations = 200

	tests := []struct {
		name     string
		d        int
		min, max int
	}{
		{name: "zero passes through", d: 0, min: 0, max: 0},
		{name: "one passes through", d: 1, min: 1, max: 1},
		{name: "even delay", d: 1000, min: 500, max: 1000},
		{name: "odd delay", d: 101, min: 50, max: 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for range iterations {
				if got := equalJitterMS(tt.d); got < tt.min || got > tt.max {
					t.Fatalf("equalJitterMS(%d) = %d, want in [%d, %d]", tt.d, got, tt.min, tt.max)
				}
			}
		})
	}
}

// TestCanRetryServerScheduleWithReset covers the server-provided schedule path
// plus the H9 reset: the budget must re-open after a healthy connection.
func TestCanRetryServerScheduleWithReset(t *testing.T) {
	t.Parallel()

	ln := newTestListener(t, map[string]*retryState{
		"3001": {max: 2, times: []int{100, 200}},
	}, nil)

	assertDelay := func(step string, wantMin, wantMax int) {
		t.Helper()
		d, ok := ln.canRetry(3001)
		if !ok {
			t.Fatalf("%s: canRetry(3001) = false, want true", step)
		}
		if d < wantMin || d > wantMax {
			t.Fatalf("%s: delay = %d, want in [%d, %d]", step, d, wantMin, wantMax)
		}
	}

	assertDelay("attempt 1", 50, 100)  // equal jitter over 100
	assertDelay("attempt 2", 100, 200) // equal jitter over 200
	if _, ok := ln.canRetry(3001); ok {
		t.Fatal("attempt 3: canRetry(3001) = true, want false (budget exhausted)")
	}

	// Healthy connection (cipher key) resets all counters.
	ln.rotateCount = 1
	ln.resetRetryCounters()

	if ln.rotateCount != 0 {
		t.Fatalf("rotateCount after reset = %d, want 0", ln.rotateCount)
	}
	assertDelay("attempt 1 after reset", 50, 100)
}

// TestCanRetryFallbackWithReset covers H10: with empty server retry settings
// the listener must still reconnect using default backoff, and H9 must reset
// the fallback attempt counter too.
func TestCanRetryFallbackWithReset(t *testing.T) {
	t.Parallel()

	ln := newTestListener(t, map[string]*retryState{}, nil)

	// Non-retryable deliberate closures.
	for _, code := range []int{ZaloManualClosure, ZaloDuplicateConnection, ZaloKickConnection} {
		if _, ok := ln.canRetry(code); ok {
			t.Fatalf("canRetry(%d) = true, want false", code)
		}
	}

	// Abnormal closure retries forever with capped exponential backoff.
	for attempt := range 10 {
		d, ok := ln.canRetry(ZaloAbnormalClosure)
		if !ok {
			t.Fatalf("attempt %d: canRetry(%d) = false, want true", attempt, ZaloAbnormalClosure)
		}
		maxWant := min(fallbackRetryMaxDelayMS, fallbackRetryBaseDelayMS<<attempt)
		if d < 1 || d > maxWant {
			t.Fatalf("attempt %d: delay = %d, want in [1, %d]", attempt, d, maxWant)
		}
	}

	// Reset returns the backoff to the first step.
	ln.resetRetryCounters()
	if d, ok := ln.canRetry(ZaloAbnormalClosure); !ok || d > fallbackRetryBaseDelayMS {
		t.Fatalf("after reset: canRetry = (%d, %v), want delay <= %d", d, ok, fallbackRetryBaseDelayMS)
	}
}

// TestCanRetryFallbackHonorsCloseAndRetryList: when the server provides a
// CloseAndRetry list (but no schedule), the list gates unknown codes.
func TestCanRetryFallbackHonorsCloseAndRetryList(t *testing.T) {
	t.Parallel()

	settings := &session.Settings{}
	settings.Features.Socket.CloseAndRetry = []int{4000}
	ln := newTestListener(t, map[string]*retryState{}, settings)

	if _, ok := ln.canRetry(4000); !ok {
		t.Fatal("canRetry(4000) = false, want true (listed in CloseAndRetry)")
	}
	if _, ok := ln.canRetry(4002); ok {
		t.Fatal("canRetry(4002) = true, want false (not in CloseAndRetry)")
	}
	if _, ok := ln.canRetry(ZaloAbnormalClosure); !ok {
		t.Fatal("canRetry(1006) = false, want true (always retryable)")
	}
}
