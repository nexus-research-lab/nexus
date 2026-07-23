// INPUT: 经过主智能体鉴权的配置变更与 expected_revision。
// OUTPUT: 预检计划、领域执行、变更后核对与审计闭环。
// POS: configuration 控制面的顶层变更编排。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{7,127}$`)

// PlanChange 校验输入并返回当前 revision、风险和生效语义，不写入任何真相源。
func (s *Service) PlanChange(ctx context.Context, actor Actor, request ChangeRequest) (*ChangePlan, error) {
	if err := requireMainActor(actor); err != nil {
		return nil, err
	}
	definition, err := definitionFor(request.Domain)
	if err != nil {
		return nil, err
	}
	operation, err := operationFor(definition, request.Operation)
	if err != nil {
		return nil, err
	}
	request.Domain = definition.Name
	request.Operation = operation.Name
	if err = requireInputFields(request.Input, operation.RequiredInputFields); err != nil {
		return nil, err
	}
	if err = validateChangeRequest(request); err != nil {
		return nil, err
	}
	current, err := s.domainSnapshot(scopedContext(ctx, actor), actor, definition.Name, false)
	if err != nil {
		return nil, err
	}
	risk := "normal"
	if operation.RequiresConfirmation {
		risk = "destructive"
	} else if containsSensitiveInput(request.Input) {
		risk = "sensitive"
	}
	return &ChangePlan{
		Domain: definition.Name, Operation: operation.Name, Target: strings.TrimSpace(request.Target),
		CurrentRevision: current.Revision, Risk: risk, RuntimeEffect: runtimeEffectForRequest(request, operation),
		RequiresConfirmation: operation.RequiresConfirmation,
		Summary:              fmt.Sprintf("%s.%s target=%s", definition.Name, operation.Name, displayTarget(request.Target)),
		SanitizedInput:       sanitizeRawInput(request.Input),
	}, nil
}

// ApplyChange 使用 request_id 幂等、expected_revision 乐观锁与 destructive confirm 应用变更。
func (s *Service) ApplyChange(ctx context.Context, actor Actor, request ChangeRequest) (*ApplyResult, error) {
	if err := requireMainActor(actor); err != nil {
		return nil, err
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if !requestIDPattern.MatchString(request.RequestID) {
		return nil, errors.New("request_id 必须为 8-128 位字母、数字、点、下划线、冒号或连字符，并在重试时保持不变")
	}
	if strings.TrimSpace(request.ExpectedRevision) == "" {
		return nil, ErrRevisionRequired
	}
	if existing, err := s.auditByID(ctx, actor.OwnerUserID, request.RequestID); err != nil {
		return nil, err
	} else if existing != nil {
		return replayResult(existing)
	}
	plan, err := s.PlanChange(ctx, actor, request)
	if err != nil {
		return nil, err
	}
	request.Domain = plan.Domain
	request.Operation = plan.Operation
	request.Target = plan.Target
	if request.ExpectedRevision != plan.CurrentRevision {
		return nil, fmt.Errorf(
			"配置已变化：expected_revision=%s current_revision=%s；请重新 inspect/plan 后核对",
			request.ExpectedRevision, plan.CurrentRevision,
		)
	}
	if plan.RequiresConfirmation && !request.Confirm {
		return nil, fmt.Errorf("%s.%s 是破坏性操作，必须在核对 plan 后显式传 confirm=true", plan.Domain, plan.Operation)
	}
	audit, created, err := s.beginAudit(ctx, actor, request, *plan)
	if err != nil {
		return nil, fmt.Errorf("建立配置审计失败，未执行变更: %w", err)
	}
	if !created {
		return replayResult(audit)
	}

	resultValue, executionErr := s.executeChange(scopedContext(ctx, actor), actor, request)
	if executionErr != nil {
		executionErr = redactInputSecrets(executionErr, request.Input)
		_ = s.finishAudit(ctx, actor, request.RequestID, "failed", map[string]any{
			"error": executionErr.Error(), "applied": false,
		}, "", executionErr)
		return nil, executionErr
	}
	after, err := s.domainSnapshot(scopedContext(ctx, actor), actor, plan.Domain, true)
	if err != nil {
		executionErr = fmt.Errorf("配置已写入，但变更后核对失败: %w", err)
		_ = s.finishAudit(ctx, actor, request.RequestID, "failed", map[string]any{
			"error": executionErr.Error(), "applied": true, "result": resultValue,
		}, "", executionErr)
		return nil, executionErr
	}
	applyResult := &ApplyResult{
		RequestID: request.RequestID, Applied: true, Domain: plan.Domain, Operation: plan.Operation,
		Target: plan.Target, RevisionBefore: plan.CurrentRevision, RevisionAfter: after.Revision,
		RuntimeEffect: plan.RuntimeEffect, Result: sanitizeValue(resultValue), Checks: after.Checks,
	}
	if err = s.finishAudit(ctx, actor, request.RequestID, "success", applyResult, after.Revision, nil); err != nil {
		return nil, fmt.Errorf("配置已写入并核对，但审计完成失败: %w", err)
	}
	return applyResult, nil
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
