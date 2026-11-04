// INPUT: 已提交目录版本、即时目录通知原因与跨进程观察器的目录变化。
// OUTPUT: 即时通知覆盖的版本记录、带版本的观察器通知与订阅核对信号。
// POS: 目录失效 transport 边界；保证同一 mutation 只令客户端重读一次目录。
package websocket

import (
	"context"
	"strings"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
)

const (
	// DirectoryReasonSubscribed 是订阅建立后的核对信号；客户端首连可按版本跳过，重连必须重读。
	DirectoryReasonSubscribed = "app_events_subscribed"
	// directoryReasonRevisionChanged 只令目录重读；agent_ 前缀的原因同时令 Agent 列表重读。
	directoryReasonRevisionChanged      = "directory_revision_changed"
	directoryReasonAgentRevisionChanged = "agent_directory_revision_changed"
	directoryVersionReadTimeout         = 2 * time.Second
)

// DirectoryCoverage 暴露即时通知覆盖记录，供跨进程观察器跳过已通知的版本。
func (h *Handler) DirectoryCoverage() *roompkg.DirectoryCoverage {
	return &h.directoryCoverage
}

// BroadcastObservedDirectoryChange 发送观察器发现的未覆盖目录变化，携带版本供客户端去重。
func (h *Handler) BroadcastObservedDirectoryChange(ctx context.Context, change roompkg.DirectoryChange) {
	if h.appEventSubs == nil {
		return
	}
	reason := directoryReasonRevisionChanged
	if change.Agents {
		reason = directoryReasonAgentRevisionChanged
	}
	h.appEventSubs.Broadcast(ctx, protocol.NewEvent(protocol.EventTypeDirectoryChanged, map[string]any{
		"reason":             reason,
		"directory_revision": change.Version.Directory,
	}))
}

// coverDirectoryVersion 在即时通知发出前读取已提交版本：mutation 已提交，客户端收到后重读的数据不早于该版本。
// 读取失败时不记录覆盖，观察器可能补发一次重复通知，但不会漏发；无订阅者时之后的订阅核对会重新读取。
func (h *Handler) coverDirectoryVersion(ctx context.Context, reason string) {
	if h.roomService == nil || h.appEventSubs.Empty() {
		return
	}
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), directoryVersionReadTimeout)
	defer cancel()
	version, err := h.roomService.DirectoryVersion(readCtx)
	if err != nil {
		return
	}
	h.directoryCoverage.Cover(version, strings.HasPrefix(reason, "agent_"))
}

// sendDirectorySubscriptionReconcile 在订阅注册后发送核对信号，并附带当前版本供首屏去重。
func (h *Handler) sendDirectorySubscriptionReconcile(ctx context.Context, sender *handlershared.WebSocketSender) {
	payload := map[string]any{"reason": DirectoryReasonSubscribed}
	if h.roomService != nil {
		readCtx, cancel := context.WithTimeout(ctx, directoryVersionReadTimeout)
		version, err := h.roomService.DirectoryVersion(readCtx)
		cancel()
		if err == nil {
			payload["directory_revision"] = version.Directory
		}
	}
	_ = sender.SendEvent(ctx, protocol.NewEvent(protocol.EventTypeDirectoryChanged, payload))
}
