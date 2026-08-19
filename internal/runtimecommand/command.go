// INPUT: Agent runtime 通过受管 nexus CLI 提交的领域、操作、严格 JSON 输入与稳定 request identity。
// OUTPUT: Goal、Execution 与 Automation 共用的 contract/inspect/invoke wire，以及 transport-neutral operation result。
// POS: Skill、CLI、loopback broker 与领域 command adapter 之间的线格式真相；不依赖 MCP 或 Provider tool schema。
package runtimecommand

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

const (
	DomainAutomation = "automation"
	DomainGoal       = "goal"
	DomainExecution  = "execution"
)

const (
	ActionContract = "contract"
	ActionInspect  = "inspect"
	ActionInvoke   = "invoke"
	ActionPlan     = "plan"
	ActionApply    = "apply"
	ActionReplay   = "replay"
)

// Request 是 nexus CLI 到宿主 command broker 的唯一请求 envelope。
type Request struct {
	Domain           string         `json:"domain"`
	Action           string         `json:"action"`
	Operation        string         `json:"operation,omitempty"`
	Input            map[string]any `json:"input,omitempty"`
	RequestID        string         `json:"request_id,omitempty"`
	ExpectedRevision string         `json:"expected_revision,omitempty"`
	PlanDigest       string         `json:"plan_digest,omitempty"`
}

// OperationContract 是按需返回给 Skill 的单操作 contract。
type OperationContract struct {
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Idempotent  bool           `json:"idempotent,omitempty"`
}

// Contract 是一个领域稳定的操作目录。目录不携带 schema；指定 operation 时
// 只返回该项及其精确输入 schema。
type Contract struct {
	Domain     string              `json:"domain"`
	Inspect    string              `json:"inspect_operation,omitempty"`
	Operations []OperationContract `json:"operations"`
}

// CallContext 只包含宿主生成或验证过的调用 identity；领域输入不能覆盖。
type CallContext struct {
	// RequestID 是 CLI broker 验证过的稳定 request_id。
	RequestID string
	SessionID string
}

// Result 是领域 command 的稳定结果。StructuredContent 是机器面，Content 是模型可读面。
type Result struct {
	Content           []map[string]any `json:"content,omitempty"`
	StructuredContent map[string]any   `json:"data,omitempty"`
	IsError           bool             `json:"is_error,omitempty"`
}

// Operation 是 transport-neutral 的模型语义操作。
type Operation struct {
	Name           string
	Description    string
	SearchHint     string
	InputSchema    map[string]any
	ReadOnly       bool
	Idempotent     bool
	Annotations    *OperationAnnotations
	Handler        func(context.Context, map[string]any) (Result, error)
	ContextHandler func(context.Context, map[string]any, *CallContext) (Result, error)
}

// OperationAnnotations 是 transport-neutral 的读写/幂等提示。
type OperationAnnotations struct {
	ReadOnlyHint    bool
	ReadOnly        bool
	IdempotentHint  bool
	DestructiveHint bool
}

// Invoke 执行操作并保留 request identity。
func (o Operation) Invoke(ctx context.Context, input map[string]any, call *CallContext) (Result, error) {
	if err := ValidateInput(o.InputSchema, input); err != nil {
		return Result{}, fmt.Errorf("%s input %w", strings.TrimSpace(o.Name), err)
	}
	if o.ContextHandler != nil {
		return o.ContextHandler(ctx, input, call)
	}
	if o.Handler != nil {
		return o.Handler(ctx, input)
	}
	return Result{IsError: true, Content: []map[string]any{{"type": "text", "text": "runtime command handler is nil"}}}, nil
}

// MarshalJSONInput 把严格输入重编码给领域 parser。
func MarshalJSONInput(input map[string]any) ([]byte, error) {
	if input == nil {
		input = map[string]any{}
	}
	return json.Marshal(input)
}

func FindOperation(operations []Operation, name string) (Operation, bool) {
	name = strings.TrimSpace(name)
	for _, operation := range operations {
		if strings.TrimSpace(operation.Name) == name {
			return operation, true
		}
	}
	return Operation{}, false
}

func ValidRequestID(value string) bool {
	return requestIDPattern.MatchString(strings.TrimSpace(value))
}

func BuildContract(domain, inspect, selected string, operations []Operation) (Contract, error) {
	contract := Contract{Domain: strings.TrimSpace(domain), Inspect: strings.TrimSpace(inspect)}
	selected = strings.TrimSpace(selected)
	includeSchema := selected != ""
	for _, operation := range operations {
		if selected != "" && operation.Name != selected {
			continue
		}
		kind := "mutation"
		if operation.ReadOnly || operation.Annotations != nil &&
			(operation.Annotations.ReadOnly || operation.Annotations.ReadOnlyHint) {
			kind = "query"
		}
		definition := OperationContract{
			Name: operation.Name, Kind: kind, Description: operation.Description,
			Idempotent: operation.Idempotent || operation.Annotations != nil && operation.Annotations.IdempotentHint,
		}
		if includeSchema {
			definition.InputSchema = operation.InputSchema
		}
		contract.Operations = append(contract.Operations, definition)
	}
	if selected != "" && len(contract.Operations) == 0 {
		return Contract{}, fmt.Errorf("unknown %s operation %q", contract.Domain, selected)
	}
	return contract, nil
}
