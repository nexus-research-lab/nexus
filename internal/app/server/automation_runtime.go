// INPUT: Automation service、可信 runtime round、loopback CLI 请求与 runtime permission context。
// OUTPUT: nxs/Claude 共用的 Nexus Automation CLI capability 环境和命令响应。
// POS: Agent-facing Automation command broker；身份、跨 Agent 权限和真人确认均由宿主固定。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

const maxRuntimeAutomationRequestBytes = 1 << 20

func newAutomationRuntimeEnvironmentBuilder(
	cfg config.Config,
	svc *automationsvc.Service,
	agents runtimeAgentResolver,
) func(
	context.Context,
	*protocol.Agent,
	string,
	string,
	string,
	string,
	string,
	*protocol.AutomationRunContext,
) (map[string]string, error) {
	endpoint := automationRuntimeBrokerURL(cfg)
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		automationRun *protocol.AutomationRunContext,
	) (map[string]string, error) {
		if svc == nil || agents == nil || agentValue == nil {
			return nil, nil
		}
		agentID := strings.TrimSpace(agentValue.AgentID)
		lease, hasLease := runtimectx.MCPRoundLeaseFromContext(ctx)
		if agentID == "" || !hasLease ||
			strings.TrimSpace(lease.SessionKey) == "" || strings.TrimSpace(lease.RoundID) == "" {
			return nil, nil
		}
		record, err := agents.GetAgent(ctx, agentID)
		if err != nil || record == nil || strings.TrimSpace(record.OwnerUserID) == "" ||
			strings.TrimSpace(record.AgentID) != agentID {
			return nil, err
		}
		actor := automationsvc.RuntimeCommandActor{
			OwnerUserID: strings.TrimSpace(record.OwnerUserID),
			AgentID:     agentID, AgentName: strings.TrimSpace(record.Name),
			SessionKey: strings.TrimSpace(sessionKey), RoundID: strings.TrimSpace(roundID),
			LeaseSessionKey: strings.TrimSpace(lease.SessionKey), LeaseRoundID: strings.TrimSpace(lease.RoundID),
			SourceContextType:  strings.ToLower(strings.TrimSpace(sourceContextType)),
			SourceContextID:    strings.TrimSpace(sourceContextID),
			SourceContextLabel: strings.TrimSpace(sourceContextLabel),
			SessionLabel:       strings.TrimSpace(sourceContextLabel),
			DefaultTimezone:    strings.TrimSpace(cfg.DefaultTimezone),
		}
		if normalized := normalizedAutomationRunContext(automationRun); normalized != nil {
			actor.SourceContextType = "automation_run"
			actor.SourceContextID = normalized.JobID
			actor.SourceContextLabel = normalized.JobName
			actor.CurrentJobID = normalized.JobID
			actor.CurrentRunID = normalized.RunID
		} else if !trustedAutomationRuntimeActor(ctx, record, actor) {
			// 仍签发只读 capability；mutation 在 service 中按来源 fail closed。
			actor.SourceContextType = strings.TrimSuffix(actor.SourceContextType, "_untrusted") + "_untrusted"
		}
		actor.IsMainAgent = record.IsMain && trustedMainAutomationRuntime(actor)
		token, err := svc.IssueRuntimeCommandCapability(actor)
		if err != nil {
			return nil, err
		}
		environment := map[string]string{
			protocol.NexusCommandBrokerURLEnvName:       endpoint,
			protocol.NexusCommandCapabilityTokenEnvName: token,
		}
		inputPath, cleanup, stagingErr := prepareAutomationRuntimeInput(
			actor.OwnerUserID, actor.LeaseRoundID, token,
		)
		if stagingErr != nil {
			return nil, stagingErr
		}
		environment[protocol.NexusAutomationInputPathEnvName] = inputPath
		context.AfterFunc(ctx, cleanup)
		return environment, nil
	}
}

func normalizedAutomationRunContext(value *protocol.AutomationRunContext) *protocol.AutomationRunContext {
	if value == nil {
		return nil
	}
	normalized := value.Normalized()
	if !normalized.Valid() {
		return nil
	}
	return &normalized
}

func trustedAutomationRuntimeActor(
	ctx context.Context,
	agent *protocol.Agent,
	actor automationsvc.RuntimeCommandActor,
) bool {
	if agent == nil || strings.TrimSpace(actor.SessionKey) == "" ||
		strings.TrimSpace(actor.RoundID) == "" ||
		actor.RoundID != actor.LeaseRoundID {
		return false
	}
	switch actor.SourceContextType {
	case "agent":
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRuntimeRoute(
			actor.AgentID, "agent", actor.SessionKey, actor.RoundID,
			actor.LeaseSessionKey, actor.LeaseRoundID,
		)
		return ok && actor.SourceContextID == actor.AgentID
	case "room":
		if _, _, _, ok := trustedRuntimePrincipal(ctx, actor.OwnerUserID); !ok {
			return false
		}
		_, ok := trustedRuntimeRoute(
			actor.AgentID, "room", actor.SessionKey, actor.RoundID,
			actor.LeaseSessionKey, actor.LeaseRoundID,
		)
		return ok && actor.SourceContextID != ""
	case "agent_paired":
		parsed := protocol.ParseSessionKey(actor.SessionKey)
		return actor.SessionKey == actor.LeaseSessionKey && parsed.IsStructured &&
			parsed.Kind == protocol.SessionKeyKindAgent &&
			parsed.ChatType == protocol.RoomTypeDM &&
			strings.TrimSpace(parsed.AgentID) == actor.AgentID &&
			protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelWebSocket &&
			protocol.NormalizeStoredChannelType(parsed.Channel) != protocol.SessionChannelInternalSegment
	default:
		return false
	}
}

func trustedMainAutomationRuntime(actor automationsvc.RuntimeCommandActor) bool {
	if actor.SourceContextType != "agent" || actor.SessionKey != actor.LeaseSessionKey {
		return false
	}
	parsed := protocol.ParseSessionKey(actor.SessionKey)
	return parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.Channel == protocol.SessionChannelWebSocketSegment &&
		parsed.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(parsed.AgentID) == strings.TrimSpace(actor.AgentID)
}

func automationRuntimeBrokerURL(cfg config.Config) string {
	prefix := "/" + strings.Trim(strings.TrimSpace(cfg.APIPrefix), "/")
	if prefix == "/" {
		prefix = ""
	}
	return (&url.URL{
		Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Path: prefix + "/internal/runtime/automation",
	}).String()
}

func newRuntimeAutomationHandler(
	svc *automationsvc.Service,
	permissions *permissionctx.Context,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if svc == nil || !runtimeConfigurationLoopbackRequest(request) {
			writeRuntimeAutomationError(writer, http.StatusForbidden, "Automation broker 只接受本机请求")
			return
		}
		actor, err := svc.ResolveRuntimeCommandCapability(
			request.Header.Get(protocol.NexusCommandCapabilityHeader),
		)
		if err != nil {
			writeRuntimeAutomationError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		var command automationdomain.AutomationCommandRequest
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRuntimeAutomationRequestBytes))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&command); err != nil {
			writeRuntimeAutomationError(writer, http.StatusBadRequest, "请求参数错误: "+err.Error())
			return
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeRuntimeAutomationError(writer, http.StatusBadRequest, "请求只能包含一个 JSON 对象")
			return
		}
		var result any
		switch strings.ToLower(strings.TrimSpace(command.Action)) {
		case automationdomain.AutomationCommandActionContract:
			result, err = svc.RuntimeCommandContract(request.Context(), actor)
		case automationdomain.AutomationCommandActionInspect:
			result, err = svc.InspectRuntimeCommand(request.Context(), actor, command.Operation, command.Input)
		case automationdomain.AutomationCommandActionPlan:
			result, err = svc.PlanRuntimeCommand(request.Context(), actor, command.Operation, command.Input)
		case automationdomain.AutomationCommandActionReplay:
			var replayed *automationdomain.AutomationCommandApplyResult
			var found bool
			replayed, found, err = svc.ReplayRuntimeCommand(request.Context(), actor, command)
			result = automationdomain.AutomationCommandReplayResult{Found: found, Result: replayed}
		case automationdomain.AutomationCommandActionApply:
			var replayed *automationdomain.AutomationCommandApplyResult
			var found bool
			replayed, found, err = svc.ReplayRuntimeCommand(request.Context(), actor, command)
			if err == nil && found {
				result = replayed
				break
			}
			if err == nil {
				var plan *automationdomain.AutomationCommandPlan
				plan, err = svc.PlanRuntimeCommand(request.Context(), actor, command.Operation, command.Input)
				approvalRequestID := ""
				if err == nil {
					approvalRequestID, err = requireRuntimeAutomationConfirmation(request.Context(), permissions, actor, *plan)
				}
				if err == nil {
					result, err = svc.ApplyRuntimeCommand(
						request.Context(), actor, command,
						automationsvc.RuntimeCommandApplyOptions{
							HumanConfirmed: true, HumanApprovalRequestID: approvalRequestID,
						},
					)
				}
			}
		default:
			err = errors.New("未知 Nexus Automation command action")
		}
		if err != nil {
			writeRuntimeAutomationError(writer, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeRuntimeAutomationJSON(writer, http.StatusOK, map[string]any{
			"success": true, "data": result,
		})
	}
}

func requireRuntimeAutomationConfirmation(
	ctx context.Context,
	permissions *permissionctx.Context,
	actor automationsvc.RuntimeCommandActor,
	plan automationdomain.AutomationCommandPlan,
) (string, error) {
	if !plan.RequiresConfirmation {
		return "not-required", nil
	}
	if permissions == nil {
		return "", errors.New("Automation 真人确认服务尚未装配")
	}
	decision, requestID, err := permissions.RequestPermissionWithID(ctx, actor.LeaseSessionKey, sdkpermission.Request{
		ToolName: "nexus_automation_apply",
		Input:    runtimeAutomationConfirmationInput(plan),
		Title:    "确认 Nexus 自动化变更", DisplayName: "Nexus Automation",
		Description: plan.Summary,
	})
	if err != nil {
		return requestID, err
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		message := strings.TrimSpace(decision.Message)
		if message == "" {
			message = "用户未批准 Automation 变更"
		}
		return requestID, errors.New(message)
	}
	return requestID, nil
}

func runtimeAutomationConfirmationInput(plan automationdomain.AutomationCommandPlan) map[string]any {
	input := map[string]any{
		"operation":   plan.Operation,
		"target":      plan.Target,
		"summary":     plan.Summary,
		"risk":        plan.Risk,
		"revision":    plan.CurrentRevision,
		"plan_digest": plan.PlanDigest,
	}
	changes := map[string]any{}
	add := func(name string, value any, present bool) {
		if present {
			changes[name] = value
		}
	}
	command := plan.Input
	add("name", command.Name, strings.TrimSpace(command.Name) != "")
	add("instruction", command.Instruction, strings.TrimSpace(command.Instruction) != "")
	add("instruction_append", command.InstructionAdd, strings.TrimSpace(command.InstructionAdd) != "")
	add("schedule", command.Schedule, command.Schedule != nil)
	add("agent_id", command.AgentID, strings.TrimSpace(command.AgentID) != "")
	add("context_mode", command.ContextMode, strings.TrimSpace(command.ContextMode) != "")
	add("deliver_result", command.DeliverResult, command.DeliverResult != nil)
	add("permission_mode", command.PermissionMode, strings.TrimSpace(command.PermissionMode) != "")
	add("overlap_policy", command.OverlapPolicy, strings.TrimSpace(command.OverlapPolicy) != "")
	add("expires_at", command.ExpiresAt, strings.TrimSpace(command.ExpiresAt) != "")
	add("clear_expires_at", command.ClearExpiresAt, command.ClearExpiresAt)
	add("enabled", command.Enabled, command.Enabled != nil)
	add("cancel_active_run", command.CancelActiveRun, command.CancelActiveRun)
	add("run_id", command.RunID, strings.TrimSpace(command.RunID) != "")
	add("every_seconds", command.EverySeconds, command.EverySeconds > 0)
	add("target_mode", command.TargetMode, strings.TrimSpace(command.TargetMode) != "")
	add("ack_max_chars", command.AckMaxChars, command.AckMaxChars != nil)
	add("wake_mode", command.Mode, strings.TrimSpace(command.Mode) != "")
	add("text", command.Text, strings.TrimSpace(command.Text) != "")
	if len(changes) > 0 {
		input["changes"] = changes
	}
	return input
}

func writeRuntimeAutomationError(writer http.ResponseWriter, status int, message string) {
	writeRuntimeAutomationJSON(writer, status, map[string]any{
		"success": false, "error": map[string]any{"message": strings.TrimSpace(message)},
	})
}

func writeRuntimeAutomationJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
