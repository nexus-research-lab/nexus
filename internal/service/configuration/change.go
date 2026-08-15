// INPUT: 动态重验后的可信 Actor、业务与 lease 身份、资源 scope、plan digest 与 expected_revision。
// OUTPUT: 身份不可转移、scope 内串行的 CAS 执行、热重载状态、变更后核对与审计闭环。
// POS: configuration 控制面的顶层计划与应用编排边界。
package configuration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{7,127}$`)

// CLIApplyOptions 是 nexuscfg 进程内确认后的执行输入。
// SecretValues 只允许来自 nexuscfg stdin，调用结束后会被清空。
type CLIApplyOptions struct {
	Confirmed    bool
	SecretValues map[string]string
}

// PlanChange 校验输入并返回当前 revision、风险和生效语义，不写入任何真相源。
func (s *Service) PlanChange(ctx context.Context, actor Actor, request ChangeRequest) (*ChangePlan, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	return s.planChange(ctx, resolved, request)
}

func (s *Service) planChange(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
) (*ChangePlan, error) {
	request, scope, err := authorizeChange(actor, request)
	if err != nil {
		return nil, err
	}
	operation, err := operationForActor(actor, request)
	if err != nil {
		return nil, err
	}
	preparedInput, secretSlots, err := secretinput.PrepareJSON(request.Input)
	if err != nil {
		return nil, err
	}
	preparedRequest := request
	preparedRequest.Input = preparedInput
	if err = requireInputFields(request.Input, operation.RequiredInputFields); err != nil {
		return nil, err
	}
	if err = validateChangeRequest(preparedRequest); err != nil {
		return nil, err
	}
	if err = s.validateScopedChange(
		scopedContext(ctx, actor.Actor),
		actor,
		preparedRequest,
	); err != nil {
		return nil, err
	}
	current, err := s.snapshotForChange(
		scopedContext(ctx, actor.Actor),
		actor,
		preparedRequest,
		false,
	)
	if err != nil {
		return nil, err
	}
	risk, requiresConfirmation := classifyChangeRisk(operation, request)
	plan := &ChangePlan{
		Domain: request.Domain, Operation: request.Operation, Target: request.Target,
		Scope: scope, CurrentRevision: current.Revision, StateVersion: current.StateVersion,
		Risk: risk, RuntimeEffect: runtimeEffectForRequest(request, operation),
		RequiresConfirmation: requiresConfirmation,
		Summary:              fmt.Sprintf("%s.%s target=%s", request.Domain, request.Operation, displayTarget(request.Target)),
		SanitizedInput:       sanitizeRawInput(request.Input),
		SecretSlots:          secretSlots,
	}
	plan.PlanDigest, err = s.planDigest(actor, request, *plan)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// ApplyChange 使用 request_id 幂等、expected_revision 乐观锁与一次性人工批准应用变更。
func (s *Service) ApplyChange(ctx context.Context, actor Actor, request ChangeRequest) (*ApplyResult, error) {
	return s.applyChange(ctx, actor, request, nil)
}

// ApplyChangeFromCLI 允许人工终端或宿主签发 round capability 的 nexuscfg 执行变更。
func (s *Service) ApplyChangeFromCLI(
	ctx context.Context,
	actor Actor,
	request ChangeRequest,
	options CLIApplyOptions,
) (*ApplyResult, error) {
	defer clear(options.SecretValues)
	return s.applyChange(ctx, actor, request, &options)
}

func (s *Service) applyChange(
	ctx context.Context,
	actor Actor,
	request ChangeRequest,
	cliOptions *CLIApplyOptions,
) (*ApplyResult, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if cliOptions != nil && len(cliOptions.SecretValues) > 0 &&
		(!resolved.isMain() || resolved.RoundLeaseRequired) {
		return nil, errors.New("只有人工终端中的 owner 主智能体 nexuscfg 可以提交 secret slot")
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if !requestIDPattern.MatchString(request.RequestID) {
		return nil, errors.New("request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符，并在重试时保持不变")
	}
	if strings.TrimSpace(request.ExpectedRevision) == "" {
		return nil, ErrRevisionRequired
	}
	if strings.TrimSpace(request.PlanDigest) == "" {
		return nil, errors.New("plan_digest 不能为空；必须使用同一进程内 plan 返回的 plan_digest")
	}
	request, scope, err := authorizeChange(resolved, request)
	if err != nil {
		return nil, err
	}
	unlock := s.lockMutation(resolved.OwnerUserID + ":" + scope.Kind + ":" + scope.ID)
	defer unlock()

	// 锁内重新读取 Actor，保证 host 转让、成员移除或 main 身份变化会立刻撤销旧权限。
	resolved, err = s.resolveActor(ctx, resolved.Actor)
	if err != nil {
		return nil, err
	}
	request, scope, err = authorizeChange(resolved, request)
	if err != nil {
		return nil, err
	}
	if existing, lookupErr := s.auditByID(ctx, resolved.OwnerUserID, request.RequestID); lookupErr != nil {
		return nil, lookupErr
	} else if existing != nil {
		if existing.IntentDigest != request.PlanDigest {
			return nil, fmt.Errorf("request_id=%s 已绑定另一项配置计划，不能复用于不同 intent", request.RequestID)
		}
		return s.replayOrRecover(ctx, resolved.Actor, existing)
	}
	plan, err := s.planChange(ctx, resolved, request)
	if err != nil {
		return nil, err
	}
	request.Domain = plan.Domain
	request.Operation = plan.Operation
	request.Target = plan.Target
	if request.PlanDigest != plan.PlanDigest {
		return nil, errors.New("plan_digest 与当前作用域、输入或 revision 不匹配；请重新 plan 并核对")
	}
	if request.ExpectedRevision != plan.CurrentRevision {
		return nil, fmt.Errorf(
			"配置已变化：expected_revision=%s current_revision=%s；请重新 inspect/plan 后核对",
			request.ExpectedRevision, plan.CurrentRevision,
		)
	}
	var humanApproval *humanApprovalRecord
	if plan.RequiresConfirmation {
		if cliOptions != nil {
			if !cliOptions.Confirmed {
				return nil, errors.New("该配置变更需要确认；请核对 plan 后使用 nexuscfg apply --confirm")
			}
		} else if s.humanVerifier == nil {
			return nil, errors.New("高风险配置变更缺少 human principal verifier")
		} else {
			principal, releaseHumanLease, leaseErr := s.humanVerifier.AcquireBoundInteractiveHumanLease(
				ctx,
				resolved.OwnerUserID,
				resolved.AuthMethod,
				resolved.AuthSessionID,
			)
			if leaseErr != nil {
				return nil, fmt.Errorf("人工批准登录已失效，未执行配置变更: %w", leaseErr)
			}
			defer releaseHumanLease()
			if principal == nil ||
				strings.TrimSpace(principal.UserID) != resolved.OwnerUserID ||
				strings.TrimSpace(principal.Role) != resolved.PrincipalRole {
				return nil, errors.New("人工批准 principal 已变化，未执行配置变更")
			}
			if humanApproval, err = s.consumeHumanApproval(resolved, request, *plan); err != nil {
				return nil, err
			}
		}
	}
	executionRequest := request
	if len(plan.SecretSlots) > 0 {
		secretValues := map[string]string(nil)
		if cliOptions != nil {
			secretValues = cliOptions.SecretValues
		} else if humanApproval != nil {
			secretValues = humanApproval.ConfigurationSecrets
		}
		if len(secretValues) == 0 {
			return nil, errors.New("含 secret slot 的配置变更缺少当前真人提供的 secret 值")
		}
		executionRequest.Input, err = secretinput.MaterializeJSON(
			request.Input,
			secretValues,
		)
		defer clear(executionRequest.Input)
		clear(secretValues)
		if humanApproval != nil {
			humanApproval.ConfigurationSecrets = nil
		}
		if err != nil {
			return nil, err
		}
		if err = validateChangeRequest(executionRequest); err != nil {
			return nil, redactInputSecrets(err, executionRequest.Input)
		}
		if err = s.validateScopedChange(
			scopedContext(ctx, resolved.Actor),
			resolved,
			executionRequest,
		); err != nil {
			return nil, redactInputSecrets(err, executionRequest.Input)
		}
	}
	audit, created, err := s.beginAudit(ctx, resolved, request, *plan, humanApproval)
	if err != nil {
		return nil, fmt.Errorf("建立配置审计失败，未执行变更: %w", err)
	}
	if !created {
		if audit.IntentDigest != plan.PlanDigest {
			return nil, fmt.Errorf("request_id=%s 已绑定另一项配置计划", request.RequestID)
		}
		return s.replayOrRecover(ctx, resolved.Actor, audit)
	}

	scoped := scopedContext(ctx, resolved.Actor)
	resultValue, executionErr := s.executeChange(
		scoped,
		resolved,
		executionRequest,
		plan.StateVersion,
	)
	if executionErr != nil {
		needsReconcile := mutationNeedsReconcile(executionRequest, executionErr)
		committedDespiteError := mutationAppliedDespiteError(executionRequest, executionErr)
		executionErr = redactInputSecrets(executionErr, executionRequest.Input)
		status := "failed"
		applied := committedDespiteError
		revisionAfter := ""
		if needsReconcile {
			status = "reconcile_required"
			after, snapshotErr := s.snapshotForChangeState(scoped, resolved, request, true, false)
			if snapshotErr == nil {
				revisionAfter = after.Revision
				applied = applied || after.Revision != plan.CurrentRevision
			}
		} else {
			after, snapshotErr := s.snapshotForChangeState(scoped, resolved, request, true, false)
			if snapshotErr == nil {
				revisionAfter = after.Revision
				applied = after.Revision != plan.CurrentRevision
				if applied {
					status = "reconcile_required"
				}
			}
		}
		_ = s.finishAudit(ctx, resolved.Actor, request.RequestID, status, map[string]any{
			"error": executionErr.Error(), "applied": applied, "result": resultValue,
		}, revisionAfter, executionErr)
		return nil, executionErr
	}
	after, err := s.snapshotAfterChange(scoped, resolved, request, *plan, resultValue)
	if err != nil {
		executionErr = fmt.Errorf("配置已写入，但变更后核对失败: %w", err)
		_ = s.finishAudit(ctx, resolved.Actor, request.RequestID, "reconcile_required", map[string]any{
			"error": executionErr.Error(), "applied": true, "result": resultValue,
		}, after.Revision, executionErr)
		return nil, executionErr
	}
	applyResult := &ApplyResult{
		RequestID: request.RequestID, Applied: true, Domain: plan.Domain, Operation: plan.Operation,
		Target: plan.Target, Scope: scope, RevisionBefore: plan.CurrentRevision, RevisionAfter: after.Revision,
		RuntimeEffect: plan.RuntimeEffect, Reload: reloadStatusFor(request, plan.RuntimeEffect),
		Result: sanitizeValue(resultValue), Checks: after.Checks,
	}
	if err = s.finishAudit(ctx, resolved.Actor, request.RequestID, "success", applyResult, after.Revision, nil); err != nil {
		return nil, fmt.Errorf("配置已写入并核对，但审计完成失败: %w", err)
	}
	return applyResult, nil
}

func mutationNeedsReconcile(request ChangeRequest, err error) bool {
	if request.Domain == DomainSkills && skillsvc.SkillMutationNeedsReconcile(err) {
		return true
	}
	return mutationAppliedDespiteError(request, err)
}

func mutationAppliedDespiteError(request ChangeRequest, err error) bool {
	switch request.Domain {
	case DomainAgents:
		return request.Operation == "delete" && agentsvc.AgentDeletionCommitted(err)
	case DomainAutomation:
		return request.Operation == "delete" && automationsvc.TaskDeletionCommitted(err)
	case DomainRooms:
		switch request.Operation {
		case "delete":
			return roomsvc.RoomDeletionCommitted(err)
		case "delete_conversation":
			return roomsvc.ConversationDeletionCommitted(err)
		case "remove_member":
			return roomsvc.RoomMemberDeletionCommitted(err)
		}
	case DomainSessions:
		return request.Operation == "delete" &&
			sessionsvc.SessionDeletionCommitted(err)
	case DomainSkills:
		return skillsvc.SkillMutationApplied(err)
	default:
		return false
	}
	return false
}

func (s *Service) lockMutation(key string) func() {
	var shard uint32 = 2166136261
	for index := 0; index < len(key); index++ {
		shard ^= uint32(key[index])
		shard *= 16777619
	}
	mutex := &s.mutationLocks[shard%uint32(len(s.mutationLocks))]
	mutex.Lock()
	return mutex.Unlock
}

func (s *Service) planDigest(actor *resolvedActor, request ChangeRequest, plan ChangePlan) (string, error) {
	input := any(map[string]any{})
	if len(request.Input) > 0 {
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return "", fmt.Errorf("计算 plan digest: %w", err)
		}
	}
	payload, err := json.Marshal(map[string]any{
		"owner_user_id":     actor.OwnerUserID,
		"actor_agent_id":    actor.AgentID,
		"authority":         actor.Authority,
		"context":           actor.Context,
		"session_key":       actor.SessionKey,
		"round_id":          actor.RoundID,
		"lease_session_key": actor.LeaseSessionKey,
		"lease_round_id":    actor.LeaseRoundID,
		"source_context":    actor.SourceContext,
		"scope":             plan.Scope,
		"domain":            plan.Domain,
		"operation":         plan.Operation,
		"target":            plan.Target,
		"input":             input,
		"current_revision":  plan.CurrentRevision,
		"state_version":     plan.StateVersion,
	})
	if err != nil {
		return "", err
	}
	key, err := s.integrityKeyBytes()
	if err != nil {
		return "", fmt.Errorf("初始化配置计划摘要密钥: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil)), nil
}

func reloadStatusFor(request ChangeRequest, effect string) ReloadStatus {
	status := ReloadStatus{
		Mode:                 effect,
		State:                "applied",
		CurrentRoundAffected: false,
		Message:              "持久配置已核对；运行时将在声明的安全边界采用新值",
	}
	switch effect {
	case "immediate":
		status.CurrentRoundAffected = true
	case "mixed":
		status.State = "scheduled"
		status.CurrentRoundAffected = true
		status.Message = "安全或在线可重配字段立即生效；其余 runtime 配置在下一轮采用"
	case "restart_required":
		status.State = "restart_required"
		status.Message = "配置已持久化并核对，需重启 Nexus 才会载入"
	case "next_round":
		status.State = "scheduled"
		status.Message = "配置已持久化并核对，现有轮次不变，下一轮通过 runtime Reconfigure 采用"
	case "next_session":
		status.State = "scheduled"
		status.Message = "配置已持久化并核对；现有连接保持不变，下一次会话或重新连接时采用"
	case "next_ingress":
		status.State = "scheduled"
		status.Message = "配置已持久化并核对；Channel runtime 不替换，下一条外部消息重新查询配对记录"
	case "next_session_or_new_agent":
		status.State = "scheduled"
		status.Message = "配置已持久化并核对；下一次会话或新建 Agent 时采用"
	case "ui_immediate_runtime_next_round":
		status.State = "scheduled"
		status.Message = "目录/UI 立即刷新；现有轮次不变，下一轮重建提示与 runtime 配置"
	case "ui_immediate":
		status.Message = "目录/UI 已立即刷新；当前 runtime 执行内容不变"
	case "ui_immediate_runtime_next_input":
		status.State = "scheduled"
		status.Message = "目录/UI 已立即刷新；新建上下文从下一条输入开始参与 runtime 路由"
	case "authority_immediate_routing_next_input":
		status.CurrentRoundAffected = true
		status.Message = "群主授权已立即切换；下一条 Room 输入使用新的默认路由"
	case "security_immediate_runtime_next_round":
		status.CurrentRoundAffected = true
		status.Message = "安全限制已在服务层立即生效；工具与提示在下一轮重载"
	}
	if strings.HasPrefix(effect, "mixed:") {
		status.State = "scheduled"
		status.CurrentRoundAffected = true
		status.Message = "在线可重配字段立即生效；其余默认值在下一次会话或新建 Agent 时采用"
	}
	if request.Domain == DomainAgents && request.Operation == "update" {
		status.CurrentRoundAffected = true
		status.Message = "permission_mode（若变更）已同步活跃 DM/Room runtime；其他 Agent 配置下一轮采用"
	}
	return status
}

func containsSensitiveInput(input json.RawMessage) bool {
	if len(input) == 0 || !json.Valid(input) {
		return false
	}
	var value any
	if json.Unmarshal(input, &value) != nil {
		return false
	}
	return containsSensitiveNode(value, "")
}

func containsSensitiveNode(value any, key string) bool {
	if isSensitiveKey(key) {
		return true
	}
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			if containsSensitiveNode(child, childKey) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSensitiveNode(child, key) {
				return true
			}
		}
	}
	return false
}

func sanitizeRawInput(input json.RawMessage) any {
	if len(input) == 0 {
		return map[string]any{}
	}
	var value any
	if json.Unmarshal(input, &value) != nil {
		return map[string]any{"redacted": true}
	}
	return sanitizeValue(value)
}

func redactInputSecrets(err error, input json.RawMessage) error {
	if err == nil || len(input) == 0 || !json.Valid(input) {
		return err
	}
	var value any
	if json.Unmarshal(input, &value) != nil {
		return err
	}
	secrets := make([]string, 0)
	collectSecretStrings(value, "", &secrets)
	message := err.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	return errors.New(message)
}

func collectSecretStrings(value any, key string, result *[]string) {
	if isSensitiveKey(key) {
		collectStrings(value, result)
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			collectSecretStrings(child, childKey, result)
		}
	case []any:
		for _, child := range typed {
			collectSecretStrings(child, key, result)
		}
	}
}

func collectStrings(value any, result *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			*result = append(*result, typed)
		}
	case map[string]any:
		for _, child := range typed {
			collectStrings(child, result)
		}
	case []any:
		for _, child := range typed {
			collectStrings(child, result)
		}
	}
}

func displayTarget(target string) string {
	if value := strings.TrimSpace(target); value != "" {
		return value
	}
	return "(domain)"
}

func runtimeEffectForRequest(request ChangeRequest, operation OperationDefinition) string {
	if request.Domain != DomainPreferences || request.Operation != "update" {
		return operation.RuntimeEffect
	}
	var fields map[string]any
	if json.Unmarshal(request.Input, &fields) != nil {
		return operation.RuntimeEffect
	}
	hasWebSearch := false
	hasOther := false
	for field := range fields {
		switch field {
		case "web_search", "web_search_api_key":
			hasWebSearch = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasWebSearch && hasOther:
		return "mixed: WebSearch immediate; other defaults next session or new Agent"
	case hasWebSearch:
		return "immediate"
	default:
		return "next_session_or_new_agent"
	}
}
