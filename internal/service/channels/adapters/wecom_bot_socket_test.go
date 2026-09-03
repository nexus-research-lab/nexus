package adapters

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingWeComBotSocket struct {
	mu        sync.Mutex
	deadlines []time.Time
	writes    []any
	closed    chan struct{}
	closeOnce sync.Once
}

func newRecordingWeComBotSocket() *recordingWeComBotSocket {
	return &recordingWeComBotSocket{closed: make(chan struct{})}
}

func (s *recordingWeComBotSocket) ReadMessage() (int, []byte, error) {
	<-s.closed
	return 0, nil, errors.New("socket closed")
}

func (s *recordingWeComBotSocket) SetWriteDeadline(deadline time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadlines = append(s.deadlines, deadline)
	return nil
}

func (s *recordingWeComBotSocket) WriteJSON(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, value)
	return nil
}

func (s *recordingWeComBotSocket) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func TestWeComBotWriteFrameUsesContextBoundedDeadline(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	socket := newRecordingWeComBotSocket()
	channel.setSocket(socket, true)
	channel.writeTimeout = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ctxDeadline, _ := ctx.Deadline()

	if err := channel.writeFrame(ctx, weComBotCommandFrame{Cmd: weComBotPingCommand}, true); err != nil {
		t.Fatalf("写企业微信帧失败: %v", err)
	}
	socket.mu.Lock()
	deadlines := append([]time.Time(nil), socket.deadlines...)
	socket.mu.Unlock()
	if len(deadlines) != 2 || deadlines[0].IsZero() || !deadlines[1].IsZero() {
		t.Fatalf("企业微信写截止时间未设置并复位: %+v", deadlines)
	}
	if deadlines[0].After(ctxDeadline) {
		t.Fatalf("企业微信写截止时间越过请求 deadline: got=%v context=%v", deadlines[0], ctxDeadline)
	}
}

func TestWeComBotPingLoopClosesSocketAfterMissedPongs(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	socket := newRecordingWeComBotSocket()
	channel.setSocket(socket, true)
	channel.pingInterval = 5 * time.Millisecond
	channel.maxMissedPongs = 2
	channel.writeTimeout = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		channel.pingLoop(ctx)
		close(done)
	}()
	select {
	case <-socket.closed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("连续缺失 pong 后未关闭半开连接")
	}
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("关闭半开连接后心跳循环未退出")
	}
	socket.mu.Lock()
	writes := len(socket.writes)
	socket.mu.Unlock()
	if writes != 2 {
		t.Fatalf("达到丢失阈值前的心跳次数不正确: %d", writes)
	}
}

func TestWeComBotPongResetsMissedHeartbeatState(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	channel.setLastPingReqID("ping-1")
	channel.missedPongs = 2
	if err := channel.handleFrame(context.Background(), []byte(`{"cmd":"pong","headers":{"req_id":"ping-1"}}`)); err != nil {
		t.Fatalf("处理企业微信 pong 失败: %v", err)
	}
	channel.mu.RLock()
	lastPingReqID := channel.lastPingReqID
	missedPongs := channel.missedPongs
	channel.mu.RUnlock()
	if lastPingReqID != "" || missedPongs != 0 {
		t.Fatalf("pong 未重置心跳状态: req_id=%q missed=%d", lastPingReqID, missedPongs)
	}
}
