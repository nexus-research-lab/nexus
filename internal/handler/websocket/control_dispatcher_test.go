package websocket

import (
	"context"
	"testing"
	"time"
)

func TestControlMessageDispatcherInterruptBypassesBlockedControl(t *testing.T) {
	dispatcher := newControlMessageDispatcher(context.Background())
	defer dispatcher.close()

	controlStarted := make(chan struct{})
	interruptFinished := make(chan struct{})
	releaseControl := make(chan struct{})

	dispatcher.enqueueJob(
		&controlMessage{msgType: "chat"},
		func() {
			close(controlStarted)
			<-releaseControl
		},
	)
	select {
	case <-controlStarted:
	case <-time.After(time.Second):
		t.Fatal("普通控制消息未开始执行")
	}

	dispatcher.enqueueJob(
		&controlMessage{msgType: "interrupt"},
		func() { close(interruptFinished) },
	)
	select {
	case <-interruptFinished:
	case <-time.After(time.Second):
		t.Fatal("interrupt 被阻塞的普通控制消息阻塞")
	}

	close(releaseControl)
}
