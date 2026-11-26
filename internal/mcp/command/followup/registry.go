// INPUT: 宿主固定的 round Actor（session、agent）与模型提交的跟进输入。
// OUTPUT: 主窗口跟进的 inspect/invoke 操作目录与 applied/rejected 结果。
// POS: 跟进的唯一模型适配层；模型不能指定目标会话，跟进只落在当前 round 所在的主窗口。
package followup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

// InspectOperation 是 follow_up domain 的固定 inspect 操作。
const InspectOperation = "list_follow_ups"

// Service 是 dm service 提供的跟进能力。
type Service interface {
	ScheduleFollowUp(context.Context, string, string, dmsvc.FollowUpRequest) (dmsvc.FollowUp, error)
	ListFollowUps(context.Context, string, string) ([]dmsvc.FollowUp, error)
	CancelFollowUp(context.Context, string, string, string) error
}

type registry struct {
	service    Service
	sessionKey string
	agentID    string
}

// BuildAll 返回绑定当前主窗口 round 的跟进操作。
func BuildAll(service Service, sessionKey string, agentID string) []command.Operation {
	r := registry{service: service, sessionKey: sessionKey, agentID: agentID}
	return []command.Operation{r.list(), r.schedule(), r.cancel()}
}

func (r registry) list() command.Operation {
	return command.Operation{
		Name:        InspectOperation,
		Description: "List follow-ups you scheduled in this main window that have not fired yet, ordered by due time.",
		SearchHint:  "follow up list pending reminder 跟进 待触发 列表",
		InputSchema: objectSchema(map[string]any{}),
		ReadOnly:    true,
		Annotations: &command.OperationAnnotations{ReadOnlyHint: true, ReadOnly: true},
		Handler: func(ctx context.Context, _ map[string]any) (command.Result, error) {
			items, err := r.service.ListFollowUps(ctx, r.sessionKey, r.agentID)
			if err != nil {
				return rejected(err), nil
			}
			return structured(map[string]any{"follow_ups": items}), nil
		},
	}
}

type scheduleInput struct {
	AfterMinutes int    `json:"after_minutes,omitempty"`
	At           string `json:"at,omitempty"`
	Note         string `json:"note"`
}

func (r registry) schedule() command.Operation {
	return command.Operation{
		Name: "schedule_follow_up",
		Description: "Schedule a one-off follow-up to yourself in this main window. At the due time the host wakes this conversation with your note as a <follow-up> input, so you can check back on something unfinished (a running job, a pending reply, a promised update). " +
			"Give exactly one of after_minutes or at; the due time must be 5 minutes to 7 days ahead. At most 10 can be pending. Use scheduled tasks instead for recurring work. The note is all you will see when it fires: state what to check and why.",
		SearchHint: "follow up later remind check back schedule 稍后 跟进 回头看 提醒 自己",
		InputSchema: objectSchema(map[string]any{
			"after_minutes": map[string]any{"type": "integer", "description": "Minutes from now, 5 to 10080. Omit when using at."},
			"at":            map[string]any{"type": "string", "description": "Absolute due time in RFC3339 with offset, e.g. 2026-11-26T15:30:00+08:00. Omit when using after_minutes."},
			"note":          map[string]any{"type": "string", "description": "Required. What to do when it fires, written for your future self; at most 1000 characters."},
		}, "note"),
		ContextHandler: func(ctx context.Context, input map[string]any, call *command.CallContext) (command.Result, error) {
			var parsed scheduleInput
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			request := dmsvc.FollowUpRequest{
				After: time.Duration(parsed.AfterMinutes) * time.Minute,
				Note:  parsed.Note,
			}
			if call != nil {
				request.RequestKey = call.RequestID
			}
			if parsed.At != "" {
				at, err := time.Parse(time.RFC3339, parsed.At)
				if err != nil {
					return rejected(fmt.Errorf("at 必须是带时区的 RFC3339 时间: %w", err)), nil
				}
				request.At = at
			}
			item, err := r.service.ScheduleFollowUp(ctx, r.sessionKey, r.agentID, request)
			if err != nil {
				return rejected(err), nil
			}
			return applied(map[string]any{"follow_up": item}), nil
		},
	}
}

type cancelInput struct {
	FollowUpID string `json:"follow_up_id"`
}

func (r registry) cancel() command.Operation {
	return command.Operation{
		Name:        "cancel_follow_up",
		Description: "Cancel a pending follow-up by follow_up_id when it is no longer needed. Follow-ups that already fired cannot be cancelled.",
		SearchHint:  "follow up cancel remove 取消 跟进",
		InputSchema: objectSchema(map[string]any{
			"follow_up_id": map[string]any{"type": "string", "description": "Required. The follow_up_id from list_follow_ups or schedule_follow_up."},
		}, "follow_up_id"),
		Idempotent: true,
		Handler: func(ctx context.Context, input map[string]any) (command.Result, error) {
			var parsed cancelInput
			if err := decodeInput(input, &parsed); err != nil {
				return rejected(err), nil
			}
			if err := r.service.CancelFollowUp(ctx, r.sessionKey, r.agentID, parsed.FollowUpID); err != nil {
				return rejected(err), nil
			}
			return applied(map[string]any{"follow_up_id": parsed.FollowUpID}), nil
		},
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
		"required":             append([]string{}, required...),
	}
}

func decodeInput(input map[string]any, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func structured(data map[string]any) command.Result {
	payload, _ := json.Marshal(data)
	return command.Result{
		Content:           []map[string]any{{"type": "text", "text": string(payload)}},
		StructuredContent: data,
	}
}

func applied(data map[string]any) command.Result {
	data["outcome"] = string(protocol.MutationResultApplied)
	return structured(data)
}

func rejected(err error) command.Result {
	data := map[string]any{"outcome": string(protocol.MutationResultRejected), "message": err.Error()}
	result := structured(data)
	result.IsError = true
	return result
}
