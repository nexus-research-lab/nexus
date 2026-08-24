// INPUT: WorkGraph 服务签发的 Nexus 主智能体与隐藏目标 Session identity。
// OUTPUT: 不继承源 transcript 的可恢复专用 DM 与安全 Session 删除主链。
// POS: workgraphworkflow 与 dm/session 服务之间的组合层适配器。
package server

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type workGraphEditorSessionManager struct {
	dm       *dmsvc.Service
	sessions *sessionsvc.Service
}

func (m workGraphEditorSessionManager) CreateWorkGraphEditorSession(
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

func (m workGraphEditorSessionManager) DeleteWorkGraphEditorSession(
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
