// INPUT: owner-scoped exact preview、来源 DM/Room scope 与 coordinator Agent。
// OUTPUT: 在独立目录隐藏内部 DM Session 中运行的 HiddenFromUser + Synthetic Agent round。
// POS: HTTP 草图确认到隔离 runtime 的宿主路由；不 fork/续写来源 transcript，实际持久化仍由 round 内 Skill + CLI 完成。
package server

import (
	"context"
	"errors"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

const workGraphSaveRoundPurpose = "workgraph_distillation"

type workGraphDMRoundDispatcher interface {
	HandleChat(context.Context, dmsvc.Request) error
}

type workGraphTransientSessionCreator interface {
	CreateTransientSession(context.Context, dmsvc.TransientSessionRequest) (*protocol.Session, error)
}

type workGraphSaveRoundDispatcher struct {
	dm       workGraphDMRoundDispatcher
	sessions workGraphTransientSessionCreator
}

func newWorkGraphSaveRoundDispatcher(
	dm interface {
		workGraphDMRoundDispatcher
		workGraphTransientSessionCreator
	},
) *workGraphSaveRoundDispatcher {
	return &workGraphSaveRoundDispatcher{dm: dm, sessions: dm}
}

func (d *workGraphSaveRoundDispatcher) DispatchWorkGraphSave(
	ctx context.Context,
	request workgraphworkflowsvc.SaveRoundRequest,
) error {
	request.OwnerUserID = strings.TrimSpace(request.OwnerUserID)
	request.AgentID = strings.TrimSpace(request.AgentID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.OwnerUserID == "" || request.AgentID == "" || request.SourceSessionKey == "" || request.PreviewID == "" || request.Prompt == "" {
		return errors.New("workgraph background save request is incomplete")
	}
	parsedSource := protocol.ParseSessionKey(request.SourceSessionKey)
	if !parsedSource.IsStructured {
		return protocol.StructuredSessionKeyError{Message: "workgraph background save requires a structured session_key"}
	}
	if d == nil || d.dm == nil || d.sessions == nil {
		return errors.New("WorkGraph isolated background save dispatcher is unavailable")
	}
	ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: request.OwnerUserID,
		Role:   authctx.RoleOwner,
	})
	runtimeSessionKey := protocol.BuildAgentSessionKey(
		request.AgentID,
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		request.PreviewID,
		"",
	)
	if _, err := d.sessions.CreateTransientSession(ownerCtx, dmsvc.TransientSessionRequest{
		AgentID:          request.AgentID,
		TargetSessionKey: runtimeSessionKey,
		Purpose:          protocol.SessionPurposeWorkGraphDistillation,
		Title:            "保存工作图草图",
	}); err != nil {
		return err
	}
	inputOptions := sdkprotocol.OutboundMessageOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        workGraphSaveRoundPurpose,
		Priority:       "internal",
		Metadata: map[string]string{
			"preview_id": request.PreviewID,
		},
	}
	return d.dm.HandleChat(ownerCtx, dmsvc.Request{
		SessionKey:                    runtimeSessionKey,
		AgentID:                       request.AgentID,
		Content:                       request.Prompt,
		Internal:                      true,
		ExecutionOrigin:               workGraphSaveRoundPurpose,
		WorkGraphSaveSourceSessionKey: request.SourceSessionKey,
		InputOptions:                  inputOptions,
	})
}
