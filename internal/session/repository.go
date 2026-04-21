// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：repository.go
// @Date   ：2026/04/11 00:13:00
// @Author ：leemysw
// 2026/04/11 00:13:00   Create
// =====================================================

package session

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SQLRepository 定义 Room Session 视图所需的 SQL 读取能力。
type SQLRepository interface {
	ListRoomSessions(context.Context, string) ([]Session, error)
	ListRoomSessionsByAgent(context.Context, string) ([]Session, error)
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}
