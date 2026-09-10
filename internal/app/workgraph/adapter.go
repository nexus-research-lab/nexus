// INPUT: WorkGraph 编辑请求、主智能体与来源 Session identity。
// OUTPUT: 隔离隐藏编辑 DM Session 与安全 Session 删除主链。
// POS: workgraphworkflow 与 dm/session runtime 之间的组合层适配器。
package workgraph

import (
	"context"
	"errors"

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
