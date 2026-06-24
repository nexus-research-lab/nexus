package room

import (
	"fmt"
	"log/slog"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

func roomRuntimeStartupLogFields(
	options agentclient.Options,
	runtimeSelection runtimeselectionsvc.Selection,
	runtimeProvider string,
	slot *activeRoomSlot,
) []any {
	return append(clientopts.RuntimeStartupLogFields(options),
		"agent_id", slot.AgentID,
		"agent_round_id", slot.AgentRoundID,
		"runtime_session_key", slot.RuntimeSessionKey,
		"requested_runtime_kind", strings.TrimSpace(runtimeSelection.RuntimeKind),
		"requested_provider", strings.TrimSpace(runtimeSelection.Provider),
		"requested_model", strings.TrimSpace(runtimeSelection.Model),
		"runtime_provider", runtimeProvider,
	)
}

func roomRuntimeConnectFailureLogFields(
	options agentclient.Options,
	runtimeSelection runtimeselectionsvc.Selection,
	runtimeProvider string,
	slot *activeRoomSlot,
	err error,
) []any {
	return append(roomRuntimeStartupLogFields(options, runtimeSelection, runtimeProvider, slot),
		"stage", "connect",
		"err", err,
		"error_type", fmt.Sprintf("%T", err),
		"transport_closed", runtimectx.IsRuntimeTransportClosedError(err),
	)
}

func withRoomRuntimeDiagnosticsLogger(options agentclient.Options, logger *slog.Logger) agentclient.Options {
	previousStderr := options.Callbacks.Stderr
	options.Callbacks.Stderr = func(line string) {
		normalizedLine := runtimectx.NormalizeRuntimeStderrLine(line)
		if previousStderr != nil {
			previousStderr(normalizedLine)
		}
		logger.Debug("Agent SDK stderr", "stderr", normalizedLine)
	}
	previousDiagnostics := options.Callbacks.Diagnostics
	diagnosticsEnabled := runtimectx.AgentSDKDiagnosticsEnabled(options.Env)
	options.Callbacks.Diagnostics = func(event agentclient.DiagnosticEvent) {
		if previousDiagnostics != nil {
			previousDiagnostics(event)
		}
		if diagnosticsEnabled {
			logger.Info("Agent SDK diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldLogRuntimeStartupDiagnostic(event) {
			logger.Info("Agent SDK startup diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
			return
		}
		if clientopts.ShouldWarnRuntimeStartupDiagnostic(event) {
			logger.Warn("Agent SDK startup diagnostics",
				"component", strings.TrimSpace(event.Component),
				"event", strings.TrimSpace(event.Event),
				"attrs", clientopts.SanitizeRuntimeDiagnosticAttributes(event.Event, event.Attributes),
			)
		}
	}
	if !diagnosticsEnabled {
		return options
	}
	logger.Info("Agent SDK diagnostics 已启用",
		"diagnostics_env", runtimectx.AgentSDKDiagnosticsValue(options.Env),
		"provider_debug_body", runtimectx.AgentSDKProviderDebugBodyValue(options.Env),
	)
	return options
}
