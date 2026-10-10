// INPUT: durable domain reconcile callback, coalesced mutation hints and exact next deadline.
// OUTPUT: one process-local worker lifecycle without fixed high-frequency polling.
// POS: infrastructure timer/wake driver; domain services retain claim, lease and retry semantics.
package duework

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultErrorRetry    = time.Second
	defaultErrorRetryMax = 30 * time.Second
	// 远端要求的等待只在合理范围内服从，防止异常响应把 worker 挂起数小时。
	maxRetryDelay = 5 * time.Minute
)

// Result tells the driver when durable work can next become eligible.
// HasMore asks the driver to yield once and immediately continue draining.
// ResetBackoff accompanies an error from work that had been healthy for a
// while (for example a long-lived connection that finally dropped): the
// failure restarts from the minimum delay instead of inheriting old backoff.
type Result struct {
	HasMore      bool
	NextDueAt    *time.Time
	ResetBackoff bool
}

// RetryDelayer is implemented by errors carrying a server-requested minimum
// delay, such as HTTP 429/503 Retry-After. The loop waits at least that long
// and ignores Notify until the delay has elapsed.
type RetryDelayer interface {
	RetryDelay() time.Duration
}

// ReconcileFunc claims and processes one bounded slice of due durable work.
// It must remain safe under duplicate calls and concurrent process workers.
type ReconcileFunc func(context.Context, time.Time) (Result, error)

// ErrorHandler observes a failed reconcile attempt. The loop retries with a
// bounded delay and remains alive until its context is cancelled.
type ErrorHandler func(error)

// Options configures a Loop. Now exists so domain tests can share a clock; a
// production loop should normally leave it unset.
type Options struct {
	// AuditInterval 为零时不做周期审计，只响应 Notify 与 NextDueAt。
	AuditInterval time.Duration
	ErrorRetry    time.Duration
	ErrorRetryMax time.Duration
	Now           func() time.Time
	OnError       ErrorHandler
}

// Loop coalesces any number of mutation hints into a single wake. Exactly one
// goroutine may call Run; Notify is safe from any goroutine and before Run.
type Loop struct {
	wake chan struct{}

	auditInterval time.Duration
	errorRetry    time.Duration
	errorRetryMax time.Duration
	now           func() time.Time
	onError       ErrorHandler

	runMu   sync.Mutex
	running bool
}

// New constructs an idle loop. It does not start a goroutine.
func New(options Options) *Loop {
	auditInterval := options.AuditInterval
	if auditInterval < 0 {
		auditInterval = 0
	}
	errorRetry := options.ErrorRetry
	if errorRetry <= 0 {
		errorRetry = defaultErrorRetry
	}
	errorRetryMax := options.ErrorRetryMax
	if errorRetryMax <= 0 {
		errorRetryMax = defaultErrorRetryMax
	}
	if errorRetryMax < errorRetry {
		errorRetryMax = errorRetry
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Loop{
		wake:          make(chan struct{}, 1),
		auditInterval: auditInterval,
		errorRetry:    errorRetry,
		errorRetryMax: errorRetryMax,
		now:           now,
		onError:       options.OnError,
	}
}

// Notify records a lossy, coalesced process-local hint. Callers that disable
// audits must notify after every mutation that can make work eligible.
func (l *Loop) Notify() {
	if l == nil {
		return
	}
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run immediately reconciles startup state, then sleeps until a mutation,
// exact deadline, optional audit boundary or cancellation. It returns only on
// context cancellation or invalid concurrent use.
func (l *Loop) Run(ctx context.Context, reconcile ReconcileFunc) error {
	if l == nil {
		return errors.New("due work loop is nil")
	}
	if reconcile == nil {
		return errors.New("due work reconcile callback is nil")
	}
	if !l.beginRun() {
		return errors.New("due work loop is already running")
	}
	defer l.endRun()

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	errorRetry := l.errorRetry

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		l.discardPendingWake()
		now := l.now().UTC()
		result, err := reconcile(ctx, now)
		if ctx.Err() != nil {
			return nil
		}

		wait := l.auditInterval
		useTimer := wait > 0
		var holdUntil time.Time
		if err != nil {
			if l.onError != nil {
				l.onError(err)
			}
			if result.ResetBackoff {
				errorRetry = l.errorRetry
			}
			// 等量抖动：保留一半下限，另一半随机，避免远端恢复时所有实例同刻重连。
			delay := jitter(errorRetry)
			if requested := retryDelay(err); requested > 0 {
				delay = max(delay, requested)
				holdUntil = l.now().Add(delay)
				wait = delay
			} else if !useTimer || delay < wait {
				wait = delay
			}
			useTimer = true
			errorRetry = nextBackoff(errorRetry, l.errorRetryMax)
		} else if result.HasMore {
			errorRetry = l.errorRetry
			wait = 0
			useTimer = true
		} else if result.NextDueAt != nil {
			errorRetry = l.errorRetry
			untilDue := result.NextDueAt.UTC().Sub(l.now().UTC())
			if untilDue < 0 {
				untilDue = 0
			}
			if !useTimer || untilDue < wait {
				wait = untilDue
			}
			useTimer = true
		} else {
			errorRetry = l.errorRetry
		}

		var timerC <-chan time.Time
		if useTimer {
			resetTimer(timer, wait)
			timerC = timer.C
		} else {
			stopTimer(timer)
		}
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-l.wake:
				// 远端明确要求的等待期内，本地提示只合并到到期后的那一次对账。
				if l.now().Before(holdUntil) {
					continue
				}
				stopTimer(timer)
			case <-timerC:
			}
			break
		}
	}
}

// ParseRetryAfter 按 RFC 9110 解析 Retry-After 的秒数或 HTTP-date；无效或已过期返回零。
func ParseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		return max(time.Duration(seconds*float64(time.Second)), 0)
	}
	if deadline, err := http.ParseTime(value); err == nil {
		return max(time.Until(deadline), 0)
	}
	return 0
}

func retryDelay(err error) time.Duration {
	var delayer RetryDelayer
	if !errors.As(err, &delayer) {
		return 0
	}
	return min(max(delayer.RetryDelay(), 0), maxRetryDelay)
}

func jitter(delay time.Duration) time.Duration {
	if delay <= 1 {
		return delay
	}
	half := delay / 2
	return half + rand.N(delay-half+1)
}

func (l *Loop) beginRun() bool {
	l.runMu.Lock()
	defer l.runMu.Unlock()
	if l.running {
		return false
	}
	l.running = true
	return true
}

func (l *Loop) endRun() {
	l.runMu.Lock()
	l.running = false
	l.runMu.Unlock()
}

func (l *Loop) discardPendingWake() {
	select {
	case <-l.wake:
	default:
	}
}

func nextBackoff(current time.Duration, maximum time.Duration) time.Duration {
	if current >= maximum {
		return maximum
	}
	if current > maximum/2 {
		return maximum
	}
	return current * 2
}

func resetTimer(timer *time.Timer, wait time.Duration) {
	stopTimer(timer)
	timer.Reset(wait)
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
