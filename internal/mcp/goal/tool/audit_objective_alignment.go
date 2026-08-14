// INPUT: 当前 Goal 的服务端目标边界与模型以单个 JSON string 提交的逐项证据判定。
// OUTPUT: durable Objective Alignment record、三态 decision 与下一步生命周期建议。
// POS: 共享 objectivealignment 契约的 Goal MCP 入口；只审计，不完成 Goal。
package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/objectivealignment"
)

type auditObjectiveAlignmentInput struct {
	ReportJSON string `json:"report_json"`
}

var auditObjectiveAlignmentDescription = strings.TrimSpace(
	"Audit the current Goal against its backend-authoritative objective and completion criteria.\n" +
		"Use this immediately before completing a Goal whose managed WorkGraph binding is confirmed, after inspecting current authoritative evidence. Goal-only and reserved Goals do not require this audit.\n" +
		"This tool records a three-state evidence report; it does not complete, block, retarget, or otherwise transition the Goal.\n" +
		"Submit report_json as one JSON object string, not as nested tool arguments. " +
		objectivealignment.ReportJSONDescription,
)

func auditObjectiveAlignment(
	svc contract.Service,
	sctx contract.ServerContext,
) sdktool.Tool {
	return sdktool.Tool{
		Name:        "audit_objective_alignment",
		Description: auditObjectiveAlignmentDescription,
		SearchHint:  searchHintAuditGoal,
		InputSchema: objectSchema(map[string]any{
			"report_json": stringProperty(
				"Required. " + objectivealignment.ReportJSONDescription +
					" Serialize the entire object into this one string.",
			),
		}, "report_json"),
		Handler: func(
			ctx context.Context,
			input map[string]any,
		) (sdktool.ToolResult, error) {
			expectedRevision := sctx.ExpectedGoalObjectiveRevision()
			var parsed auditObjectiveAlignmentInput
			if err := decodeInput(input, &parsed); err != nil {
				return errorResult(err), nil
			}
			report, err := decodeObjectiveAlignmentReport(parsed.ReportJSON)
			if err != nil {
				return errorResult(err), nil
			}
			if sctx.PlanMode {
				return planModeGoalMutationResult("audit_objective_alignment"), nil
			}
			current, err := currentGoalForMutation(ctx, svc, sctx, expectedRevision)
			if err != nil {
				return updateGoalCurrentErrorResult(err), nil
			}
			record, err := svc.AuditObjectiveAlignmentByModel(
				ctx,
				current.ID,
				protocol.AuditGoalObjectiveAlignmentRequest{
					Report:                    report,
					RoundID:                   sctx.CurrentRoundID,
					AgentID:                   sctx.CurrentAgentID,
					ExpectedObjectiveRevision: expectedRevision,
				},
			)
			if err != nil {
				return errorResult(err), nil
			}
			return structuredResult(
				"objective alignment audited",
				objectiveAlignmentPayload(current, record),
			), nil
		},
	}
}

func decodeObjectiveAlignmentReport(
	raw string,
) (protocol.ObjectiveAlignmentReport, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"report_json is required and must contain one JSON object",
		)
	}
	var report protocol.ObjectiveAlignmentReport
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"report_json must be an Objective Alignment JSON object: %w",
			err,
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
				"report_json must contain exactly one JSON object",
			)
		}
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"report_json contains trailing invalid JSON: %w",
			err,
		)
	}
	return report, nil
}

func objectiveAlignmentPayload(
	item *protocol.Goal,
	record *protocol.GoalObjectiveAlignmentRecord,
) map[string]any {
	payload := goalPayload(item)
	payload["outcome"] = protocol.MutationResultApplied
	if record == nil {
		payload["objectiveAlignment"] = nil
		return payload
	}
	payload["objectiveAlignment"] = map[string]any{
		"id":                record.ID,
		"objectiveRevision": record.ObjectiveRevision,
		"roundId":           record.RoundID,
		"decision":          record.Report.Decision,
		"auditedAt":         record.AuditedAt,
	}
	switch record.Report.Decision {
	case protocol.ObjectiveAlignmentAligned:
		payload["nextAction"] = map[string]any{
			"tool":   "update_goal",
			"status": "complete",
			"reason": "the current objective revision is aligned in this round; Goal completion still applies its WorkGraph and Room readiness gates",
		}
	case protocol.ObjectiveAlignmentNotAligned:
		payload["nextAction"] = map[string]any{
			"action": "continue_work",
			"reason": "close every reported objective gap before auditing again",
		}
	default:
		payload["nextAction"] = map[string]any{
			"action": "gather_evidence",
			"reason": "collect the missing or conflicting authoritative evidence before auditing again",
		}
	}
	return payload
}
