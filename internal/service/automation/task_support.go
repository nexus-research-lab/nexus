// INPUT: owner-scoped Automation 任务、Agent workspace 与 Session artifact 删除协调器。
// OUTPUT: 任务容量校验，以及 isolated Session 的统一 tombstone/runtime/transcript 清理。
// POS: Automation CRUD 辅助阶段；禁止直接删除 Agent Session 目录。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type skipIsolatedAutomationSessionCleanupKey struct{}

func withSkippedIsolatedAutomationSessionCleanup(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipIsolatedAutomationSessionCleanupKey{}, true)
}

func skipIsolatedAutomationSessionCleanup(ctx context.Context) bool {
	value, _ := ctx.Value(skipIsolatedAutomationSessionCleanupKey{}).(bool)
	return value
}

func (s *Service) resolveTaskOwnerUserID(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.requireAgent(ctx, agentID)
		if err != nil {
			return "", err
		}
		if agentValue != nil {
			if ownerUserID := strings.TrimSpace(agentValue.OwnerUserID); ownerUserID != "" {
				return ownerUserID, nil
			}
		}
	}
	return authctx.OwnerUserID(ctx), nil
}

func (s *Service) validateTaskExpiration(expiresAt *time.Time) error {
	if expiresAt == nil {
		return nil
	}
	if !expiresAt.UTC().After(s.nowFn().UTC()) {
		return fmt.Errorf("expires_at 必须晚于当前时间")
	}
	return nil
}

func (s *Service) validateTaskCapacity(ctx context.Context, ownerUserID string, enabling bool) error {
	if !enabling {
		return nil
	}
	limit := s.config.AutomationMaxEnabledTasksPerUser
	if limit <= 0 {
		limit = 100
	}
	count, err := s.repository.CountEnabledScheduledTasks(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return fmt.Errorf("统计已启用自动化任务: %w", err)
	}
	if count >= limit {
		return fmt.Errorf("每个用户启用的定时任务不能超过 %d 个", limit)
	}
	return nil
}

func (s *Service) cleanupIsolatedAutomationSessions(ctx context.Context, job automationdomain.ScheduledTask) error {
	if strings.TrimSpace(job.SessionTarget.Kind) != automationdomain.SessionTargetIsolated {
		return nil
	}
	cleanupCtx := context.WithoutCancel(ctx)
	workspacePath, err := s.resolveAutomationWorkspacePath(cleanupCtx, job.AgentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	ownerUserID := strings.TrimSpace(job.OwnerUserID)
	if ownerUserID == "" {
		return errors.New("清理自动化会话缺少 owner_user_id")
	}
	prefix := fmt.Sprintf("agent:%s:automation:dm:scheduled-task:%s:", strings.TrimSpace(job.AgentID), strings.TrimSpace(job.JobID))
	files := workspacestore.NewSessionFileStore(s.config.WorkspacePath).ForOwner(ownerUserID)
	sessions, err := files.ListSessions(workspacePath)
	if err != nil {
		return err
	}
	targets := make([]protocol.Session, 0)
	for _, item := range sessions {
		sessionKey := strings.TrimSpace(item.SessionKey)
		if !strings.HasPrefix(sessionKey, prefix) {
			continue
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindAgent || !parsed.IsStructured || parsed.Channel != "automation" {
			continue
		}
		targets = append(targets, item)
	}
	if len(targets) == 0 {
		return nil
	}
	if s.sessionArtifacts == nil {
		return ErrSessionArtifactDeletionCoordinatorUnavailable
	}

	errs := make([]error, 0)
	for _, item := range targets {
		sessionKey := strings.TrimSpace(item.SessionKey)
		cleanupSessionID := ""
		if item.SessionID != nil {
			cleanupSessionID = strings.TrimSpace(*item.SessionID)
		}
		if deleteErr := s.sessionArtifacts.DeleteSessionArtifacts(
			cleanupCtx,
			ownerUserID,
			workspacePath,
			sessionKey,
			cleanupSessionID,
		); deleteErr != nil {
			errs = append(errs, deleteErr)
		}
	}
	return errors.Join(errs...)
}

// DeleteTasksForSessions 删除目标或来源精确绑定到 Session 的定时任务。
func (s *Service) DeleteTasksForSessions(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) error {
	keySet := make(map[string]struct{}, len(sessionKeys))
	for _, sessionKey := range sessionKeys {
		sessionKey = strings.TrimSpace(sessionKey)
		if sessionKey != "" {
			keySet[sessionKey] = struct{}{}
		}
	}
	if len(keySet) == 0 {
		return nil
	}
	items, err := s.repository.ListScheduledTasks(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return err
	}
	for _, item := range items {
		if !scheduledTaskReferencesSession(item, keySet) {
			continue
		}
		if _, err = s.DeleteTask(contextForJobOwner(ctx, item), item.JobID); err != nil {
			return err
		}
	}
	return nil
}

// CountTasksReferencingSessions 返回每个结构化 Session 被现存任务引用的数量。
// 禁用任务也计入：用户以后重新启用时仍需要原投递/执行上下文，不能因删除历史
// Session 而静默损坏配置。
func (s *Service) CountTasksReferencingSessions(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) (map[string]int, error) {
	keySet := make(map[string]struct{}, len(sessionKeys))
	result := make(map[string]int, len(sessionKeys))
	for _, sessionKey := range sessionKeys {
		sessionKey = strings.TrimSpace(sessionKey)
		if sessionKey == "" {
			continue
		}
		keySet[sessionKey] = struct{}{}
		result[sessionKey] = 0
	}
	if len(keySet) == 0 {
		return result, nil
	}
	items, err := s.repository.ListScheduledTasks(ctx, strings.TrimSpace(ownerUserID), "")
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		for sessionKey := range scheduledTaskReferencedSessionKeys(item, keySet) {
			result[sessionKey]++
		}
	}
	return result, nil
}

// InvalidateTasksForDeletedSessions 保留引用已删除 Session 的任务定义，但停止未来调度，
// 直到执行和投递目标中的全部失效绑定都被用户或 Agent 重新分配。
func (s *Service) InvalidateTasksForDeletedSessions(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.taskControlMu.Lock()
	defer s.taskControlMu.Unlock()

	ownerUserID = strings.TrimSpace(ownerUserID)
	items, err := s.repository.ListScheduledTasks(ctx, ownerUserID, "")
	if err != nil {
		return err
	}
	for _, item := range items {
		invalidated, changed := automationdomain.InvalidateScheduledTaskSessions(item, sessionKeys)
		if !changed {
			continue
		}
		invalidated.PermissionPolicy = normalizeTaskPermissionPolicy(item.PermissionPolicy)
		invalidated.PermissionPolicy.Revision++
		invalidated.PermissionState = automationdomain.TaskPermissionStateReady
		invalidated.PendingPermissionRequestID = ""
		updated, updateErr := s.repository.UpdateScheduledTaskAtVersion(
			contextForJobOwner(ctx, item),
			invalidated,
			item.ConfigurationVersion,
		)
		if updateErr != nil {
			return updateErr
		}
		pendingRequests, updateErr := s.repository.ListPermissionRequests(
			ctx,
			updated.OwnerUserID,
			automationdomain.PermissionRequestStatusPending,
			updated.JobID,
		)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = s.repository.SupersedePendingPermissionRequests(
			ctx,
			updated.OwnerUserID,
			updated.JobID,
		); updateErr != nil {
			return updateErr
		}
		for _, request := range pendingRequests {
			request.Status = automationdomain.PermissionRequestStatusSuperseded
			s.notifyAutomationPermissionSessionResolution(ctx, item, request)
		}
		if updateErr = s.repository.CancelBlockedRunsForTaskRevision(
			ctx,
			updated.OwnerUserID,
			updated.JobID,
			updated.PermissionPolicy.Revision,
			"任务绑定的 Session 已删除，请重新分配会话后再运行",
		); updateErr != nil {
			return updateErr
		}
		state := s.ensureJobState(*updated)
		s.persistJobRuntime(ctx, s.jobRuntimeUpdateSnapshot(updated.JobID, state))
		s.recordTaskEvent(
			ctx,
			automationdomain.TaskEventActionSessionBindingInvalidated,
			*updated,
			"",
			map[string]any{
				"enabled":                false,
				"session_binding_state":  updated.SessionBindingState,
				"session_binding_issues": updated.SessionBindingIssues,
			},
		)
	}
	return nil
}

// DeleteTasksForAgent 删除属于指定 Agent 的所有定时任务。
func (s *Service) DeleteTasksForAgent(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) error {
	items, err := s.repository.ListScheduledTasks(
		ctx,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(agentID),
	)
	if err != nil {
		return err
	}
	for _, item := range items {
		deleteCtx := withSkippedIsolatedAutomationSessionCleanup(contextForJobOwner(ctx, item))
		if _, err = s.DeleteTask(deleteCtx, item.JobID); err != nil {
			return err
		}
	}
	return nil
}

func scheduledTaskReferencesSession(
	item automationdomain.ScheduledTask,
	sessionKeys map[string]struct{},
) bool {
	return len(scheduledTaskReferencedSessionKeys(item, sessionKeys)) > 0
}

func scheduledTaskReferencedSessionKeys(
	item automationdomain.ScheduledTask,
	sessionKeys map[string]struct{},
) map[string]struct{} {
	result := make(map[string]struct{})
	for _, sessionKey := range []string{
		item.SessionTarget.BoundSessionKey,
		item.SessionTarget.NamedSessionKey,
		item.Delivery.SessionKey,
		item.Delivery.To,
	} {
		sessionKey = strings.TrimSpace(sessionKey)
		if _, exists := sessionKeys[sessionKey]; exists {
			result[sessionKey] = struct{}{}
		}
	}
	return result
}

func (s *Service) resolveAutomationWorkspacePath(ctx context.Context, agentID string) (string, error) {
	if s.agents != nil && strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
		if err != nil {
			return "", err
		}
		if workspacePath := strings.TrimSpace(agentValue.WorkspacePath); workspacePath != "" {
			return workspacePath, nil
		}
	}
	return strings.TrimSpace(s.config.WorkspacePath), nil
}
