package session

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SQLRepository 定义 Room Session 视图所需的 SQL 读取能力。
type SQLRepository interface {
	ListRoomSessions(context.Context, string) ([]protocol.Session, error)
	ListRoomSessionsByAgent(context.Context, string) ([]protocol.Session, error)
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*protocol.Session, error)
	UpdateRoomSessionSDKSessionID(context.Context, string, string) error
}
