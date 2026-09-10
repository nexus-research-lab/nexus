// INPUT: 宿主固定 Actor、原生控制回调与 closed structured command。
// OUTPUT: 按需 contract 与同 round request_id 去重的子智能体操作。
// POS: 父智能体 Subagent 命令协议；不替代 SDK 生命周期或 WorkGraph 准入。
package subagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type toolUseKey struct{}

// WithToolUseID 接收 SDK SDKMCPServer 可信 metadata，业务 input 不提供此字段。
func WithToolUseID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, toolUseKey{}, id)
}

type receipt struct {
	digest [32]byte
	done   chan struct{}
	value  any
	err    error
}

// NewHandler 每个 physical round 构建一次；失败或取消也保留回执，禁止猜测后重放派生。
func NewHandler(actor command.Actor, control runtimectx.SubagentControl) command.Handler {
	var mu sync.Mutex
	receipts := map[string]*receipt{}
	operations := definitions()
	return func(ctx context.Context, req command.Request) (any, error) {
		if !actor.Valid() || strings.HasSuffix(actor.SourceContextType, "_untrusted") || strings.HasPrefix(actor.SourceContextType, "workgraph_") {
			return nil, errors.New("当前来源没有 Subagent command authority")
		}
		if req.Action == command.ActionContract {
			return command.BuildContract(command.DomainSubagent, "list", req.Operation, operations)
		}
		if req.ExpectedRevision != "" || req.PlanDigest != "" {
			return nil, errors.New("subagent 不接受 expected_revision 或 plan_digest")
		}
		operation := req.Operation
		if req.Action == command.ActionInspect {
			if operation != "" {
				return nil, errors.New("subagent inspect 不接受 operation")
			}
			operation = "list"
		} else if req.Action != command.ActionInvoke {
			return nil, errors.New("subagent 只支持 contract、inspect、invoke")
		}
		if req.Action == command.ActionInvoke && operation == "list" {
			return nil, errors.New("subagent list 固定使用 action=inspect")
		}
		op, ok := command.FindOperation(operations, operation)
		if !ok {
			return nil, fmt.Errorf("未知 subagent operation %q", operation)
		}
		if err := command.ValidateInput(op.InputSchema, req.Input); err != nil {
			return nil, err
		}
		for _, key := range []string{"description", "prompt", "task_id", "message"} {
			if v, ok := req.Input[key].(string); ok && strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("%s 不能为空", key)
			}
		}
		toolID, _ := ctx.Value(toolUseKey{}).(string)
		if toolID == "" || control == nil {
			return nil, errors.New("当前调用缺少原生 Subagent control 或 SDK tool identity")
		}
		invoke := func() (any, error) {
			payload, err := control(ctx, toolID, operation, req.Input)
			if err != nil {
				return nil, err
			}
			text, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			failed, _ := payload["is_error"].(bool)
			return command.Result{Content: []map[string]any{{"type": "text", "text": string(text)}}, StructuredContent: payload, IsError: failed}, nil
		}
		if op.ReadOnly {
			return invoke()
		}
		if err := command.ValidateRequestID(req.RequestID); err != nil {
			return nil, fmt.Errorf("subagent mutation %w", err)
		}
		data, _ := json.Marshal(struct {
			Operation string
			Input     map[string]any
		}{operation, req.Input})
		digest := sha256.Sum256(data)
		mu.Lock()
		if old := receipts[req.RequestID]; old != nil {
			mu.Unlock()
			if old.digest != digest {
				return nil, errors.New("同一 request_id 不能用于不同 subagent 意图")
			}
			select {
			case <-old.done:
				return old.value, old.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if len(receipts) >= 128 {
			mu.Unlock()
			return nil, errors.New("本轮 Subagent mutation 回执已达上限")
		}
		current := &receipt{digest: digest, done: make(chan struct{})}
		receipts[req.RequestID] = current
		mu.Unlock()
		current.value, current.err = invoke()
		close(current.done)
		return current.value, current.err
	}
}

// definitions 将单操作 schema 留在 contract 响应，不扩张顶层 MCP schema。
func definitions() []command.Operation {
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	schema := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	task := map[string]any{"task_id": text("inspect 或 spawn 返回的当前父会话子任务 ID")}
	return []command.Operation{
		{Name: "list", Description: "列出当前父会话的子智能体；通过 action=inspect 调用", ReadOnly: true, InputSchema: schema(map[string]any{})},
		{Name: "spawn", Description: "异步派生一个通用子智能体；返回 task ID，完成事件自动通知。工作图绑定由宿主解析", InputSchema: schema(map[string]any{"description": text("简短任务标题"), "prompt": text("完整任务、范围、已有证据及期望结果")}, "description", "prompt")},
		{Name: "get", Description: "读取一个子智能体的当前输出与状态", ReadOnly: true, InputSchema: schema(task, "task_id")},
		{Name: "wait", Description: "等待一个子智能体完成，最多 30 秒；超时不表示失败", ReadOnly: true, InputSchema: schema(task, "task_id")},
		{Name: "send", Description: "向当前父会话子智能体追加消息或继续任务", InputSchema: schema(map[string]any{"task_id": task["task_id"], "message": text("补充说明或后续任务")}, "task_id", "message")},
		{Name: "stop", Description: "停止当前父会话的指定子智能体", InputSchema: schema(task, "task_id")},
	}
}
