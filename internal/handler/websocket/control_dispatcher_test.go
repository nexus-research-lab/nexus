package websocket

import (
	"context"
	"testing"
	"time"
)

func TestControlMessageDispatcherCloseRejectsBlockedAndLateEnqueue(t *testing.T) {
	dispatcher := newControlMessageDispatcher(context.Background())

	activeStarted := make(chan struct{})
	releaseActive := make(chan struct{})
	dispatcher.enqueueJob(
		&controlMessage{msgType: "chat"},
		func() {
			close(activeStarted)
			<-releaseActive
		},
	)
	<-activeStarted

	// Keep the consumer occupied and fill every queue slot. The next producer
	// must block inside the acceptance fence until close cancels the dispatcher.
	for range cap(dispatcher.queue) {
		dispatcher.enqueueJob(
			&controlMessage{msgType: "chat"},
			func() {},
		)
	}
	blockedDiscarded := make(chan struct{})
	blockedReturned := make(chan struct{})
	go func() {
		dispatcher.enqueueJobValue(controlMessageJob{
			run:     func() { t.Error("blocked job ran after close") },
			discard: func() { close(blockedDiscarded) },
		})
		close(blockedReturned)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		if !dispatcher.acceptanceMu.TryLock() {
			break
		}
		dispatcher.acceptanceMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("producer did not block on the full dispatcher queue")
		}
		time.Sleep(time.Millisecond)
	}

	dispatcher.close()
	// close is intentionally idempotent because handler and tests may both own
	// cleanup paths.
	dispatcher.close()
	select {
	case <-blockedDiscarded:
	case <-time.After(time.Second):
		t.Fatal("blocked enqueue was not rejected by close")
	}
	select {
	case <-blockedReturned:
	case <-time.After(time.Second):
		t.Fatal("blocked producer did not leave enqueue after close")
	}

	lateRan := make(chan struct{}, 1)
	lateDiscarded := make(chan struct{})
	dispatcher.enqueueJobValue(controlMessageJob{
		run:     func() { lateRan <- struct{}{} },
		discard: func() { close(lateDiscarded) },
	})
	select {
	case <-lateDiscarded:
	case <-time.After(time.Second):
		t.Fatal("late enqueue was not rejected")
	}
	select {
	case <-lateRan:
		t.Fatal("late enqueue ran after dispatcher close")
	default:
	}
	close(releaseActive)
}
