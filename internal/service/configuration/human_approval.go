// INPUT: 已认证 WebSocket 允许的 configuration apply tool 与锁内重算后的 plan。
// OUTPUT: 同时绑定业务 session/root round 与真实 runtime lease 的一次性批准消费结果。
// POS: destructive 配置写入的人类授权真相源；模型参数中的 confirm 不具授权效力。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

const maximumHumanApprovalLifetime = 2 * time.Minute

type humanApprovalRecord struct {
	PermissionRequestID    string
	OwnerUserID            string
	PrincipalRole          string
	PrincipalAuthMethod    string
	PrincipalAuthSessionID string
	AgentID                string
	SessionKey             string
	RoundID                string
	LeaseSessionKey        string
	LeaseRoundID           string
	ContextKind            string
	ContextID              string
	RoomID                 string
	ConversationID         string
	Scope                  ScopeRef
	RequestID              string
	PlanDigest             string
	ExpectedRevision       string
	Domain                 string
	Operation              string
	Target                 string
	ApprovedAt             time.Time
	ExpiresAt              time.Time
	ConfigurationSecrets   map[string]string
}

// RecordHumanToolApproval 在 runtime 收到真实 WebSocket allow 后、工具执行前，
// 重验完整 plan 并记录一次性批准。非确认型操作无需记录。
func (s *Service) RecordHumanToolApproval(
	ctx context.Context,
	approval permissionctx.HumanToolApproval,
) error {
	if !isConfigurationApplyApprovalTool(approval.ToolName) {
		return errors.New("人工批准工具与配置 apply 不匹配")
	}
	if strings.TrimSpace(approval.PermissionRequestID) == "" {
		return errors.New("配置人工批准缺少 permission request_id")
	}
	if s.humanVerifier == nil {
		return errors.New("配置人工批准缺少 human principal verifier")
	}
	principal, err := s.humanVerifier.VerifyInteractiveHuman(
		ctx,
		authctx.PrincipalFromContext(ctx),
	)
	if err != nil {
		return err
	}
	ctx = authctx.WithPrincipal(ctx, principal)
	request, err := changeRequestFromApprovedTool(approval.ToolInput)
	if err != nil {
		return err
	}
	actor, err := s.actorFromHumanApproval(ctx, approval)
	if err != nil {
		return err
	}
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return err
	}
	plan, err := s.planChange(ctx, resolved, request)
	if err != nil {
		return err
	}
	if request.PlanDigest != plan.PlanDigest ||
		request.ExpectedRevision != plan.CurrentRevision {
		return errors.New("批准的配置 plan digest 或 revision 已失效")
	}
	if !plan.RequiresConfirmation {
		return nil
	}
	if !sameSecretSlots(plan.SecretSlots, approval.ConfigurationSecretSlots) {
		return errors.New("批准卡中的 secret slot 与当前配置计划不匹配")
	}
	if _, materializeErr := secretinput.MaterializeJSON(
		request.Input,
		approval.ConfigurationSecrets,
	); materializeErr != nil {
		return materializeErr
	}

	now := s.humanApprovalNow()
	expiresAt := approval.ExpiresAt.UTC()
	maximumExpiry := now.Add(maximumHumanApprovalLifetime)
	if expiresAt.IsZero() || expiresAt.After(maximumExpiry) {
		expiresAt = maximumExpiry
	}
	if !expiresAt.After(now) {
		return errors.New("配置批准请求已过期")
	}
	record := humanApprovalRecord{
		PermissionRequestID:    strings.TrimSpace(approval.PermissionRequestID),
		OwnerUserID:            resolved.OwnerUserID,
		PrincipalRole:          resolved.PrincipalRole,
		PrincipalAuthMethod:    resolved.AuthMethod,
		PrincipalAuthSessionID: resolved.AuthSessionID,
		AgentID:                resolved.AgentID,
		SessionKey:             resolved.SessionKey,
		RoundID:                resolved.RoundID,
		LeaseSessionKey:        resolved.LeaseSessionKey,
		LeaseRoundID:           resolved.LeaseRoundID,
		ContextKind:            resolved.ContextKind,
		ContextID:              resolved.ContextID,
		RoomID:                 resolved.RoomID,
		ConversationID:         resolved.ConversationID,
		Scope:                  plan.Scope,
		RequestID:              request.RequestID,
		PlanDigest:             plan.PlanDigest,
		ExpectedRevision:       plan.CurrentRevision,
		Domain:                 plan.Domain,
		Operation:              plan.Operation,
		Target:                 plan.Target,
		ApprovedAt:             now,
		ExpiresAt:              expiresAt,
		ConfigurationSecrets:   cloneSecretValues(approval.ConfigurationSecrets),
	}
	key := humanApprovalKey(record.OwnerUserID, record.RequestID)
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	s.purgeExpiredHumanApprovalsLocked(now)
	if existing, ok := s.humanApprovals[key]; ok {
		if existing.PermissionRequestID != record.PermissionRequestID ||
			!sameHumanApprovalIntent(existing, record) {
			clear(record.ConfigurationSecrets)
			return errors.New("request_id 已绑定另一项人工批准计划；请生成新的 request_id")
		}
		clear(existing.ConfigurationSecrets)
	}
	s.humanApprovals[key] = record
	return nil
}

func isConfigurationApplyApprovalTool(toolName string) bool {
	const leaf = "apply_nexus_configuration_change"
	toolName = strings.TrimSpace(toolName)
	if toolName == leaf {
		return true
	}
	for _, separator := range []string{"__", ".", "/"} {
		if strings.HasSuffix(toolName, separator+leaf) {
			return true
		}
	}
	return false
}

func (s *Service) actorFromHumanApproval(
	ctx context.Context,
	approval permissionctx.HumanToolApproval,
) (Actor, error) {
	principal := authctx.PrincipalFromContext(ctx)
	ownerUserID := ""
	role := ""
	authMethod := ""
	localSingleUser := false
	authSessionID := ""
	if principal != nil {
		ownerUserID = strings.TrimSpace(principal.UserID)
		role = strings.TrimSpace(principal.Role)
		authMethod = strings.TrimSpace(principal.AuthMethod)
		if principal.SessionID != nil {
			authSessionID = strings.TrimSpace(*principal.SessionID)
		}
	} else if authctx.IsLocalSingleUserControlPlane(ctx, authctx.SystemUserID) {
		ownerUserID = authctx.SystemUserID
		role = authctx.RoleOwner
		authMethod = authctx.AuthMethodLocal
		localSingleUser = true
	}
	if ownerUserID == "" {
		return Actor{}, errors.New("配置人工批准缺少已认证 principal")
	}

	agentID := strings.TrimSpace(approval.Route.AgentID)
	if agentID == "" {
		agentID = protocol.ParseSessionKey(approval.RuntimeSessionKey).AgentID
	}
	if agentID == "" {
		return Actor{}, errors.New("配置人工批准缺少可信 agent_id")
	}
	sessionKey := strings.TrimSpace(approval.DispatchSessionKey)
	roundID := strings.TrimSpace(approval.Route.RoundID)
	if sessionKey == "" || roundID == "" {
		return Actor{}, errors.New("配置人工批准缺少可信 session/round")
	}
	leaseSessionKey := strings.TrimSpace(approval.RuntimeSessionKey)
	leaseRoundID := roundID
	roomID := strings.TrimSpace(approval.Route.RoomID)
	if roomID != "" {
		leaseRoundID = strings.TrimSpace(approval.Route.AgentRoundID)
	}
	if leaseSessionKey == "" || leaseRoundID == "" {
		return Actor{}, errors.New("配置人工批准缺少可信 runtime session/round lease")
	}

	actor := Actor{
		OwnerUserID: ownerUserID, AgentID: agentID,
		SessionKey: sessionKey, RoundID: roundID,
		LeaseSessionKey: leaseSessionKey, LeaseRoundID: leaseRoundID,
		PrincipalRole: role, AuthMethod: authMethod,
		AuthSessionID:   authSessionID,
		LocalSingleUser: localSingleUser, RoundLeaseRequired: true,
	}
	if roomID != "" {
		actor.ContextKind = ContextKindRoom
		actor.ContextID = roomID
		actor.RoomID = roomID
		actor.ConversationID = strings.TrimSpace(approval.Route.ConversationID)
		actor.SourceContext = ContextKindRoom + ":" + roomID
		return actor, nil
	}
	actor.ContextKind = ContextKindAgent
	actor.ContextID = agentID
	actor.SourceContext = ContextKindAgent + ":" + agentID
	return actor, nil
}

func changeRequestFromApprovedTool(input map[string]any) (ChangeRequest, error) {
	rawInput := any(map[string]any{})
	if input != nil && input["input"] != nil {
		rawInput = input["input"]
	}
	payload, err := json.Marshal(rawInput)
	if err != nil {
		return ChangeRequest{}, fmt.Errorf("编码批准 input: %w", err)
	}
	request := ChangeRequest{
		RequestID:        approvedString(input, "request_id"),
		Domain:           approvedString(input, "domain"),
		Operation:        approvedString(input, "operation"),
		Target:           approvedString(input, "target"),
		Input:            payload,
		ExpectedRevision: approvedString(input, "expected_revision"),
		PlanDigest:       approvedString(input, "plan_digest"),
	}
	if !requestIDPattern.MatchString(request.RequestID) {
		return ChangeRequest{}, errors.New("人工批准中的 request_id 无效")
	}
	if request.PlanDigest == "" || request.ExpectedRevision == "" {
		return ChangeRequest{}, errors.New("人工批准缺少 plan_digest 或 expected_revision")
	}
	return request, nil
}

func approvedString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}

func (s *Service) consumeHumanApproval(
	actor *resolvedActor,
	request ChangeRequest,
	plan ChangePlan,
) (*humanApprovalRecord, error) {
	if actor == nil {
		return nil, errors.New("配置人工批准缺少可信 actor")
	}
	key := humanApprovalKey(actor.OwnerUserID, request.RequestID)
	now := s.humanApprovalNow()
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	s.purgeExpiredHumanApprovalsLocked(now)
	record, ok := s.humanApprovals[key]
	if !ok {
		return nil, errors.New("该高风险配置变更尚未获得当前会话中的人工允许；请在配置确认卡中选择“允许本次”")
	}
	expected := humanApprovalRecord{
		OwnerUserID: actor.OwnerUserID, PrincipalRole: actor.PrincipalRole,
		PrincipalAuthMethod:    actor.AuthMethod,
		PrincipalAuthSessionID: actor.AuthSessionID,
		AgentID:                actor.AgentID, SessionKey: actor.SessionKey, RoundID: actor.RoundID,
		LeaseSessionKey: actor.LeaseSessionKey, LeaseRoundID: actor.LeaseRoundID,
		ContextKind: actor.ContextKind, ContextID: actor.ContextID,
		RoomID: actor.RoomID, ConversationID: actor.ConversationID,
		Scope: plan.Scope, RequestID: request.RequestID,
		PlanDigest: plan.PlanDigest, ExpectedRevision: plan.CurrentRevision,
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
	}
	if !sameHumanApprovalIntent(record, expected) {
		return nil, errors.New("人工批准与当前 principal、会话、作用域、计划或 revision 不匹配")
	}
	delete(s.humanApprovals, key)
	copyRecord := record
	return &copyRecord, nil
}

func humanApprovalKey(ownerUserID string, requestID string) string {
	return strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(requestID)
}

func sameHumanApprovalIntent(left humanApprovalRecord, right humanApprovalRecord) bool {
	return left.OwnerUserID == right.OwnerUserID &&
		left.PrincipalRole == right.PrincipalRole &&
		left.PrincipalAuthMethod == right.PrincipalAuthMethod &&
		left.PrincipalAuthSessionID == right.PrincipalAuthSessionID &&
		left.AgentID == right.AgentID &&
		left.SessionKey == right.SessionKey &&
		left.RoundID == right.RoundID &&
		left.LeaseSessionKey == right.LeaseSessionKey &&
		left.LeaseRoundID == right.LeaseRoundID &&
		left.ContextKind == right.ContextKind &&
		left.ContextID == right.ContextID &&
		left.RoomID == right.RoomID &&
		left.ConversationID == right.ConversationID &&
		left.Scope == right.Scope &&
		left.RequestID == right.RequestID &&
		left.PlanDigest == right.PlanDigest &&
		left.ExpectedRevision == right.ExpectedRevision &&
		left.Domain == right.Domain &&
		left.Operation == right.Operation &&
		left.Target == right.Target
}

func (s *Service) purgeExpiredHumanApprovalsLocked(now time.Time) {
	for key, record := range s.humanApprovals {
		if !record.ExpiresAt.After(now) {
			clear(record.ConfigurationSecrets)
			delete(s.humanApprovals, key)
		}
	}
}

func sameSecretSlots(left []secretinput.Slot, right []secretinput.Slot) bool {
	if len(left) != len(right) {
		return false
	}
	expected := make(map[string]string, len(left))
	for _, slot := range left {
		expected[slot.ID] = slot.Path
	}
	for _, slot := range right {
		if expected[slot.ID] != slot.Path {
			return false
		}
	}
	return true
}

func cloneSecretValues(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func (s *Service) humanApprovalNow() time.Time {
	if s.approvalNow != nil {
		return s.approvalNow().UTC()
	}
	return time.Now().UTC()
}
