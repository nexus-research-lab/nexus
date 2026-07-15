package websocket

import (
	"context"
)

// controlMessageDispatcher 为每条 WebSocket 连接串行处理普通控制消息，
// 同时让 interrupt 绕过正在执行的 chat，确保停止命令能被及时消费。
type controlMessageDispatcher struct {
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan controlMessageJob
}

type controlMessageJob struct {
	run func()
}

func newControlMessageDispatcher(parent context.Context) *controlMessageDispatcher {
	ctx, cancel := context.WithCancel(parent)
	dispatcher := &controlMessageDispatcher{
		ctx:    ctx,
		cancel: cancel,
		queue:  make(chan controlMessageJob, 64),
	}
	go dispatcher.run()
	return dispatcher
}

func (d *controlMessageDispatcher) enqueue(message *controlMessage) {
	d.enqueueJob(message, message.dispatch)
}

func (d *controlMessageDispatcher) enqueueJob(
	message *controlMessage,
	run func(),
) {
	message.ctx = d.ctx
	if message.msgType == "interrupt" {
		go run()
		return
	}

	select {
	case d.queue <- controlMessageJob{run: run}:
	case <-d.ctx.Done():
	}
}

func (d *controlMessageDispatcher) run() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case job := <-d.queue:
			select {
			case <-d.ctx.Done():
				return
			default:
			}
			job.run()
		}
	}
}

func (d *controlMessageDispatcher) close() {
	d.cancel()
}
