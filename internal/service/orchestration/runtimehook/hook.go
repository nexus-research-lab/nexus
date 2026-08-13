// INPUT: Orchestration subagent admission provider、当前 actor 与 SDK hook payload。
// OUTPUT: runtime.Manager 可热切换的 callbacks、managed/runtime-only 放行、脱敏拒绝结果与内部持久化错误日志。
// POS: service 领域结果到 bridge hook wire semantics 的适配层；无 managed binding 不等于禁止原生能力。
package runtimehook

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

const subagentAdmissionErrorCode = "subagent_admission_error"

// Provider 是 runtime hook 适配器消费的最小 Orchestration 能力。
type Provider interface {
	AdmitSubagentLaunch(
		context.Context,
		orchestration.ActorContext,
		orchestration.SubagentLaunchInput,
	) (orchestration.SubagentAdmissionResult, error)
	ObserveSubagentStart(
		context.Context,
		orchestration.ActorContext,
		orchestration.SubagentLifecycleInput,
	) (orchestration.SubagentAdmissionResult, error)
	ObserveSubagentStop(
		context.Context,
		orchestration.ActorContext,
		orchestration.SubagentLifecycleInput,
	) (orchestration.SubagentAdmissionResult, error)
	ObserveSubagentParentRoundExit(
		context.Context,
		orchestration.ActorContext,
		orchestration.SubagentParentRoundExitInput,
	) (orchestration.SubagentAdmissionResult, error)
}

// Context 保留不属于 ActorContext 的物理 runtime/Room session identity。
type Context struct {
	Actor             orchestration.ActorContext
	ActorProvider     func() orchestration.ActorContext
	RuntimeSessionKey string
	RoomSessionID     string
	Logger            *slog.Logger
}

func (c Context) currentActor() orchestration.ActorContext {
	if c.ActorProvider != nil {
		return c.ActorProvider()
	}
	return c.Actor
}

// Callbacks 把当前 round identity 闭包化；Manager 在 warm session 上动态替换它。
func Callbacks(provider Provider, value Context) runtimectx.SubagentHookCallbacks {
	if provider == nil {
		return runtimectx.SubagentHookCallbacks{}
	}
	return runtimectx.SubagentHookCallbacks{
		PreToolUse: func(
			ctx context.Context,
			input sdkhook.Input,
			toolUseID string,
		) (sdkhook.Output, error) {
			result, err := provider.AdmitSubagentLaunch(ctx, value.currentActor(), orchestration.SubagentLaunchInput{
				ToolUseID:         firstValue(toolUseID, input.ToolUseID),
				RuntimeSessionKey: value.RuntimeSessionKey,
				RoomSessionID:     value.RoomSessionID,
				SDKSessionID:      input.SessionID,
			})
			return admissionOutput(ctx, value.Logger, sdkhook.EventPreToolUse, result, err), nil
		},
		PostToolUseFailure: func(
			ctx context.Context,
			input sdkhook.Input,
			toolUseID string,
		) (sdkhook.Output, error) {
			result, err := provider.ObserveSubagentStop(ctx, value.currentActor(), orchestration.SubagentLifecycleInput{
				ToolUseID:    firstValue(toolUseID, input.ToolUseID),
				SDKSessionID: input.SessionID,
				SDKAgentID:   input.AgentID,
				AgentType:    input.AgentType,
				Interrupted:  input.IsInterrupt,
				Error:        firstValue(input.Error, input.ErrorDetails, "Agent tool failed before subagent completion"),
			})
			return admissionOutput(ctx, value.Logger, sdkhook.EventPostToolUseFailure, result, err), nil
		},
		SubagentStart: func(
			ctx context.Context,
			input sdkhook.Input,
			_ string,
		) (sdkhook.Output, error) {
			result, err := provider.ObserveSubagentStart(ctx, value.currentActor(), orchestration.SubagentLifecycleInput{
				SDKSessionID: input.SessionID,
				SDKAgentID:   input.AgentID,
				AgentType:    input.AgentType,
			})
			return admissionOutput(ctx, value.Logger, sdkhook.EventSubagentStart, result, err), nil
		},
		SubagentStop: func(
			ctx context.Context,
			input sdkhook.Input,
			_ string,
		) (sdkhook.Output, error) {
			result, err := provider.ObserveSubagentStop(ctx, value.currentActor(), orchestration.SubagentLifecycleInput{
				SDKSessionID:         input.SessionID,
				SDKAgentID:           input.AgentID,
				AgentType:            input.AgentType,
				AgentTranscriptPath:  input.AgentTranscriptPath,
				LastAssistantMessage: input.LastAssistantMessage,
				Interrupted:          input.IsInterrupt,
				Error:                firstValue(input.Error, input.ErrorDetails),
			})
			return admissionOutput(ctx, value.Logger, sdkhook.EventSubagentStop, result, err), nil
		},
		ParentRoundExit: func(
			ctx context.Context,
			input runtimectx.SubagentRoundExitInput,
		) error {
			result, err := provider.ObserveSubagentParentRoundExit(
				ctx,
				value.currentActor(),
				orchestration.SubagentParentRoundExitInput{
					ToolUseID:           input.ToolUseID,
					SDKSessionID:        input.SDKSessionID,
					SDKAgentID:          input.SDKAgentID,
					SDKTaskID:           input.SDKTaskID,
					ParentRoundExitedAt: input.ParentRoundExitedAt,
					ReconcileAfter:      input.ReconcileAfter,
				},
			)
			if err != nil {
				return err
			}
			if !result.Allowed {
				return fmt.Errorf(
					"subagent reconciliation schedule rejected: [%s] %s",
					result.ReasonCode,
					result.Message,
				)
			}
			return nil
		},
	}
}

func admissionOutput(
	ctx context.Context,
	logger *slog.Logger,
	event sdkhook.Event,
	result orchestration.SubagentAdmissionResult,
	err error,
) sdkhook.Output {
	if err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.ErrorContext(
			ctx,
			"Subagent admission persistence failed",
			"hook_event", event,
			"error", err,
		)
		return runtimectx.DenySubagentHookOutput(
			event,
			subagentAdmissionErrorCode,
			"authoritative subagent admission state could not be persisted",
		)
	}
	if !result.Allowed {
		reasonCode := string(result.ReasonCode)
		message := result.Message
		return runtimectx.DenySubagentHookOutput(event, reasonCode, message)
	}
	return sdkhook.Output{}
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
