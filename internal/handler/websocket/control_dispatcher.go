// INPUT: WebSocket connection context and validated control messages.
// OUTPUT: Ordered connection-scoped dispatch, interrupt bypass, and bounded
// detached Goal command execution that survives transport cancellation.
// POS: WebSocket transport scheduling only; DM/Room services retain the
// authoritative per-session mutation ordering and durable state transitions.
package websocket

import (
	"context"
	"sync"
	"time"
)

// controlMessageDispatcher 为每条 WebSocket 连接串行处理普通控制消息，
// 同时让 interrupt 绕过正在执行的 chat，确保停止命令能被及时消费。
type controlMessageDispatcher struct {
	ctx          context.Context
	cancel       context.CancelFunc
	queue        chan controlMessageJob
	acceptanceMu sync.Mutex
	accepting    bool
	closeOnce    sync.Once
}

type controlMessageJob struct {
	run           func()
	discard       func()
	bypass        bool
	survivesClose bool
}

func newControlMessageDispatcher(parent context.Context) *controlMessageDispatcher {
	ctx, cancel := context.WithCancel(parent)
	dispatcher := &controlMessageDispatcher{
		ctx:       ctx,
		cancel:    cancel,
		queue:     make(chan controlMessageJob, 64),
		accepting: true,
	}
	go dispatcher.run()
	return dispatcher
}

func (d *controlMessageDispatcher) enqueue(message *controlMessage) {
	d.enqueueJob(message, message.dispatch)
}

// enqueueDetached accepts an already validated Goal control command into the
// same bounded FIFO as other controls. Its context retains authenticated request
// values without connection cancellation, so a socket close neither reorders
// it nor discards a command accepted immediately before disconnect.
func (d *controlMessageDispatcher) enqueueDetached(
	message *controlMessage,
	timeout time.Duration,
) {
	d.enqueueDetachedJob(message, timeout, message.dispatch)
}

func (d *controlMessageDispatcher) enqueueDetachedJob(
	message *controlMessage,
	timeout time.Duration,
	run func(),
) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(d.ctx), timeout)
	message.ctx = ctx
	d.enqueueJobValue(controlMessageJob{
		run: func() {
			defer cancel()
			run()
		},
		discard:       cancel,
		survivesClose: true,
	})
}

func (d *controlMessageDispatcher) enqueueJob(
	message *controlMessage,
	run func(),
) {
	message.ctx = d.ctx
	d.enqueueJobValue(controlMessageJob{
		run:    run,
		bypass: message.msgType == "interrupt",
	})
}

func (d *controlMessageDispatcher) enqueueJobValue(job controlMessageJob) {
	d.acceptanceMu.Lock()
	if !d.accepting || d.ctx.Err() != nil {
		d.acceptanceMu.Unlock()
		if job.discard != nil {
			job.discard()
		}
		return
	}
	if job.bypass {
		// interrupt keeps its existing bypass behavior, but now participates in
		// the same close/enqueue acceptance fence as queued controls.
		d.acceptanceMu.Unlock()
		go job.run()
		return
	}
	select {
	case d.queue <- job:
	case <-d.ctx.Done():
		d.acceptanceMu.Unlock()
		if job.discard != nil {
			job.discard()
		}
		return
	}
	d.acceptanceMu.Unlock()
}

func (d *controlMessageDispatcher) run() {
	for job := range d.queue {
		if d.ctx.Err() != nil && !job.survivesClose {
			if job.discard != nil {
				job.discard()
			}
			continue
		}
		job.run()
	}
}

func (d *controlMessageDispatcher) close() {
	d.closeOnce.Do(func() {
		// Cancel before waiting for the acceptance lock so a producer blocked on
		// a full queue can leave its select. The lock then forms the exact fence:
		// everything already sent is drained; every later enqueue is rejected.
		d.cancel()
		d.acceptanceMu.Lock()
		d.accepting = false
		close(d.queue)
		d.acceptanceMu.Unlock()
	})
}
