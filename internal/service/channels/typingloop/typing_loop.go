package typingloop

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	DefaultCallTimeout       = 10 * time.Second
	DefaultStartDelay        = 800 * time.Millisecond
	DefaultKeepaliveInterval = 5 * time.Second
	DefaultStopWait          = 500 * time.Millisecond
	DefaultMaxDuration       = 60 * time.Second
	DefaultMaxFailures       = 2
)

type SignalFunc func(context.Context, bool) error

type LoopOptions struct {
	StartDelay        time.Duration
	KeepaliveInterval time.Duration
	CallTimeout       time.Duration
	StopWait          time.Duration
	MaxDuration       time.Duration
	MaxFailures       int
	OnError           func(active bool, err error)
}

// Start 在慢回复时延迟打开 typing，并按 IM 平台常见 TTL 周期续租。
func Start(ctx context.Context, signal SignalFunc, options LoopOptions) func() {
	if signal == nil {
		return func() {}
	}
	options = normalizedOptions(options)
	typingCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	activeStarted := atomic.Bool{}
	stopSent := atomic.Bool{}

	sendStop := func() {
		if !activeStarted.Load() || !stopSent.CompareAndSwap(false, true) {
			return
		}
		callSignal(context.Background(), signal, options, false)
	}

	go func() {
		defer close(done)
		timer := time.NewTimer(options.StartDelay)
		defer timer.Stop()
		select {
		case <-typingCtx.Done():
			return
		case <-timer.C:
		}

		activeStarted.Store(true)
		failures := 0
		if !callSignal(typingCtx, signal, options, true) {
			failures++
			if failures >= options.MaxFailures {
				sendStop()
				return
			}
		}

		ticker := time.NewTicker(options.KeepaliveInterval)
		defer ticker.Stop()
		maxDuration := time.NewTimer(options.MaxDuration)
		defer maxDuration.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-maxDuration.C:
				sendStop()
				return
			case <-ticker.C:
				if callSignal(typingCtx, signal, options, true) {
					failures = 0
					continue
				}
				failures++
				if failures >= options.MaxFailures {
					sendStop()
					return
				}
			}
		}
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(options.StopWait):
		}
		sendStop()
	}
}

func normalizedOptions(options LoopOptions) LoopOptions {
	if options.StartDelay <= 0 {
		options.StartDelay = DefaultStartDelay
	}
	if options.KeepaliveInterval <= 0 {
		options.KeepaliveInterval = DefaultKeepaliveInterval
	}
	if options.CallTimeout <= 0 {
		options.CallTimeout = DefaultCallTimeout
	}
	if options.StopWait <= 0 {
		options.StopWait = DefaultStopWait
	}
	if options.MaxDuration <= 0 {
		options.MaxDuration = DefaultMaxDuration
	}
	if options.MaxFailures <= 0 {
		options.MaxFailures = DefaultMaxFailures
	}
	return options
}

func callSignal(ctx context.Context, signal SignalFunc, options LoopOptions, active bool) bool {
	callCtx, cancel := context.WithTimeout(ctx, options.CallTimeout)
	defer cancel()
	if err := signal(callCtx, active); err != nil {
		if active && ctx.Err() != nil {
			return true
		}
		if options.OnError != nil {
			options.OnError(active, err)
		}
		return false
	}
	return true
}
