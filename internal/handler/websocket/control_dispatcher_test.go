package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func TestControlMessageDispatcherDetachedGoalSurvivesImmediateParentCancellation(t *testing.T) {
	type authenticatedOwnerKey struct{}
	parentValueCtx := context.WithValue(
		context.Background(),
		authenticatedOwnerKey{},
		"owner-detached-goal",
	)
	parentCtx, cancelParent := context.WithCancel(parentValueCtx)
	dispatcher := newControlMessageDispatcher(parentCtx)
	defer dispatcher.close()

	started := make(chan context.Context, 1)
	finished := make(chan struct{})
	release := make(chan struct{})
	message := &controlMessage{msgType: "set_goal"}
	dispatcher.enqueueDetachedJob(message, time.Second, func() {
		started <- message.ctx
		<-release
		close(finished)
	})
	// Cancel immediately after acceptance. The detached job may keep its FIFO
	// position, but it must not inherit cancellation or be discarded.
	cancelParent()

	var jobCtx context.Context
	select {
	case jobCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("detached set_goal job was discarded after parent cancellation")
	}
	if err := jobCtx.Err(); err != nil {
		t.Fatalf("detached set_goal context inherited parent cancellation: %v", err)
	}
	if got := jobCtx.Value(authenticatedOwnerKey{}); got != "owner-detached-goal" {
		t.Fatalf("detached set_goal context owner = %#v, want preserved auth value", got)
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("detached set_goal job did not finish")
	}
}

func TestControlMessageDispatcherDetachedGoalsPreserveAcceptedOrderAfterClose(t *testing.T) {
	parentCtx, cancelParent := context.WithCancel(context.Background())
	dispatcher := newControlMessageDispatcher(parentCtx)
	defer dispatcher.close()

	ordinaryStarted := make(chan struct{})
	dispatcher.enqueueJob(
		&controlMessage{msgType: "chat"},
		func() {
			close(ordinaryStarted)
			<-parentCtx.Done()
		},
	)
	<-ordinaryStarted

	var (
		mu    sync.Mutex
		order []string
	)
	finished := make(chan struct{})
	appendOrder := func(value string) {
		mu.Lock()
		order = append(order, value)
		mu.Unlock()
	}
	dispatcher.enqueueDetachedJob(
		&controlMessage{msgType: "set_goal"},
		time.Second,
		func() { appendOrder("first") },
	)
	dispatcher.enqueueDetachedJob(
		&controlMessage{msgType: "set_goal"},
		time.Second,
		func() {
			appendOrder("second")
			close(finished)
		},
	)

	cancelParent()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("accepted detached Goals were not drained after connection close")
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(order, []string{"first", "second"}) {
		t.Fatalf("detached Goal order = %#v", order)
	}
}

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

func TestDetachedGoalFailureDeliveryOutlivesMutationContext(t *testing.T) {
	type authenticatedOwnerKey struct{}
	serverDone := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			serverDone <- err
			return
		}
		defer func() {
			_ = connection.Close(websocket.StatusNormalClosure, "test complete")
		}()

		valueCtx := context.WithValue(
			request.Context(),
			authenticatedOwnerKey{},
			"owner-detached-goal",
		)
		mutationCtx, cancelMutation := context.WithCancel(valueCtx)
		cancelMutation()
		message := &controlMessage{
			handler:            &Handler{api: handlershared.NewAPI(nil)},
			ctx:                mutationCtx,
			sender:             handlershared.NewWebSocketSender(connection),
			sessionKey:         "agent:nexus:ws:dm:detached-goal-delivery",
			msgType:            "set_goal",
			bestEffortDelivery: true,
		}
		deliveryCtx, cancelDelivery := message.deliveryContext()
		if err = deliveryCtx.Err(); err != nil {
			cancelDelivery()
			serverDone <- fmt.Errorf("delivery inherited mutation cancellation: %w", err)
			return
		}
		if got := deliveryCtx.Value(authenticatedOwnerKey{}); got != "owner-detached-goal" {
			cancelDelivery()
			serverDone <- fmt.Errorf("delivery owner = %#v", got)
			return
		}
		cancelDelivery()

		message.reportChatFailure(
			"request-detached-goal-delivery",
			"message-detached-goal-delivery",
			context.DeadlineExceeded,
		)
		serverDone <- nil
	}))
	defer server.Close()

	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRead()
	connection, _, err := websocket.Dial(
		readCtx,
		"ws"+strings.TrimPrefix(server.URL, "http"),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	var event protocol.EventMessage
	if err = wsjson.Read(readCtx, connection, &event); err != nil {
		t.Fatalf("read detached Goal terminal error: %v", err)
	}
	if event.EventType != protocol.EventTypeError ||
		event.Data["client_request_id"] != "request-detached-goal-delivery" ||
		event.Data["client_message_id"] != "message-detached-goal-delivery" {
		t.Fatalf("detached Goal terminal event = %#v", event)
	}
	if err = <-serverDone; err != nil {
		t.Fatal(err)
	}
}
