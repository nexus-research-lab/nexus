// INPUT: automation 调用方、目标任务与当前会话信息。
// OUTPUT: owner/Agent/外部会话三重收窄后的任务访问范围。
// POS: nexus_automation 每个工具共享的授权真相源。
package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

type ownedTaskScope struct {
	Context context.Context
	JobID   string
	Job     automationdomain.ScheduledTask
}

type taskHistoryScope struct {
	Context context.Context
	JobID   string
}

func requireOwnedTaskScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (ownedTaskScope, error) {
	jobID := argx.String(args, "job_id")
	if jobID != "" {
		return requireOwnedTaskScopeForJob(ctx, svc, sctx, jobID)
	}
	query := argx.String(args, "query")
	if query == "" {
		if currentJobID := strings.TrimSpace(sctx.CurrentJobID); currentJobID != "" {
			return requireOwnedTaskScopeForJob(ctx, svc, sctx, currentJobID)
		}
		return ownedTaskScope{}, errors.New("job_id or query is required")
	}
	return requireOwnedTaskScopeForQuery(ctx, svc, sctx, args, query)
}

func requireOwnedTaskScopeForJob(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (ownedTaskScope, error) {
	scopedCtx := scopedToolContext(ctx, sctx)
	normalizedJobID := strings.TrimSpace(jobID)
	if err := ensureCurrentAutomationJobScope(sctx, normalizedJobID); err != nil {
		return ownedTaskScope{}, err
	}
	job, err := ownedTaskInScope(scopedCtx, svc, sctx, normalizedJobID)
	if err != nil {
		return ownedTaskScope{}, err
	}
	return ownedTaskScope{Context: scopedCtx, JobID: normalizedJobID, Job: *job}, nil
}

func requireOwnedTaskScopeForQuery(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
	query string,
) (ownedTaskScope, error) {
	if currentJobID := strings.TrimSpace(sctx.CurrentJobID); currentJobID != "" {
		return requireOwnedTaskScopeForJob(ctx, svc, sctx, currentJobID)
	}
	scopedCtx := scopedToolContext(ctx, sctx)
	agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return ownedTaskScope{}, err
	}
	jobs, err := svc.ListTasks(scopedCtx, agentID)
	if err != nil {
		return ownedTaskScope{}, err
	}
	matches := bestMatchingScheduledTasksForToolQuery(jobs, query, sctx)
	switch len(matches) {
	case 0:
		return ownedTaskScope{}, fmt.Errorf("no current scheduled task matched query %q", strings.TrimSpace(query))
	case 1:
		job := matches[0]
		if err = ensureTaskBelongsToCaller(sctx, job.JobID, strings.TrimSpace(job.AgentID)); err != nil {
			return ownedTaskScope{}, err
		}
		return ownedTaskScope{Context: scopedCtx, JobID: strings.TrimSpace(job.JobID), Job: job}, nil
	default:
		return ownedTaskScope{}, fmt.Errorf("query %q matched multiple current scheduled tasks; ask the user to choose one job_id: %s", strings.TrimSpace(query), describeScheduledTaskCandidates(matches, 5))
	}
}

func requireOwnedTaskHistoryScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (taskHistoryScope, error) {
	jobID := argx.String(args, "job_id")
	if jobID != "" {
		return requireOwnedTaskHistoryScopeForJob(ctx, svc, sctx, jobID)
	}
	query := argx.String(args, "query")
	if query == "" {
		if currentJobID := strings.TrimSpace(sctx.CurrentJobID); currentJobID != "" {
			return requireOwnedTaskHistoryScopeForJob(ctx, svc, sctx, currentJobID)
		}
		return taskHistoryScope{}, errors.New("job_id or query is required")
	}
	return requireOwnedTaskHistoryScopeForQuery(ctx, svc, sctx, args, query)
}

func requireOwnedTaskHistoryScopeForJob(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (taskHistoryScope, error) {
	scopedCtx := scopedToolContext(ctx, sctx)
	normalizedJobID := strings.TrimSpace(jobID)
	if err := ensureCurrentAutomationJobScope(sctx, normalizedJobID); err != nil {
		return taskHistoryScope{}, err
	}
	job, err := svc.GetTask(scopedCtx, normalizedJobID)
	if err != nil {
		return taskHistoryScope{}, err
	}
	if job != nil {
		if err = ensureTaskBelongsToCaller(sctx, normalizedJobID, strings.TrimSpace(job.AgentID)); err != nil {
			return taskHistoryScope{}, err
		}
		if err = ensureTaskVisibleInReadContext(sctx, *job); err != nil {
			return taskHistoryScope{}, err
		}
		return taskHistoryScope{Context: scopedCtx, JobID: normalizedJobID}, nil
	}
	events, eventErr := svc.ListTaskEvents(scopedCtx, normalizedJobID, 50)
	runs, runErr := svc.ListTaskRuns(scopedCtx, normalizedJobID)
	if eventErr != nil && !errors.Is(eventErr, automationdomain.ErrJobNotFound) {
		return taskHistoryScope{}, eventErr
	}
	if runErr != nil && !errors.Is(runErr, automationdomain.ErrJobNotFound) {
		return taskHistoryScope{}, runErr
	}
	if len(events) == 0 && len(runs) == 0 {
		return taskHistoryScope{}, automationdomain.ErrJobNotFound
	}
	if !hasMainAgentScopeAuthority(sctx) {
		caller, err := callerAgentID(sctx)
		if err != nil {
			return taskHistoryScope{}, err
		}
		if len(events) == 0 {
			return taskHistoryScope{}, fmt.Errorf("scheduled task %s has no ownership audit; only the main agent can inspect deleted run history without task events", normalizedJobID)
		}
		for _, event := range events {
			if strings.TrimSpace(event.AgentID) != caller {
				return taskHistoryScope{}, taskOwnershipError(normalizedJobID)
			}
		}
	}
	if err = ensureTaskHistoryVisibleInReadContext(sctx, normalizedJobID, events); err != nil {
		return taskHistoryScope{}, err
	}
	return taskHistoryScope{Context: scopedCtx, JobID: normalizedJobID}, nil
}

func requireOwnedTaskHistoryScopeForQuery(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
	query string,
) (taskHistoryScope, error) {
	if currentJobID := strings.TrimSpace(sctx.CurrentJobID); currentJobID != "" {
		return requireOwnedTaskHistoryScopeForJob(ctx, svc, sctx, currentJobID)
	}
	scopedCtx := scopedToolContext(ctx, sctx)
	agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return taskHistoryScope{}, err
	}
	if scope, handled, err := requireCurrentConversationTaskHistoryScopeForQuery(scopedCtx, svc, sctx, agentID, query); handled {
		return scope, err
	}
	items, err := svc.SearchTaskHistory(scopedCtx, automationdomain.ScheduledTaskHistorySearchInput{
		Query:          query,
		AgentID:        agentID,
		IncludeActive:  true,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		return taskHistoryScope{}, err
	}
	switch len(items) {
	case 0:
		return taskHistoryScope{}, fmt.Errorf("no scheduled task history matched query %q", strings.TrimSpace(query))
	case 1:
		return requireOwnedTaskHistoryScopeForJob(ctx, svc, sctx, items[0].JobID)
	default:
		return taskHistoryScope{}, fmt.Errorf("query %q matched multiple scheduled task history candidates; ask the user to choose one job_id: %s", strings.TrimSpace(query), describeTaskHistoryCandidates(items, 5))
	}
}

func ensureCurrentAutomationJobScope(sctx contract.ServerContext, requestedJobID string) error {
	currentJobID := strings.TrimSpace(sctx.CurrentJobID)
	if currentJobID == "" || currentJobID == strings.TrimSpace(requestedJobID) {
		return nil
	}
	return fmt.Errorf(
		"scheduled task run %s is scoped to job %s",
		strings.TrimSpace(sctx.CurrentRunID),
		currentJobID,
	)
}

func ownedTaskInScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (*automationdomain.ScheduledTask, error) {
	job, err := svc.GetTask(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("scheduled task %s not found", jobID)
	}
	if err = ensureTaskBelongsToCaller(sctx, jobID, strings.TrimSpace(job.AgentID)); err != nil {
		return nil, err
	}
	if err = ensureTaskVisibleInReadContext(sctx, *job); err != nil {
		return nil, err
	}
	return job, nil
}

func ensureTaskBelongsToCaller(sctx contract.ServerContext, jobID string, agentID string) error {
	if hasMainAgentScopeAuthority(sctx) {
		return nil
	}
	caller, err := callerAgentID(sctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(agentID) != caller {
		return taskOwnershipError(jobID)
	}
	return nil
}

func callerAgentID(sctx contract.ServerContext) (string, error) {
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if caller == "" {
		return "", errors.New("missing caller agent context")
	}
	return caller, nil
}

func taskOwnershipError(jobID string) error {
	return fmt.Errorf("scheduled task %s belongs to another agent; only its Agent or the main Agent in its trusted private Nexus DM can access it", jobID)
}

func hasMainAgentScopeAuthority(sctx contract.ServerContext) bool {
	return sctx.IsMainAgent &&
		strings.TrimSpace(sctx.SourceContextType) == "agent"
}

func ensureTaskVisibleInReadContext(sctx contract.ServerContext, job automationdomain.ScheduledTask) error {
	current, ok := externalTaskReadContext(sctx)
	if !ok || scheduledTaskMatchesCurrentContext(job, current) {
		return nil
	}
	return fmt.Errorf("scheduled task %s is outside the current external conversation", strings.TrimSpace(job.JobID))
}

func ensureTaskHistoryVisibleInReadContext(
	sctx contract.ServerContext,
	jobID string,
	events []automationdomain.ScheduledTaskEvent,
) error {
	current, ok := externalTaskReadContext(sctx)
	if !ok {
		return nil
	}
	for _, event := range events {
		if taskEventMatchesCurrentContext(event, current) {
			return nil
		}
	}
	return fmt.Errorf("scheduled task %s history is outside the current external conversation", strings.TrimSpace(jobID))
}

func externalTaskReadContext(sctx contract.ServerContext) (currentTaskContext, bool) {
	if isTrustedInteractiveSource(sctx) {
		return currentTaskContext{}, false
	}
	current, ok := currentTaskContextFromServerContext(sctx)
	if !ok || !current.external {
		return currentTaskContext{}, false
	}
	return current, true
}

func requireTrustedInteractiveMutation(sctx contract.ServerContext) error {
	if isTrustedInteractiveSource(sctx) {
		return nil
	}
	return errors.New("scheduled task mutations require a trusted interactive Nexus DM or Room context")
}

func requireAgentExecutionTaskMutation(job automationdomain.ScheduledTask) error {
	if automationdomain.NormalizeExecutionKind(job.ExecutionKind) != automationdomain.ExecutionKindScript {
		return nil
	}
	return errors.New("script scheduled tasks are human-control-plane only and cannot be modified, deleted, repaired, or run through an Agent conversation")
}

func describeScheduledTaskCandidates(jobs []automationdomain.ScheduledTask, limit int) string {
	parts := make([]string, 0, len(jobs))
	for index, job := range jobs {
		if limit > 0 && index >= limit {
			parts = append(parts, fmt.Sprintf("...and %d more", len(jobs)-index))
			break
		}
		parts = append(parts, describeTaskCandidate(job.JobID, job.Name, job.AgentID, job.Enabled, job.Running, false))
	}
	return strings.Join(parts, "; ")
}

func describeTaskHistoryCandidates(items []automationdomain.ScheduledTaskHistoryItem, limit int) string {
	parts := make([]string, 0, len(items))
	for index, item := range items {
		if limit > 0 && index >= limit {
			parts = append(parts, fmt.Sprintf("...and %d more", len(items)-index))
			break
		}
		enabled := false
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		parts = append(parts, describeTaskCandidate(item.JobID, item.Name, item.AgentID, enabled, item.Running, item.Deleted))
	}
	return strings.Join(parts, "; ")
}

func describeTaskCandidate(jobID string, name string, agentID string, enabled bool, running bool, deleted bool) string {
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	if running {
		status += ",running"
	}
	if deleted {
		status += ",deleted"
	}
	label := strings.TrimSpace(name)
	if label == "" {
		label = strings.TrimSpace(jobID)
	}
	return fmt.Sprintf("%s (%s, agent=%s, %s)", strings.TrimSpace(jobID), label, strings.TrimSpace(agentID), status)
}

func scopedToolContext(ctx context.Context, sctx contract.ServerContext) context.Context {
	ctx = automationexec.WithActorAgentID(ctx, sctx.CurrentAgentID)
	ownerUserID := strings.TrimSpace(sctx.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "mcp_runtime",
	})
}

// resolveListAgentID 决定 automation_query 的 Agent 过滤条件。
// owner main 仅在自己的可信私有 DM 支持显式过滤或全部列出；其他上下文限定为自己。
func resolveListAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if hasMainAgentScopeAuthority(sctx) {
		return requested, nil
	}
	if caller == "" {
		return "", fmt.Errorf("missing caller agent context")
	}
	if requested != "" && requested != caller {
		return "", fmt.Errorf("agent %s cannot list scheduled tasks of another agent", caller)
	}
	return caller, nil
}

// resolveCreateAgentID 决定 automation_update 的目标 Agent。
// owner main 仅在自己的可信私有 DM 可指定；其他上下文强制为自己。
func resolveCreateAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if hasMainAgentScopeAuthority(sctx) {
		if requested != "" {
			return requested, nil
		}
		if caller == "" {
			return "", fmt.Errorf("agent_id is required")
		}
		return caller, nil
	}
	if caller == "" {
		return "", fmt.Errorf("missing caller agent context")
	}
	if requested != "" && requested != caller {
		return "", fmt.Errorf("agent %s cannot create scheduled tasks for another agent", caller)
	}
	return caller, nil
}
