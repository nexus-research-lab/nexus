// INPUT: Host-bound Actor, registered semantic operations and command transport results.
// OUTPUT: Best-effort start/finish diagnostics with bounded labels and hashed correlation IDs.
// POS: Read-only command measurements; never an authority, mutation receipt or retry signal.
package command

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Diagnostics is immutable and shared by concurrent calls in one physical round.
// Correlation hashes are diagnostic identifiers, not authentication credentials.
type Diagnostics struct {
	logger    *slog.Logger
	scopeHash string
	roundHash string
}

// NewDiagnostics receives the application base logger, without request-scoped
// attributes; raw caller identity is replaced by hashes before emitting fields.
func NewDiagnostics(logger *slog.Logger, actor Actor) *Diagnostics {
	if logger == nil {
		logger = slog.Default()
	}
	scope := diagnosticHash(actor.OwnerUserID, actor.SessionKey, actor.AgentID)
	return &Diagnostics{logger: logger, scopeHash: scope, roundHash: diagnosticHash(scope, actor.LeaseSessionKey, actor.LeaseRoundID)}
}

type diagnosticKey struct{}

type diagnosticCall struct {
	collector *Diagnostics
	started   time.Time
	id        string
	domain    string
	action    string
	operation string
	request   string
	requestID string
	failure   string
	status    string
}

func (d *Diagnostics) start(ctx context.Context, input map[string]any) (context.Context, *diagnosticCall) {
	if d == nil {
		return ctx, nil
	}
	call := &diagnosticCall{
		collector: d, started: time.Now(), id: rand.Text(),
		domain:    diagnosticLabel(input["domain"], DomainExecution, DomainGoal, DomainAutomation, DomainSubagent),
		action:    diagnosticLabel(input["action"], ActionContract, ActionInspect, ActionInvoke, ActionPlan, ActionApply),
		operation: "unknown", requestID: "missing",
	}
	if value, exists := input["request_id"]; exists {
		call.requestID = "invalid"
		if id, ok := value.(string); ok && requestIDPattern.MatchString(id) {
			call.requestID = "valid"
			call.request = diagnosticHash(d.scopeHash, call.domain, id)
		}
	}
	call.emit(ctx, "start", "unknown", "unknown", "not_reported", nil)
	return context.WithValue(ctx, diagnosticKey{}, call), call
}

// Registry names are trusted host labels. Never use a raw model operation as a
// diagnostic label, even when an error includes that operation in its message.
func observeSemanticOperation(ctx context.Context, request Request, inspect string, operations []Operation) {
	call, _ := ctx.Value(diagnosticKey{}).(*diagnosticCall)
	if call == nil {
		return
	}
	name := request.Operation
	if request.Action == ActionContract && name == "" {
		call.operation = "catalog"
		return
	}
	if request.Action == ActionInspect {
		name = inspect
	}
	if operation, ok := FindOperation(operations, name); ok {
		call.operation = operation.Name
	}
}

func observeDiagnosticError(ctx context.Context, reason string, err error) {
	call, _ := ctx.Value(diagnosticKey{}).(*diagnosticCall)
	if call == nil {
		return
	}
	call.failure, call.status = reason, "error"
	var invalid *OperationInputError
	switch {
	case errors.Is(err, context.Canceled):
		call.failure, call.status = "cancelled", "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		call.failure, call.status = "deadline_exceeded", "deadline_exceeded"
	case errors.Is(err, errInvalidRequestID):
		call.failure = "invalid_request_id"
	case errors.As(err, &invalid):
		call.failure = "invalid_input"
	}
}

func (c *diagnosticCall) finish(ctx context.Context, result sdktool.ToolResult, returned bool) {
	if c == nil {
		return
	}
	status, outcome, reason := "ok", "unknown", "not_reported"
	if !returned {
		status, reason = "incomplete", "handler_incomplete"
	} else if c.failure != "" {
		status, reason = c.status, c.failure
	} else if result.IsError {
		status, reason = "error", "result_error"
	} else {
		outcome = diagnosticLabel(result.StructuredContent["outcome"],
			string(protocol.MutationResultApplied), string(protocol.MutationResultNoOp),
			string(protocol.MutationResultRejected), string(protocol.MutationResultSuperseded))
		reason = diagnosticReason(result.StructuredContent["reason_code"])
	}
	durationMS := float64(time.Since(c.started)) / float64(time.Millisecond)
	c.emit(ctx, "finish", status, outcome, reason, &durationMS)
}

func (c *diagnosticCall) emit(ctx context.Context, phase, status, outcome, reason string, durationMS *float64) {
	// A diagnostic writer failure (including a broken custom slog Handler) must
	// not replace a domain result or suppress a domain panic.
	defer func() { _ = recover() }()
	attrs := []slog.Attr{
		slog.String("event", "nexus.command.call"), slog.Int("schema_version", 1), slog.String("phase", phase),
		slog.String("call_id", c.id), slog.String("scope_hash", c.collector.scopeHash), slog.String("round_hash", c.collector.roundHash),
		slog.String("domain", c.domain), slog.String("action", c.action), slog.String("operation", c.operation),
		slog.String("request_id_state", c.requestID), slog.String("request_hash", c.request),
		slog.String("status", status), slog.String("outcome", outcome), slog.String("reason_code", reason),
	}
	if durationMS != nil {
		attrs = append(attrs, slog.Float64("host_duration_ms", *durationMS))
	}
	c.collector.logger.LogAttrs(ctx, slog.LevelInfo, "nexus.command.call", attrs...)
}

func diagnosticHash(parts ...string) string {
	encoded, _ := json.Marshal(parts)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:16])
}

func diagnosticLabel(value any, allowed ...string) string {
	text, ok := value.(string)
	if ok {
		for _, candidate := range allowed {
			if text == candidate {
				return candidate
			}
		}
	}
	return "unknown"
}

// This is a deliberately small measurement vocabulary, not the wire contract.
// New/unrecognized domain codes remain visible as "other" without logging text.
func diagnosticReason(value any) string {
	if value == nil || value == "" {
		return "not_reported"
	}
	reason := diagnosticLabel(value, "invalid_input", "stale_execution", "wrong_owner", "wrong_reviewer",
		"dependency_not_accepted", "work_binding_mismatch", "review_binding_required", "no_current_execution",
		"plan_mode", "execution_terminal", "duplicate_assignment", "duplicate_attempt", "plan_proposal_stale",
		"plan_proposal_blocked", "plan_document_invalid", "completion_blocked", "projection_limit_exceeded")
	if reason == "unknown" {
		return "other"
	}
	return reason
}
