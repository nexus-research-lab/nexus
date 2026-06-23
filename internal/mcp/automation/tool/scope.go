package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type ownedTaskScope struct {
	Context context.Context
	JobID   string
	Job     protocol.CronJob
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
		return ownedTaskScope{}, errors.New("job_id or query is required")
	}
	return requireOwnedTaskScopeForQuery(ctx, svc, sctx, args, query)
}

func requireOwnedTaskScopeForJob(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (ownedTaskScope, error) {
	scopedCtx := scopedToolContext(ctx, sctx)
	normalizedJobID := strings.TrimSpace(jobID)
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
	scopedCtx := scopedToolContext(ctx, sctx)
	agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return ownedTaskScope{}, err
	}
	jobs, err := svc.ListTasks(scopedCtx, agentID)
	if err != nil {
		return ownedTaskScope{}, err
	}
	matches := bestMatchingCronJobsForToolQuery(jobs, query, sctx)
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
		return ownedTaskScope{}, fmt.Errorf("query %q matched multiple current scheduled tasks; ask the user to choose one job_id: %s", strings.TrimSpace(query), describeCronJobCandidates(matches, 5))
	}
}

func requireOwnedTaskHistoryScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (taskHistoryScope, error) {
	jobID := argx.String(args, "job_id")
	if jobID != "" {
		return requireOwnedTaskHistoryScopeForJob(ctx, svc, sctx, jobID)
	}
	query := argx.String(args, "query")
	if query == "" {
		return taskHistoryScope{}, errors.New("job_id or query is required")
	}
	return requireOwnedTaskHistoryScopeForQuery(ctx, svc, sctx, args, query)
}

func requireOwnedTaskHistoryScopeForJob(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (taskHistoryScope, error) {
	scopedCtx := scopedToolContext(ctx, sctx)
	normalizedJobID := strings.TrimSpace(jobID)
	job, err := svc.GetTask(scopedCtx, normalizedJobID)
	if err != nil {
		return taskHistoryScope{}, err
	}
	if job != nil {
		if err = ensureTaskBelongsToCaller(sctx, normalizedJobID, strings.TrimSpace(job.AgentID)); err != nil {
			return taskHistoryScope{}, err
		}
		return taskHistoryScope{Context: scopedCtx, JobID: normalizedJobID}, nil
	}
	events, eventErr := svc.ListTaskEvents(scopedCtx, normalizedJobID, 50)
	runs, runErr := svc.ListTaskRuns(scopedCtx, normalizedJobID)
	if eventErr != nil && !errors.Is(eventErr, protocol.ErrJobNotFound) {
		return taskHistoryScope{}, eventErr
	}
	if runErr != nil && !errors.Is(runErr, protocol.ErrJobNotFound) {
		return taskHistoryScope{}, runErr
	}
	if len(events) == 0 && len(runs) == 0 {
		return taskHistoryScope{}, protocol.ErrJobNotFound
	}
	if !sctx.IsMainAgent {
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
	return taskHistoryScope{Context: scopedCtx, JobID: normalizedJobID}, nil
}

func requireOwnedTaskHistoryScopeForQuery(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	args map[string]any,
	query string,
) (taskHistoryScope, error) {
	scopedCtx := scopedToolContext(ctx, sctx)
	agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
	if err != nil {
		return taskHistoryScope{}, err
	}
	if scope, handled, err := requireCurrentConversationTaskHistoryScopeForQuery(scopedCtx, svc, sctx, agentID, query); handled {
		return scope, err
	}
	items, err := svc.SearchTaskHistory(scopedCtx, protocol.CronTaskHistorySearchInput{
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

// ensureJobOwnedByCaller 校验 jobID 对应任务是否属于当前调用方。
// 主智能体豁免；普通 agent 仅可访问/修改自己 agent_id 的任务。
func ensureJobOwnedByCaller(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) error {
	return ensureJobOwnedByCallerInScope(scopedToolContext(ctx, sctx), svc, sctx, jobID)
}

func ensureJobOwnedByCallerInScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) error {
	_, err := ownedTaskInScope(ctx, svc, sctx, jobID)
	return err
}

func ownedTaskInScope(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) (*protocol.CronJob, error) {
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
	return job, nil
}

func ensureTaskBelongsToCaller(sctx contract.ServerContext, jobID string, agentID string) error {
	if sctx.IsMainAgent {
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
	return fmt.Errorf("scheduled task %s belongs to another agent; only its owner or main agent can manage it", jobID)
}

func describeCronJobCandidates(jobs []protocol.CronJob, limit int) string {
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

func describeTaskHistoryCandidates(items []protocol.CronTaskHistoryItem, limit int) string {
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
	ctx = automationdomain.WithActorAgentID(ctx, sctx.CurrentAgentID)
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

// resolveListAgentID 决定 list_scheduled_tasks 的过滤条件。
// 主智能体支持显式过滤或全部列出；普通 agent 强制限定为自己。
func resolveListAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if sctx.IsMainAgent {
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

// resolveCreateAgentID 决定 create_scheduled_task 的归属 agent_id。
// 主智能体可任意指定；普通 agent 强制为自己。
func resolveCreateAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if sctx.IsMainAgent {
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
