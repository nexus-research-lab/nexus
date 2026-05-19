package conversation

import (
	"context"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// UnifiedRequest 表示统一会话入站请求。
type UnifiedRequest struct {
	SessionKey        string
	AgentID           string
	RoomID            string
	ConversationID    string
	Content           string
	RoundID           string
	ReqID             string
	PermissionMode    sdkpermission.Mode
	PermissionHandler sdkpermission.Handler
}

// UnifiedInterruptRequest 表示统一中断请求。
type UnifiedInterruptRequest struct {
	SessionKey string
	RoundID    string
	MsgID      string
}

// DMHandler 定义 DM 侧统一处理能力。
type DMHandler interface {
	HandleDM(context.Context, UnifiedRequest) error
	HandleDMInterrupt(context.Context, UnifiedInterruptRequest) error
}

// RoomHandler 定义 Room 侧统一处理能力。
type RoomHandler interface {
	HandleRoom(context.Context, UnifiedRequest) error
	HandleRoomInterrupt(context.Context, UnifiedInterruptRequest) error
}
