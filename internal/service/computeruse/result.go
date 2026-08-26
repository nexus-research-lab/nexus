// INPUT: Nexus lifecycle errors and official nexus-cua typed errors.
// OUTPUT: stable Agent-facing runtimecommand results without secret or payload reflection.
// POS: Computer Use error taxonomy and safe response projection boundary.
package computeruse

import (
	"errors"
	"strings"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
)

type commandError struct {
	code     string
	message  string
	recovery string
}

func (err *commandError) Error() string { return err.message }

func newCommandError(code, message, recovery string) error {
	return &commandError{code: code, message: strings.TrimSpace(message), recovery: strings.TrimSpace(recovery)}
}

func computerResult(data map[string]any, message string) runtimecommand.Result {
	if data == nil {
		data = map[string]any{}
	}
	if _, exists := data["outcome"]; !exists {
		data["outcome"] = "applied"
	}
	return runtimecommand.Result{
		StructuredContent: data,
		Content:           []map[string]any{{"type": "text", "text": strings.TrimSpace(message)}},
	}
}

func computerErrorResult(err error) runtimecommand.Result {
	code := "computer_use_failed"
	message := "Computer Use operation failed"
	retryable := false
	recovery := "inspect Computer Use state and obtain fresh desktop state before continuing"
	mutationStatus := string(nexuscua.MutationNotApplicable)

	var cuaError *nexuscua.CUAError
	var indeterminate *nexuscua.MutationIndeterminateError
	var commandFailure *commandError
	switch {
	case errors.As(err, &commandFailure):
		code = commandFailure.code
		message = commandFailure.message
		recovery = commandFailure.recovery
	case errors.As(err, &indeterminate):
		code = "mutation_indeterminate"
		message = "The desktop action outcome is indeterminate"
		retryable = true
		recovery = "retry the exact same perform_action input with the same Nexus request ID"
		mutationStatus = string(nexuscua.MutationIndeterminate)
	case errors.As(err, &cuaError):
		code = string(cuaError.Code)
		message = safeCUAMessage(cuaError)
		retryable = cuaError.Retryable
		mutationStatus = string(cuaError.MutationStatus)
		if cuaError.RecoveryAction != nil && strings.TrimSpace(*cuaError.RecoveryAction) != "" {
			recovery = strings.TrimSpace(*cuaError.RecoveryAction)
		} else if cuaError.MutationStatus == nexuscua.MutationIndeterminate {
			recovery = "retry the exact same perform_action input with the same Nexus request ID"
		}
	case errors.Is(err, ErrDisabled):
		code, message, recovery = "disabled", ErrDisabled.Error(), "enable Computer Use in Settings"
	case errors.Is(err, ErrUnavailable):
		code, message, recovery = "unavailable", ErrUnavailable.Error(), "use a supported Nexus desktop host"
	case errors.Is(err, ErrNotInstalled):
		code, message, recovery = "not_installed", ErrNotInstalled.Error(), "install the pinned Computer Use runtime in Settings"
	case errors.Is(err, ErrProtocolMismatch):
		code, message, recovery = "protocol_mismatch", ErrProtocolMismatch.Error(), "install the Nexus-pinned Computer Use runtime version"
	case errors.Is(err, ErrRestartThrottled):
		code, message, recovery = "restart_throttled", ErrRestartThrottled.Error(), "run doctor and wait before restarting the runtime"
	case errors.Is(err, ErrEpochChanged):
		code, message = "epoch_changed", ErrEpochChanged.Error()
		recovery = "select the target again in the fresh sidecar epoch; never replay the old action"
		mutationStatus = string(nexuscua.MutationIndeterminate)
	case errors.Is(err, ErrSidecarNotReady):
		code, message, recovery = "sidecar_not_ready", ErrSidecarNotReady.Error(), "run Computer Use doctor and restart the runtime"
	default:
	}
	return runtimecommand.Result{
		IsError: true,
		StructuredContent: map[string]any{
			"outcome": "rejected", "reason_code": code, "message": message,
			"retryable": retryable, "recovery_action": recovery, "mutation_status": mutationStatus,
		},
		Content: []map[string]any{{"type": "text", "text": message + ". Recovery: " + recovery}},
	}
}

func safeCUAMessage(err *nexuscua.CUAError) string {
	if err == nil {
		return "Computer Use runtime rejected the operation"
	}
	message := strings.TrimSpace(err.Message)
	if message == "" || len(message) > 512 {
		return "Computer Use runtime rejected the operation: " + string(err.Code)
	}
	return message
}
