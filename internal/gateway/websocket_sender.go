// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：websocket_sender.go
// @Date   ：2026/04/11 00:56:00
// @Author ：leemysw
// 2026/04/11 00:56:00   Create
// =====================================================

package gateway

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var websocketSenderSeq atomic.Uint64

type websocketSender struct {
	key    string
	conn   *websocket.Conn
	mu     sync.Mutex
	closed atomic.Bool
}

func newWebSocketSender(conn *websocket.Conn) *websocketSender {
	return &websocketSender{
		key:  strconv.FormatUint(websocketSenderSeq.Add(1), 10),
		conn: conn,
	}
}

func (s *websocketSender) Key() string {
	return s.key
}

func (s *websocketSender) IsClosed() bool {
	return s.closed.Load()
}

func (s *websocketSender) MarkClosed() {
	s.closed.Store(true)
}

func (s *websocketSender) SendEvent(ctx context.Context, event protocol.EventMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return context.Canceled
	}
	return wsjson.Write(ctx, s.conn, event)
}
