// INPUT: durable domain reconcile callback, coalesced mutation hints and exact next deadline.
// OUTPUT: one process-local worker lifecycle without fixed high-frequency polling.
// POS: infrastructure timer/wake driver; domain services retain claim, lease and retry semantics.
package duework

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultAuditInterval = 30 * time.Second
	defaultErrorRetry    = time.Second
	defaultErrorRetryMax = 30 * time.Second
)

// Result tells the driver when durable work can next become eligible.
// HasMore asks the driver to yield once and immediately continue draining.
type Result struct {
	HasMore   bool
	NextDueAt *time.Time
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
	if auditInterval <= 0 {
		auditInterval = defaultAuditInterval
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

// Notify records a lossy, coalesced process-local hint. Durable state and the
// audit path guarantee recovery; callers therefore never block on a busy loop.
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
// exact deadline, audit boundary or cancellation. It returns only on context
// cancellation or invalid concurrent use.
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
		if err != nil {
			if l.onError != nil {
				l.onError(err)
			}
			wait = minDuration(wait, errorRetry)
			errorRetry = nextBackoff(errorRetry, l.errorRetryMax)
		} else if result.HasMore {
			errorRetry = l.errorRetry
			wait = 0
		} else if result.NextDueAt != nil {
			errorRetry = l.errorRetry
			untilDue := result.NextDueAt.UTC().Sub(l.now().UTC())
			if untilDue < 0 {
				untilDue = 0
			}
			wait = minDuration(wait, untilDue)
		} else {
			errorRetry = l.errorRetry
		}

		resetTimer(timer, wait)
		select {
		case <-ctx.Done():
			return nil
		case <-l.wake:
			stopTimer(timer)
		case <-timer.C:
		}
	}
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

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if right < left {
		return right
	}
	return left
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
