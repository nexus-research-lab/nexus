// INPUT: Nexus 自有命令定义、会话作用域与原始 Slash 文本。
// OUTPUT: 安全目录描述、稳定命令匹配和宿主 handler 执行结果。
// POS: Nexus host command 的唯一注册与派发边界；不接管 runtime 命令。
package slashcommand

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Scope 表示 Nexus 命令可出现的会话类型。
type Scope string

const (
	ScopeDM   Scope = "dm"
	ScopeRoom Scope = "room"
)

// Invocation 是宿主命令 handler 可消费的稳定调用输入。
type Invocation struct {
	SessionKey      string
	AgentID         string
	RoundID         string
	UserMessageID   string
	ClientRequestID string
	ClientMessageID string
	Content         string
	Arguments       string
	TargetAgentIDs  []string
	GoalOptions     protocol.GoalCommandOptions
	AttachmentCount int
}

// DirectoryInvalidation 描述宿主命令完成后需要广播的目录失效信号。
type DirectoryInvalidation struct {
	Reason string
	Data   map[string]any
}

// Result 承载宿主命令执行后需要直接返回当前客户端的瞬时事件。
type Result struct {
	Events                 []protocol.EventMessage
	DirectoryInvalidation  *DirectoryInvalidation
	UserMessageCommitted   bool
	AfterResponseAttempted func(context.Context)
}

// Handler 执行一条 Nexus 自有命令。
type Handler func(context.Context, Invocation) (Result, error)

// Authorizer 在 host handler 执行前校验调用方是否有权使用当前会话。
type Authorizer func(context.Context, Invocation) error

// Definition 描述 Nexus 自有命令。Name 不带前导斜杠。
type Definition struct {
	Name           string
	Description    string
	ArgumentHint   string
	Scopes         []Scope
	Enabled        bool
	DisabledReason string
	Handler        Handler
}

// Registry 保存 Nexus 自有命令；runtime 命令由上层在投影时合并。
type Registry struct {
	definitions map[string]Definition
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{definitions: map[string]Definition{}}
}

// Register 注册一条 Nexus 命令并拒绝模糊名称或重复定义。
func (r *Registry) Register(definition Definition) error {
	if r == nil {
		return errors.New("slash command registry is nil")
	}
	name, ok := normalizeCommandName(definition.Name)
	if !ok {
		return errors.New("slash command name must be one non-empty token")
	}
	if len(definition.Scopes) == 0 {
		return errors.New("slash command scope is required")
	}
	if r.definitions == nil {
		r.definitions = map[string]Definition{}
	}
	if _, exists := r.definitions[name]; exists {
		return errors.New("slash command is already registered")
	}
	definition.Name = name
	definition.Description = strings.TrimSpace(definition.Description)
	definition.ArgumentHint = strings.TrimSpace(definition.ArgumentHint)
	definition.DisabledReason = strings.TrimSpace(definition.DisabledReason)
	definition.Scopes = normalizeScopes(definition.Scopes)
	if len(definition.Scopes) == 0 {
		return errors.New("slash command scope is invalid")
	}
	if definition.Enabled && definition.Handler == nil {
		return errors.New("enabled slash command handler is required")
	}
	r.definitions[name] = definition
	return nil
}

// Descriptors 返回指定会话作用域的浏览器安全目录。
func (r *Registry) Descriptors(scope Scope) []protocol.CommandDescriptor {
	if r == nil {
		return nil
	}
	result := make([]protocol.CommandDescriptor, 0, len(r.definitions))
	for _, definition := range r.definitions {
		if !supportsScope(definition.Scopes, scope) {
			continue
		}
		result = append(result, protocol.CommandDescriptor{
			Name:           definition.Name,
			Description:    definition.Description,
			ArgumentHint:   definition.ArgumentHint,
			Execution:      protocol.CommandExecutionHost,
			Enabled:        definition.Enabled,
			DisabledReason: definition.DisabledReason,
		})
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Name < result[right].Name
	})
	return result
}

// Execute 匹配并执行 Nexus 自有命令；未命中时由调用方继续交给 runtime。
func (r *Registry) Execute(
	ctx context.Context,
	scope Scope,
	invocation Invocation,
) (Result, bool, error) {
	return r.execute(ctx, scope, invocation, nil)
}

// ExecuteAuthorized 在命令匹配成功后先执行调用方鉴权，再进入 host handler。
// 未匹配的 Slash 仍返回 matched=false，交由 runtime 原样解释。
func (r *Registry) ExecuteAuthorized(
	ctx context.Context,
	scope Scope,
	invocation Invocation,
	authorize Authorizer,
) (Result, bool, error) {
	return r.execute(ctx, scope, invocation, authorize)
}

func (r *Registry) execute(
	ctx context.Context,
	scope Scope,
	invocation Invocation,
	authorize Authorizer,
) (Result, bool, error) {
	name, arguments, ok := parseInvocation(invocation.Content)
	if !ok || r == nil {
		return Result{}, false, nil
	}
	definition, exists := r.definitions[name]
	if !exists || !supportsScope(definition.Scopes, scope) {
		return Result{}, false, nil
	}
	invocation.Content = strings.TrimSpace(invocation.Content)
	invocation.Arguments = arguments
	if authorize != nil {
		if err := authorize(ctx, invocation); err != nil {
			return Result{}, true, err
		}
	}
	if !definition.Enabled {
		reason := definition.DisabledReason
		if reason == "" {
			reason = "当前 Slash 指令暂不可用。"
		}
		return Result{}, true, commandInputError{message: reason}
	}
	if invocation.AttachmentCount > 0 {
		return Result{}, true, commandInputError{
			message: "Nexus Slash 指令必须作为独立文本发送，请先移除附件。",
		}
	}
	result, err := definition.Handler(ctx, invocation)
	return result, true, err
}

type commandInputError struct {
	message string
}

func (e commandInputError) Error() string {
	return e.message
}

func (e commandInputError) ClientMessage() string {
	return e.message
}

func parseInvocation(content string) (string, string, bool) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/") {
		return "", "", false
	}
	fields := strings.Fields(strings.TrimPrefix(content, "/"))
	if len(fields) == 0 {
		return "", "", false
	}
	name, ok := normalizeCommandName(fields[0])
	if !ok {
		return "", "", false
	}
	arguments := strings.TrimSpace(strings.TrimPrefix(
		strings.TrimPrefix(content, "/"),
		fields[0],
	))
	return name, arguments, true
}

func normalizeCommandName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(strings.TrimLeft(name, "/")))
	return name, name != "" && len(strings.Fields(name)) == 1
}

func normalizeScopes(scopes []Scope) []Scope {
	seen := map[Scope]struct{}{}
	result := make([]Scope, 0, len(scopes))
	for _, scope := range scopes {
		if scope != ScopeDM && scope != ScopeRoom {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result
}

func supportsScope(scopes []Scope, target Scope) bool {
	for _, scope := range scopes {
		if scope == target {
			return true
		}
	}
	return false
}
