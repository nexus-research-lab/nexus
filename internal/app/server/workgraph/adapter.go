// INPUT: WorkGraph 编辑/保存请求、主智能体与来源 Session identity。
// OUTPUT: 隔离隐藏 DM Session、保存 round 与安全 Session 删除主链。
// POS: workgraphworkflow 与 dm/session runtime 之间的组合层适配器。
package workgraph

import (
	"context"
	"errors"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type editorSessionManager struct {
	dm       *dmsvc.Service
	sessions *sessionsvc.Service
}

// NewEditorSessionManager 创建 WorkGraph 隐藏编辑 Session 管理器。
func NewEditorSessionManager(
	dm *dmsvc.Service,
	sessions *sessionsvc.Service,
) *editorSessionManager {
	return &editorSessionManager{dm: dm, sessions: sessions}
}

func (m editorSessionManager) CreateWorkGraphEditorSession(
	ctx context.Context,
	request workgraphworkflowsvc.EditorSessionCreateRequest,
) (*protocol.Session, error) {
	if m.dm == nil {
		return nil, errors.New("DM service is unavailable")
	}
	return m.dm.CreateTransientSession(ctx, dmsvc.TransientSessionRequest{
		AgentID:               request.AgentID,
		TargetSessionKey:      request.TargetSessionKey,
		Purpose:               protocol.SessionPurposeWorkGraphEditor,
		Title:                 "调整草图",
		DisplayAfterUnixMilli: request.DisplayAfterUnixMilli,
	})
}

func (m editorSessionManager) DeleteWorkGraphEditorSession(
	ctx context.Context,
	sessionKey string,
) error {
	if m.sessions == nil {
		return errors.New("Session service is unavailable")
	}
	err := m.sessions.DeleteSession(ctx, sessionKey)
	if errors.Is(err, sessionsvc.ErrSessionNotFound) {
		return nil
	}
	return err
}

const saveRoundPurpose = "workgraph_distillation"

type dmRoundDispatcher interface {
	HandleChat(context.Context, dmsvc.Request) error
}

type transientSessionCreator interface {
	CreateTransientSession(context.Context, dmsvc.TransientSessionRequest) (*protocol.Session, error)
}

type saveRoundDispatcher struct {
	dm       dmRoundDispatcher
	sessions transientSessionCreator
}

// NewSaveRoundDispatcher 创建隔离的 WorkGraph 保存 round 派发器。
func NewSaveRoundDispatcher(
	dm interface {
		dmRoundDispatcher
		transientSessionCreator
	},
) *saveRoundDispatcher {
	return &saveRoundDispatcher{dm: dm, sessions: dm}
}

func (d *saveRoundDispatcher) DispatchWorkGraphSave(
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
		Purpose:        saveRoundPurpose,
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
		ExecutionOrigin:               saveRoundPurpose,
		WorkGraphSaveSourceSessionKey: request.SourceSessionKey,
		InputOptions:                  inputOptions,
	})
}
