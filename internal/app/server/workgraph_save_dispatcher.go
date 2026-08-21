// INPUT: owner-scoped exact preview 保存请求与当前 DM/Room session key。
// OUTPUT: HiddenFromUser + Synthetic 的内部 Agent round；不写入用户聊天时间线。
// POS: HTTP 草图确认到 DM/Room runtime 的宿主路由；实际持久化仍由 round 内 Skill + CLI 完成。
package server

import (
	"context"
	"errors"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

const workGraphSaveRoundPurpose = "workgraph_distillation"

type workGraphDMRoundDispatcher interface {
	HandleChat(context.Context, dmsvc.Request) error
}

type workGraphRoomRoundDispatcher interface {
	HandleChat(context.Context, roomrealtime.ChatRequest) error
}

type workGraphSaveRoundDispatcher struct {
	dm   workGraphDMRoundDispatcher
	room workGraphRoomRoundDispatcher
}

func newWorkGraphSaveRoundDispatcher(
	dm workGraphDMRoundDispatcher,
	room workGraphRoomRoundDispatcher,
) *workGraphSaveRoundDispatcher {
	return &workGraphSaveRoundDispatcher{dm: dm, room: room}
}

func (d *workGraphSaveRoundDispatcher) DispatchWorkGraphSave(
	ctx context.Context,
	request workgraphworkflowsvc.SaveRoundRequest,
) error {
	request.OwnerUserID = strings.TrimSpace(request.OwnerUserID)
	request.SessionKey = strings.TrimSpace(request.SessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.OwnerUserID == "" || request.SessionKey == "" || request.PreviewID == "" || request.Prompt == "" {
		return errors.New("workgraph background save request is incomplete")
	}
	parsed := protocol.ParseSessionKey(request.SessionKey)
	if !parsed.IsStructured {
		return protocol.StructuredSessionKeyError{Message: "workgraph background save requires a structured session_key"}
	}
	ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: request.OwnerUserID,
		Role:   authctx.RoleOwner,
	})
	inputOptions := sdkprotocol.OutboundMessageOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        workGraphSaveRoundPurpose,
		Priority:       "internal",
		Metadata: map[string]string{
			"preview_id": request.PreviewID,
		},
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if d == nil || d.dm == nil {
			return errors.New("DM background save dispatcher is unavailable")
		}
		return d.dm.HandleChat(ownerCtx, dmsvc.Request{
			SessionKey:      request.SessionKey,
			AgentID:         parsed.AgentID,
			Content:         request.Prompt,
			Internal:        true,
			ExecutionOrigin: workGraphSaveRoundPurpose,
			InputOptions:    inputOptions,
		})
	case protocol.SessionKeyKindRoom:
		if d == nil || d.room == nil {
			return errors.New("Room background save dispatcher is unavailable")
		}
		return d.room.HandleChat(ownerCtx, roomrealtime.ChatRequest{
			SessionKey:      request.SessionKey,
			ConversationID:  parsed.ConversationID,
			Content:         request.Prompt,
			Internal:        true,
			ExecutionOrigin: workGraphSaveRoundPurpose,
			InputOptions:    inputOptions,
		})
	default:
		return errors.New("workgraph background save supports only DM or Room sessions")
	}
}
