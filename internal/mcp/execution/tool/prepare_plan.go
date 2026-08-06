// INPUT: 单个完整 Nexus Plan Document string 与 trusted current scope/round identity。
// OUTPUT: strict validation 后的 sealed proposal id、full-fence digest 与 commit 指引。
// POS: Provider 稳定文本传输到 durable non-authoritative proposal 的模型适配入口。
package tool

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type planTransportGuard struct {
	emptyPrepare atomic.Uint32
	emptyCommit  atomic.Uint32
}

func preparePlanExecution(
	svc contract.Service,
	sctx contract.ServerContext,
	guards ...*planTransportGuard,
) sdktool.Tool {
	const toolName = "prepare_plan_execution"
	guard := selectPlanTransportGuard(guards)
	return sdktool.Tool{
		Name: toolName,
		Description: "Validate and durably seal one complete Nexus Plan Document v1 without mutating Execution, Plan, or Goal state. " +
			"Pass the entire YAML document in plan_document as one non-empty string. The root keys are nexus_plan, operation, objective, completion_criteria, optional revision controls, and items. " +
			"Each item declares logical_key, kind, subject, objective, deliverable, acceptance_criteria, required, terminal, parent_logical_key, depends_on, soft_depends_on, input_refs, output_scopes, and shared_output_scopes. " +
			"Unknown keys, duplicate keys, aliases, custom tags, multiple documents, placeholders, invalid graphs, and stale target boundaries are rejected. " +
			"On success, call plan_execution once with the returned proposal_id and proposal_digest. Preparation is allowed in Plan Mode because the proposal is non-authoritative and recoverable across rounds/restarts.",
		SearchHint:  "prepare validate plan document yaml work graph proposal dependencies",
		InputSchema: preparePlanExecutionSchema(),
		Annotations: &sdktool.ToolAnnotations{IdempotentHint: true},
		ContextHandler: func(
			ctx context.Context,
			input map[string]any,
			callContext *sdktool.CallContext,
		) (sdktool.ToolResult, error) {
			var parsed preparePlanExecutionInput
			if err := decodeInput(input, &parsed); err != nil {
				return transportErrorResult(err), nil
			}
			if strings.TrimSpace(parsed.PlanDocument) == "" {
				return malformedPlanTransportResult(
					toolName,
					"plan_document is required and must be non-empty",
					guard.emptyPrepare.Add(1),
				), nil
			}
			guard.emptyPrepare.Store(0)
			prepareCommandID, err := commandID(sctx, callContext, toolName, input, 0)
			if err != nil {
				return transportErrorResult(err), nil
			}
			proposal, err := svc.PreparePlanExecution(
				ctx,
				sctx.Actor(),
				orchestration.PreparePlanExecutionInput{
					CommandID:    prepareCommandID,
					PlanDocument: parsed.PlanDocument,
				},
			)
			if err != nil {
				var domainErr *orchestration.DomainError
				if errors.As(err, &domainErr) {
					return mutationResult(orchestration.RejectedResult(nil, err, []orchestration.NextAction{{
						Tool:   toolName,
						Reason: "repair the complete Plan Document using the reported validation error",
					}})), nil
				}
				return transportErrorResult(err), nil
			}
			return jsonResult(map[string]any{
				"outcome":                  "prepared",
				"proposal_id":              proposal.ID,
				"proposal_digest":          proposal.ContentDigest,
				"proposal_status":          proposal.Status,
				"operation":                proposal.Document.Operation,
				"target_execution_id":      emptyStringToNil(proposal.TargetExecutionID),
				"target_execution_version": proposal.TargetExecutionVersion,
				"base_plan_id":             emptyStringToNil(proposal.BasePlanID),
				"goal_id":                  emptyStringToNil(proposal.GoalID),
				"goal_objective_revision":  proposal.GoalObjectiveRevision,
				"item_count":               len(proposal.Document.Items),
				"message":                  "Complete Plan proposal is sealed; commit it without changing the document.",
				"next_actions": []orchestration.NextAction{{
					Tool:   "plan_execution",
					Reason: "pass this exact proposal_id and proposal_digest to atomically materialize the sealed Plan",
				}},
			}), nil
		},
	}
}

func malformedPlanTransportResult(toolName, message string, attempt uint32) sdktool.ToolResult {
	actions := []orchestration.NextAction{{
		Tool:   toolName,
		Reason: "retry once with the required non-empty scalar fields; do not send {} or placeholder values",
	}}
	if attempt > 1 {
		message += "; repeated empty input indicates a provider tool-argument transport failure, so stop retrying this tool in the current round"
		actions = nil
	}
	return mutationResult(orchestration.RejectedResult(
		nil,
		errors.New(strings.TrimSpace(message)),
		actions,
	))
}

func selectPlanTransportGuard(guards []*planTransportGuard) *planTransportGuard {
	for _, guard := range guards {
		if guard != nil {
			return guard
		}
	}
	return &planTransportGuard{}
}

func emptyStringToNil(value string) any {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return nil
}
