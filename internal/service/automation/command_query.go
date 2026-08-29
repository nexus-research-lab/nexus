// INPUT: round-scoped Automation Actor、只读 operation 与封闭 CLI input。
// OUTPUT: owner/Agent/job/run/会话范围收紧后的任务、报告或 heartbeat 投影。
// POS: Nexus Automation command 的只读 service；后台 run 只能读取宿主绑定的当前任务。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var runtimeAutomationQueryOperations = []string{
	automationdomain.AutomationCommandOperationList,
	automationdomain.AutomationCommandOperationGet,
	automationdomain.AutomationCommandOperationRuns,
	automationdomain.AutomationCommandOperationEvents,
	automationdomain.AutomationCommandOperationReport,
}

var runtimeAutomationMutationOperations = []string{
	automationdomain.AutomationCommandOperationCreate,
	automationdomain.AutomationCommandOperationUpdate,
	automationdomain.AutomationCommandOperationDelete,
	automationdomain.AutomationCommandOperationRun,
	automationdomain.AutomationCommandOperationRetryDelivery,
}

// RuntimeCommandContract 返回当前 Actor 的按需操作目录，不泄漏路由或 capability。
func (s *Service) RuntimeCommandContract(
	ctx context.Context,
	actor command.Actor,
) (automationdomain.AutomationCommandContract, error) {
	if s == nil || !actor.Valid() {
		return automationdomain.AutomationCommandContract{}, errors.New("Automation runtime command Actor 无效")
	}
	ctx = runtimeAutomationCommandContext(ctx, actor)
	if err := s.validateRuntimeCommandActor(ctx, actor); err != nil {
		return automationdomain.AutomationCommandContract{}, err
	}
	queries := append([]string(nil), runtimeAutomationQueryOperations...)
	mutations := []string{}
	if actor.MutationAllowed() {
		queries = append(queries, automationdomain.AutomationCommandOperationDeliveryTargets)
		mutations = append(mutations, runtimeAutomationMutationOperations...)
	}
	return automationdomain.AutomationCommandContract{
		QueryOperations:    queries,
		MutationOperations: mutations,
		MutationAllowed:    actor.MutationAllowed(),
		CrossAgentAllowed:  actor.CrossAgentAllowed(),
		Operations:         runtimeAutomationOperationContracts(actor),
	}, nil
}

func runtimeAutomationOperationContracts(
	actor command.Actor,
) map[string]automationdomain.AutomationCommandOperationContract {
	contracts := map[string]automationdomain.AutomationCommandOperationContract{
		"list":   {Kind: "query", Optional: []string{"query", "agent_id", "include_active", "include_deleted", "enabled", "limit"}},
		"get":    {Kind: "query", Optional: []string{"job_id", "query", "agent_id", "run_limit", "event_limit"}, Notes: []string{"job_id or a unique query is required outside a scheduled run"}},
		"runs":   {Kind: "query", Optional: []string{"job_id", "query", "agent_id", "run_limit"}},
		"events": {Kind: "query", Optional: []string{"job_id", "query", "agent_id", "event_limit"}},
		"report": {Kind: "query", Optional: []string{"date", "timezone", "agent_id", "job_id", "query"}},
	}
	if !actor.MutationAllowed() {
		return contracts
	}
	contracts[automationdomain.AutomationCommandOperationDeliveryTargets] = automationdomain.AutomationCommandOperationContract{
		Kind:     "query",
		Optional: []string{"agent_id", "delivery_channel"},
		Notes: []string{
			"returns existing active-paired DM Sessions only",
			"use the returned session_key as delivery_session_key; do not construct one",
		},
	}
	contracts["create"] = automationdomain.AutomationCommandOperationContract{
		Kind: "mutation", Required: []string{"name", "instruction", "schedule"},
		Optional: []string{"context_mode", "deliver_result", "delivery_session_key", "permission_mode", "overlap_policy", "expires_at", "enabled"},
		Notes:    []string{"schedule.kind=single|daily|interval|cron", "apply requires request_id and native confirmation"},
	}
	contracts["update"] = automationdomain.AutomationCommandOperationContract{
		Kind: "mutation", Optional: []string{"job_id", "query", "name", "instruction", "instruction_append", "schedule", "context_mode", "deliver_result", "delivery_session_key", "permission_mode", "overlap_policy", "expires_at", "clear_expires_at", "enabled", "cancel_active_run", "run_id"},
		Notes: []string{"job_id or a unique query is required", "instruction and instruction_append are mutually exclusive"},
	}
	contracts["delete"] = automationdomain.AutomationCommandOperationContract{Kind: "mutation", Optional: []string{"job_id", "query"}}
	contracts["run"] = automationdomain.AutomationCommandOperationContract{Kind: "mutation", Optional: []string{"job_id", "query"}}
	contracts["retry_delivery"] = automationdomain.AutomationCommandOperationContract{Kind: "mutation", Required: []string{"run_id"}, Optional: []string{"job_id", "query"}}
	if actor.CrossAgentAllowed() {
		advanced := []string{"agent_id", "execution_mode", "reply_mode", "selected_session_key", "named_session_key", "selected_reply_session_key", "reply_session_key"}
		create := contracts["create"]
		create.Optional = append(create.Optional, advanced...)
		contracts["create"] = create
		update := contracts["update"]
		update.Optional = append(update.Optional, advanced...)
		contracts["update"] = update
	}
	return contracts
}

// InspectRuntimeCommand 执行无副作用查询。
func (s *Service) InspectRuntimeCommand(
	ctx context.Context,
	actor command.Actor,
	operation string,
	input automationdomain.AutomationCommandInput,
) (any, error) {
	if s == nil || !actor.Valid() {
		return nil, errors.New("Automation runtime command Actor 无效")
	}
	ctx = runtimeAutomationCommandContext(ctx, actor)
	if err := s.validateRuntimeCommandActor(ctx, actor); err != nil {
		return nil, err
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	switch operation {
	case automationdomain.AutomationCommandOperationList:
		return s.runtimeCommandList(ctx, actor, input)
	case automationdomain.AutomationCommandOperationGet:
		scope, err := s.runtimeCommandTaskScope(ctx, actor, input, false)
		if err != nil {
			return nil, err
		}
		return s.GetTaskStatus(ctx, scope.JobID, commandLimit(input.RunLimit, 10, 50), commandLimit(input.EventLimit, 10, 50))
	case automationdomain.AutomationCommandOperationRuns:
		scope, err := s.runtimeCommandHistoryScope(ctx, actor, input)
		if err != nil {
			return nil, err
		}
		items, err := s.ListTaskRuns(ctx, scope.JobID)
		if err != nil {
			return nil, err
		}
		limit := commandLimit(input.RunLimit, 10, 50)
		if len(items) > limit {
			items = items[:limit]
		}
		return items, nil
	case automationdomain.AutomationCommandOperationEvents:
		scope, err := s.runtimeCommandHistoryScope(ctx, actor, input)
		if err != nil {
			return nil, err
		}
		return s.ListTaskEvents(ctx, scope.JobID, commandLimit(input.EventLimit, 10, 50))
	case automationdomain.AutomationCommandOperationReport:
		return s.runtimeCommandReport(ctx, actor, input)
	case automationdomain.AutomationCommandOperationDeliveryTargets:
		return s.runtimeCommandDeliveryTargets(ctx, actor, input)
	case automationdomain.AutomationCommandOperationHeartbeat:
		if strings.TrimSpace(actor.SourceContextType) == "agent_paired" {
			return nil, errors.New("外部 IM round 不开放 Agent heartbeat 配置查询")
		}
		agentID, err := runtimeCommandAgentID(actor, input.AgentID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(actor.CurrentJobID) != "" && agentID != actor.AgentID {
			return nil, errors.New("后台 scheduled run 只能读取当前 Agent heartbeat")
		}
		return s.GetHeartbeatStatus(ctx, agentID)
	default:
		return nil, fmt.Errorf("未知 Automation inspect operation %q", operation)
	}
}

func (s *Service) runtimeCommandDeliveryTargets(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
) ([]automationdomain.AutomationCommandDeliverySession, error) {
	if !actor.MutationAllowed() {
		return nil, errors.New("当前 runtime round 不能选择 Automation 投递会话")
	}
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return nil, err
	}
	if s.deliveryGrants == nil || s.deliverySessions == nil {
		return nil, errors.New("Automation 投递会话目录尚未装配")
	}
	candidates, err := s.deliveryGrants.ListAutomationDeliverySessions(
		ctx,
		actor.OwnerUserID,
		agentID,
		strings.TrimSpace(input.DeliveryChannel),
	)
	if err != nil {
		return nil, err
	}
	result := make([]automationdomain.AutomationCommandDeliverySession, 0, len(candidates))
	for _, candidate := range candidates {
		stored, resolveErr := s.deliverySessions.ResolveDeliverySession(ctx, candidate.SessionKey)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if stored == nil ||
			strings.TrimSpace(stored.SessionKey) != strings.TrimSpace(candidate.SessionKey) ||
			strings.TrimSpace(stored.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		label := strings.TrimSpace(candidate.Label)
		if label == "" {
			label = strings.TrimSpace(stored.Title)
		}
		result = append(result, automationdomain.AutomationCommandDeliverySession{
			SessionKey: strings.TrimSpace(candidate.SessionKey),
			Channel:    protocol.NormalizeStoredChannelType(candidate.Channel),
			Label:      label,
			AgentID:    agentID,
		})
	}
	return result, nil
}

func (s *Service) validateRuntimeCommandActor(ctx context.Context, actor command.Actor) error {
	if s.agents == nil {
		return nil
	}
	record, err := s.agents.GetAgent(ctx, strings.TrimSpace(actor.AgentID))
	if err != nil {
		return err
	}
	if record == nil || strings.TrimSpace(record.OwnerUserID) != strings.TrimSpace(actor.OwnerUserID) {
		return errors.New("Automation command Actor 已失效")
	}
	if actor.IsMainAgent && !record.IsMain {
		return errors.New("主智能体跨 Agent Automation authority 已失效")
	}
	return nil
}

type runtimeCommandTaskScope struct {
	JobID string
	Job   automationdomain.ScheduledTask
}

func (s *Service) runtimeCommandTaskScope(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
	mutation bool,
) (runtimeCommandTaskScope, error) {
	if mutation && !actor.MutationAllowed() {
		return runtimeCommandTaskScope{}, errors.New("当前 runtime round 只有 Automation 查询权限")
	}
	jobID := strings.TrimSpace(input.JobID)
	if current := strings.TrimSpace(actor.CurrentJobID); current != "" {
		if jobID != "" && jobID != current {
			return runtimeCommandTaskScope{}, fmt.Errorf("当前 scheduled run 只绑定任务 %s", current)
		}
		jobID = current
	}
	if jobID != "" {
		return s.runtimeCommandTaskByID(ctx, actor, jobID)
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return runtimeCommandTaskScope{}, errors.New("job_id 或 query 至少提供一个")
	}
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return runtimeCommandTaskScope{}, err
	}
	jobs, err := s.ListTasks(ctx, agentID)
	if err != nil {
		return runtimeCommandTaskScope{}, err
	}
	matches := runtimeBestMatchingTasks(jobs, query, actor)
	switch len(matches) {
	case 0:
		return runtimeCommandTaskScope{}, fmt.Errorf("没有任务匹配 query %q", query)
	case 1:
		return s.runtimeCommandTaskByID(ctx, actor, matches[0].JobID)
	default:
		return runtimeCommandTaskScope{}, fmt.Errorf("query %q 匹配多个任务，请先 inspect list 后提供精确 job_id", query)
	}
}

func (s *Service) runtimeCommandTaskByID(
	ctx context.Context,
	actor command.Actor,
	jobID string,
) (runtimeCommandTaskScope, error) {
	job, err := s.GetTask(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return runtimeCommandTaskScope{}, err
	}
	if job == nil {
		return runtimeCommandTaskScope{}, automationdomain.ErrJobNotFound
	}
	if !actor.CrossAgentAllowed() && strings.TrimSpace(job.AgentID) != strings.TrimSpace(actor.AgentID) {
		return runtimeCommandTaskScope{}, errors.New("该任务属于其他 Agent")
	}
	return runtimeCommandTaskScope{JobID: strings.TrimSpace(job.JobID), Job: *job}, nil
}

func (s *Service) runtimeCommandHistoryScope(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
) (runtimeCommandTaskScope, error) {
	if scope, err := s.runtimeCommandTaskScope(ctx, actor, input, false); err == nil {
		return scope, nil
	} else if !errors.Is(err, automationdomain.ErrJobNotFound) && strings.TrimSpace(input.Query) == "" {
		return runtimeCommandTaskScope{}, err
	}
	jobID := strings.TrimSpace(input.JobID)
	if jobID != "" {
		agentID, err := runtimeCommandAgentID(actor, input.AgentID)
		if err != nil {
			return runtimeCommandTaskScope{}, err
		}
		items, err := s.SearchTaskHistory(ctx, automationdomain.ScheduledTaskHistorySearchInput{
			Query: jobID, AgentID: agentID, IncludeActive: true, IncludeDeleted: true, Limit: 10,
		})
		if err != nil {
			return runtimeCommandTaskScope{}, err
		}
		for _, item := range items {
			if strings.TrimSpace(item.JobID) == jobID {
				return runtimeCommandTaskScope{JobID: jobID}, nil
			}
		}
		return runtimeCommandTaskScope{}, automationdomain.ErrJobNotFound
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return runtimeCommandTaskScope{}, errors.New("删除任务历史需要 job_id 或 query")
	}
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return runtimeCommandTaskScope{}, err
	}
	items, err := s.SearchTaskHistory(ctx, automationdomain.ScheduledTaskHistorySearchInput{
		Query: query, AgentID: agentID, IncludeActive: true, IncludeDeleted: true, Limit: 10,
	})
	if err != nil {
		return runtimeCommandTaskScope{}, err
	}
	if len(items) != 1 {
		return runtimeCommandTaskScope{}, fmt.Errorf("query %q 必须唯一匹配一条任务历史", query)
	}
	return runtimeCommandTaskScope{JobID: strings.TrimSpace(items[0].JobID)}, nil
}

func (s *Service) runtimeCommandList(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
) (any, error) {
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return nil, err
	}
	if current := strings.TrimSpace(actor.CurrentJobID); current != "" {
		scope, scopeErr := s.runtimeCommandTaskByID(ctx, actor, current)
		if scopeErr != nil {
			return nil, scopeErr
		}
		return []automationdomain.ScheduledTask{scope.Job}, nil
	}
	includeActive := true
	if input.IncludeActive != nil {
		includeActive = *input.IncludeActive
	}
	limit := commandLimit(input.Limit, 20, 50)
	if input.IncludeDeleted {
		query := strings.TrimSpace(input.Query)
		current, hasCurrent := runtimeTaskContextFromActor(actor)
		scopeCurrent := hasCurrent && (current.external || runtimeQueryMentionsCurrentConversation(query))
		if scopeCurrent && runtimeQueryMentionsCurrentConversation(query) {
			query = runtimeStripCurrentConversationTerms(query)
		}
		if scopeCurrent {
			return s.runtimeCurrentContextHistoryItems(
				ctx, actor, agentID, query, includeActive, input.Enabled, limit,
			)
		}
		searchLimit := limit
		if input.Enabled != nil {
			searchLimit = 50
		}
		items, searchErr := s.SearchTaskHistory(ctx, automationdomain.ScheduledTaskHistorySearchInput{
			Query: query, AgentID: agentID,
			IncludeActive: includeActive, IncludeDeleted: true, Limit: searchLimit,
		})
		if searchErr != nil {
			return nil, searchErr
		}
		filtered := make([]automationdomain.ScheduledTaskHistoryItem, 0, len(items))
		for _, item := range items {
			if input.Enabled != nil && (item.Enabled == nil || *item.Enabled != *input.Enabled) {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
		return filtered, nil
	}
	if !includeActive {
		return []automationdomain.ScheduledTask{}, nil
	}
	items, err := s.ListTasks(ctx, agentID)
	if err != nil {
		return nil, err
	}
	items = runtimeFilterTasksForList(items, input.Query, actor)
	filtered := make([]automationdomain.ScheduledTask, 0, len(items))
	for _, item := range items {
		if input.Enabled != nil && item.Enabled != *input.Enabled {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) == limit {
			break
		}
	}
	return filtered, nil
}

func (s *Service) runtimeCommandReport(
	ctx context.Context,
	actor command.Actor,
	input automationdomain.AutomationCommandInput,
) (any, error) {
	agentID, err := runtimeCommandAgentID(actor, input.AgentID)
	if err != nil {
		return nil, err
	}
	if report, handled, reportErr := s.runtimeCurrentConversationReport(ctx, actor, input, agentID); handled {
		return report, reportErr
	}
	jobID := strings.TrimSpace(input.JobID)
	if current := strings.TrimSpace(actor.CurrentJobID); current != "" {
		if jobID != "" && jobID != current {
			return nil, errors.New("后台 scheduled run 只能查看当前任务报告")
		}
		jobID = current
	}
	if jobID == "" && strings.TrimSpace(input.Query) != "" {
		scope, scopeErr := s.runtimeCommandHistoryScope(ctx, actor, input)
		if scopeErr != nil {
			return nil, scopeErr
		}
		jobID = scope.JobID
	}
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = strings.TrimSpace(actor.DefaultTimezone)
	}
	return s.GetDailyReport(ctx, automationdomain.ScheduledTaskDailyReportInput{
		Date: strings.TrimSpace(input.Date), Timezone: timezone, AgentID: agentID, JobID: jobID,
	})
}

func runtimeAutomationCommandContext(ctx context.Context, actor command.Actor) context.Context {
	ctx = automationexec.WithActorAgentID(ctx, strings.TrimSpace(actor.AgentID))
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: strings.TrimSpace(actor.OwnerUserID), Username: strings.TrimSpace(actor.OwnerUserID),
		Role: authctx.RoleOwner, AuthMethod: "nexus_runtime_command",
	})
}

func commandLimit(value int, fallback int, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
