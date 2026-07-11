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

type loop struct {
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	signal        SignalFunc
	options       LoopOptions
	activeStarted atomic.Bool
	stopSent      atomic.Bool
}

// Start 在慢回复时延迟打开 typing，并按 IM 平台常见 TTL 周期续租。
func Start(ctx context.Context, signal SignalFunc, options LoopOptions) func() {
	if signal == nil {
		return func() {}
	}
	loop := newLoop(ctx, signal, normalizedOptions(options))
	go loop.run()
	return loop.stop
}

func newLoop(ctx context.Context, signal SignalFunc, options LoopOptions) *loop {
	typingCtx, cancel := context.WithCancel(ctx)
	return &loop{
		ctx:     typingCtx,
		cancel:  cancel,
		done:    make(chan struct{}),
		signal:  signal,
		options: options,
	}
}

func (l *loop) run() {
	defer close(l.done)
	if !l.waitForStart() {
		return
	}
	l.activeStarted.Store(true)
	failures, keepGoing := l.sendActive(0)
	if !keepGoing {
		l.sendStop()
		return
	}
	l.runKeepalive(failures)
}

func (l *loop) waitForStart() bool {
	timer := time.NewTimer(l.options.StartDelay)
	defer timer.Stop()
	select {
	case <-l.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (l *loop) runKeepalive(failures int) {
	ticker := time.NewTicker(l.options.KeepaliveInterval)
	defer ticker.Stop()
	maxDuration := time.NewTimer(l.options.MaxDuration)
	defer maxDuration.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-maxDuration.C:
			l.sendStop()
			return
		case <-ticker.C:
			var keepGoing bool
			failures, keepGoing = l.sendActive(failures)
			if !keepGoing {
				l.sendStop()
				return
			}
		}
	}
}

func (l *loop) sendActive(failures int) (int, bool) {
	if callSignal(l.ctx, l.signal, l.options, true) {
		return 0, true
	}
	failures++
	return failures, failures < l.options.MaxFailures
}

func (l *loop) stop() {
	l.cancel()
	select {
	case <-l.done:
	case <-time.After(l.options.StopWait):
	}
	l.sendStop()
}

func (l *loop) sendStop() {
	if !l.activeStarted.Load() || !l.stopSent.CompareAndSwap(false, true) {
		return
	}
	callSignal(context.Background(), l.signal, l.options, false)
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
