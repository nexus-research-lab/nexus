package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// DeletionReconcileError 表示 session 目录已删除，但 transcript 清理未完成。
type DeletionReconcileError struct {
	cause error
}

func (e *DeletionReconcileError) Error() string {
	return fmt.Sprintf("Session 数据已删除，但关联 transcript 清理需要 reconcile: %v", e.cause)
}

func (e *DeletionReconcileError) Unwrap() error {
	return e.cause
}

// SessionDeletionCommitted 判断删除错误是否发生在 session 目录提交之后。
func SessionDeletionCommitted(err error) bool {
	var committed *DeletionReconcileError
	return errors.As(err, &committed)
}

const sessionRuntimeCloseTimeout = 3 * time.Second

// CreateSession 创建或幂等返回普通 Agent 会话。
func (s *Service) CreateSession(ctx context.Context, request CreateRequest) (*protocol.Session, error) {
	sessionKey, parsed, err := s.requireSessionKey(request.SessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 创建", ErrSessionMutationUnsupported)
	}
	if request.AgentID != "" && request.AgentID != parsed.AgentID {
		return nil, errors.New("agent_id 与 session_key 不一致")
	}

	existing, err := s.GetSession(ctx, sessionKey)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	agentValue, err := s.agentService.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	initial := normalizeSession(protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      parsed.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        cmp.Or(strings.TrimSpace(request.Title), "New Chat"),
		MessageCount: 0,
		Options:      map[string]any{},
		IsActive:     false,
	})
	initial.ConfigurationVersion = 0
	created, err := s.ownerFiles(ctx).UpsertSession(agentValue.WorkspacePath, initial)
	if errors.Is(err, workspacestore.ErrSessionConfigurationVersionConflict) {
		return s.GetMutableSession(ctx, sessionKey)
	}
	if err != nil {
		return nil, err
	}
	s.notifyDirectoryChanged(ctx, "session_created", *created)
	return created, nil
}

// UpdateSession 更新普通 Agent 会话标题。
func (s *Service) UpdateSession(ctx context.Context, rawSessionKey string, request UpdateRequest) (*protocol.Session, error) {
	item, workspacePath, parsed, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	next := closePersistedSessionMeta(normalizeSession(*item))
	if request.Title != nil {
		next.Title = cmp.Or(strings.TrimSpace(*request.Title), "New Chat")
	}
	if parsed.AgentID != "" {
		next.AgentID = parsed.AgentID
	}
	updated, err := s.ownerFiles(ctx).UpsertSessionAtVersion(
		workspacePath,
		next,
		item.ConfigurationVersion,
	)
	if err != nil {
		return nil, mapSessionStorageError(err)
	}
	if updated == nil {
		projected := s.applyRuntimeStateToSession(next)
		s.notifyDirectoryChanged(ctx, "session_updated", projected)
		return &projected, nil
	}
	projected := s.applyRuntimeStateToSession(*updated)
	s.notifyDirectoryChanged(ctx, "session_updated", projected)
	return &projected, nil
}

// UpdateSessionTitle 以最小输入更新会话标题，供跨领域服务复用。
func (s *Service) UpdateSessionTitle(ctx context.Context, rawSessionKey string, title string) (*protocol.Session, error) {
	return s.UpdateSession(ctx, rawSessionKey, UpdateRequest{Title: &title})
}

// UpdateSessionTitleAtVersion 使用 session configuration_version CAS 更新标题。
func (s *Service) UpdateSessionTitleAtVersion(
	ctx context.Context,
	rawSessionKey string,
	title string,
	expectedConfigurationVersion int64,
) (*protocol.Session, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected session configuration_version 必须大于 0")
	}
	item, workspacePath, parsed, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	if item.ConfigurationVersion != expectedConfigurationVersion {
		return nil, fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			expectedConfigurationVersion,
			item.ConfigurationVersion,
		)
	}
	next := closePersistedSessionMeta(normalizeSession(*item))
	next.Title = cmp.Or(strings.TrimSpace(title), "New Chat")
	if parsed.AgentID != "" {
		next.AgentID = parsed.AgentID
	}
	updated, err := s.ownerFiles(ctx).UpsertSessionAtVersion(
		workspacePath,
		next,
		expectedConfigurationVersion,
	)
	if err != nil {
		return nil, mapSessionStorageError(err)
	}
	projected := s.applyRuntimeStateToSession(*updated)
	s.notifyDirectoryChanged(ctx, "session_updated", projected)
	return &projected, nil
}

// DeleteSession 安全关闭运行态后删除普通 Agent 会话目录及其引用产物。
func (s *Service) DeleteSession(ctx context.Context, rawSessionKey string) error {
	return s.deleteSession(ctx, rawSessionKey, nil)
}

// DeleteSessionAtVersion 安全关闭运行态，并用 session configuration_version CAS 删除。
func (s *Service) DeleteSessionAtVersion(
	ctx context.Context,
	rawSessionKey string,
	expectedConfigurationVersion int64,
) error {
	if expectedConfigurationVersion < 1 {
		return errors.New("expected session configuration_version 必须大于 0")
	}
	return s.deleteSession(ctx, rawSessionKey, &expectedConfigurationVersion)
}

func (s *Service) deleteSession(
	ctx context.Context,
	rawSessionKey string,
	expectedConfigurationVersion *int64,
) (returnErr error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return err
	}
	item, workspacePath, _, err := s.loadMutableWorkspaceSession(ctx, sessionKey)
	if err != nil {
		return err
	}
	if workspacePath == "" || item == nil {
		return ErrSessionNotFound
	}
	if expectedConfigurationVersion != nil &&
		item.ConfigurationVersion != *expectedConfigurationVersion {
		return fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			*expectedConfigurationVersion,
			item.ConfigurationVersion,
		)
	}
	if err = s.validateExternalSessionDeletion(ctx, sessionKey, parsed); err != nil {
		return err
	}
	if s.runtime == nil {
		return errors.New("Session 删除缺少 runtime manager，不能安全确认热态已关闭")
	}
	deleteVersion := item.ConfigurationVersion
	if expectedConfigurationVersion != nil {
		deleteVersion = *expectedConfigurationVersion
	}
	files := s.ownerFiles(ctx)
	cleanupSessionIDs := protocol.SessionTranscriptIDs(*item)
	storageLease, err := files.BeginSessionDeletionWithTranscriptIDs(
		workspacePath,
		sessionKey,
		deleteVersion,
		cleanupSessionIDs,
	)
	if err != nil {
		return mapSessionStorageError(err)
	}
	runtimeLease, err := s.runtime.BeginSessionDeletion(sessionKey)
	if err != nil {
		abortErr := files.AbortSessionDeletion(storageLease)
		return errors.Join(err, abortErr)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		s.runtime.AbortSessionDeletion(runtimeLease)
		if abortErr := files.AbortSessionDeletion(storageLease); abortErr != nil {
			returnErr = errors.Join(
				returnErr,
				fmt.Errorf("撤销 Session 删除 tombstone: %w", abortErr),
			)
		}
	}()
	if err = s.runtime.CloseSession(ctx, sessionKey); err != nil &&
		!runtimectx.IsRuntimeTransportClosedError(err) {
		return fmt.Errorf("关闭 Session 运行态失败，未删除持久数据: %w", err)
	}
	item, workspacePath, _, err = s.loadMutableWorkspaceSession(ctx, sessionKey)
	if err != nil {
		return err
	}
	if item == nil || workspacePath == "" {
		return ErrSessionNotFound
	}
	if item.ConfigurationVersion != deleteVersion {
		return fmt.Errorf(
			"%w: runtime 已关闭但 session 未删除；expected=%d actual=%d",
			ErrSessionConfigurationVersionConflict,
			deleteVersion,
			item.ConfigurationVersion,
		)
	}
	deleted, err := files.CommitSessionDeletion(
		storageLease,
		deleteVersion,
	)
	if deleted {
		committed = true
	}
	if err != nil {
		if deleted {
			return &DeletionReconcileError{cause: err}
		}
		return mapSessionStorageError(err)
	}
	if !deleted {
		return ErrSessionNotFound
	}
	committed = true
	cleanupCtx := context.WithoutCancel(ctx)
	cleanupErrs := make([]error, 0)
	if s.deletion != nil {
		if cleanupErr := s.deletion.CleanupSessionReferencesPreservingTasks(
			cleanupCtx,
			authctx.OwnerUserID(ctx),
			[]string{sessionKey},
		); cleanupErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Session 引用: %w", cleanupErr))
		}
	}
	for _, transcriptSessionID := range protocol.SessionTranscriptIDs(*item) {
		if _, cleanupErr := s.ownerHistory(ctx).DeleteTranscriptSession(
			workspacePath,
			transcriptSessionID,
		); cleanupErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 transcript %s: %w", transcriptSessionID, cleanupErr))
		}
	}
	if cleanupErr := files.CompleteSessionDeletionCleanup(storageLease); cleanupErr != nil {
		cleanupErrs = append(cleanupErrs, cleanupErr)
	}
	if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
		return &DeletionReconcileError{cause: cleanupErr}
	}
	if item != nil {
		s.notifyDirectoryChanged(ctx, "session_deleted", *item)
	}
	return nil
}

func mapSessionStorageError(err error) error {
	if errors.Is(err, workspacestore.ErrSessionConfigurationVersionConflict) {
		return fmt.Errorf("%w: %v", ErrSessionConfigurationVersionConflict, err)
	}
	if errors.Is(err, workspacestore.ErrSessionDeleted) {
		return fmt.Errorf("%w: %v", ErrSessionDeleted, err)
	}
	return err
}

func (s *Service) closeSessionRuntimeForDeletion(sessionKey string) error {
	if s.runtime == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), sessionRuntimeCloseTimeout)
	err := s.runtime.CloseSession(closeCtx, sessionKey)
	cancel()
	if runtimectx.IsRuntimeTransportClosedError(err) {
		return nil
	}
	return err
}

func (s *Service) notifyDirectoryChanged(ctx context.Context, reason string, session protocol.Session) {
	if s.notifier == nil {
		return
	}
	s.notifier.NotifyDirectoryChanged(ctx, strings.TrimSpace(reason), session)
}

func (s *Service) loadMutableWorkspaceSession(ctx context.Context, rawSessionKey string) (*protocol.Session, string, protocol.SessionKey, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, "", protocol.SessionKey{}, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, "", parsed, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 修改", ErrSessionMutationUnsupported)
	}

	roomSession, err := s.repository.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
	if err != nil {
		return nil, "", parsed, err
	}
	if roomSession != nil {
		return nil, "", parsed, fmt.Errorf("%w: Room 成员会话必须通过 room/conversation 语义修改", ErrSessionMutationUnsupported)
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, "", parsed, err
	}
	item, workspacePath, err := s.ownerFiles(ctx).FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, "", parsed, err
	}
	return item, workspacePath, parsed, nil
}
