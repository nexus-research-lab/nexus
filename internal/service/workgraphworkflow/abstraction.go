// INPUT: 完整实际 WorkGraph 的责任语义、依赖与非内容型执行信号。
// OUTPUT: 默认后台模型自动抽取的最小通用草图、命名 Slash 与 key/collaboration 角色。
// POS: WorkGraph 草图预览前的强制抽象边界；模型失败、虚构节点或结构漂移时失败关闭。
package workgraphworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

const (
	abstractionTimeout      = 45 * time.Second
	abstractionMaxTokens    = 4096
	abstractionSystemPrompt = `你是 WorkGraph 结构提炼器。请根据一张已经实际执行完成的责任图，提取可跨 Session、跨主题复用的最小结构草图。
严格要求：
1. 自动选择真正构成可复用骨架的 Work Item；省略一次性细节、偶发分支和仅服务于具体课题的节点。用户不会手动选节点。
2. 输出节点必须是输入 logical_key 的非空子集；不得增加、合并、拆分或虚构节点。至少保留一条 key 主路径和一个 terminal 最终交付。
3. 删除具体课题、专有名词、章节名、文件路径、项目名、时间与一次性数据；保留阶段目的、交付物类型和可验证标准。
4. key 表示即使单 Agent 执行也必须保留的主路径、验证或最终交付。
5. collaboration 只表示实际分工、跨 owner 交接、独立复核或汇总整合边界；参考 delegated、assignment_strategy、independent_review 等实际信号，普通并行节点不是天然 collaboration。
6. slash_name 必须是通用英文 kebab-case，匹配 ^[a-z][a-z0-9-]{0,63}$，不能含具体课题名。
7. 验收条件应保持严格但改写为与主题无关的可验证表述。
8. 只输出一个 JSON 对象，不要 Markdown、解释或代码围栏。
JSON 结构：{"slash_name":"...","title":"...","description":"...","objective":"...","completion_criteria":["..."],"nodes":[{"logical_key":"...","role":"key|collaboration","subject":"...","objective":"...","deliverable":"...","acceptance_criteria":["..."]}]}`
)

// AbstractionSourceNode 是用于结构抽取的安全节点输入；只保留责任语义和粗粒度执行信号。
type AbstractionSourceNode struct {
	LogicalKey            string                               `json:"logical_key"`
	Kind                  protocol.WorkItemKind                `json:"kind"`
	Subject               string                               `json:"subject"`
	Objective             string                               `json:"objective"`
	Deliverable           string                               `json:"deliverable"`
	AcceptanceCriteria    []string                             `json:"acceptance_criteria,omitempty"`
	Required              bool                                 `json:"required"`
	Terminal              bool                                 `json:"terminal,omitempty"`
	ParentLogicalKey      string                               `json:"parent_logical_key,omitempty"`
	DependencyLogicalKeys []string                             `json:"dependency_logical_keys,omitempty"`
	Status                protocol.ExecutionWorkItemViewStatus `json:"status,omitempty"`
	AssignmentStrategy    protocol.AssignmentStrategy          `json:"assignment_strategy,omitempty"`
	Delegated             bool                                 `json:"delegated,omitempty"`
	IndependentReview     bool                                 `json:"independent_review,omitempty"`
	AttemptCount          int                                  `json:"attempt_count,omitempty"`
}

// AbstractionInput 不包含 Tool、身份、结果正文或 Artifact。
type AbstractionInput struct {
	Objective          string                  `json:"objective"`
	CompletionCriteria []string                `json:"completion_criteria"`
	Nodes              []AbstractionSourceNode `json:"nodes"`
}

type AbstractedNode struct {
	LogicalKey         string                             `json:"logical_key"`
	Role               protocol.WorkGraphWorkflowNodeRole `json:"role"`
	Subject            string                             `json:"subject"`
	Objective          string                             `json:"objective"`
	Deliverable        string                             `json:"deliverable"`
	AcceptanceCriteria []string                           `json:"acceptance_criteria"`
}

type AbstractionOutput struct {
	SlashName          string           `json:"slash_name"`
	Title              string           `json:"title"`
	Description        string           `json:"description"`
	Objective          string           `json:"objective"`
	CompletionCriteria []string         `json:"completion_criteria"`
	Nodes              []AbstractedNode `json:"nodes"`
}

type Abstractor interface {
	Abstract(context.Context, string, AbstractionInput) (AbstractionOutput, error)
}

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type preferencesReader interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

// LLMAbstractor 使用 owner 的默认后台模型进行结构提炼。
type LLMAbstractor struct {
	providers providerResolver
	prefs     preferencesReader
	client    *llm.Client
}

func NewLLMAbstractor(providers providerResolver, prefs preferencesReader) *LLMAbstractor {
	return &LLMAbstractor{providers: providers, prefs: prefs, client: llm.NewClient(http.DefaultClient)}
}

func (a *LLMAbstractor) Abstract(ctx context.Context, ownerUserID string, input AbstractionInput) (AbstractionOutput, error) {
	if a == nil || a.providers == nil || a.prefs == nil || a.client == nil {
		return AbstractionOutput{}, errors.New("background abstraction model is unavailable")
	}
	prefs, err := a.prefs.Get(ctx, strings.TrimSpace(ownerUserID))
	if err != nil {
		return AbstractionOutput{}, err
	}
	selection := prefs.DefaultBackgroundModelSelection
	config, err := a.providers.ResolveLLMConfig(ctx, strings.TrimSpace(selection.Provider), strings.TrimSpace(selection.Model))
	if err != nil {
		return AbstractionOutput{}, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return AbstractionOutput{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, abstractionTimeout)
	defer cancel()
	raw, err := a.client.GenerateText(requestCtx, llm.GenerateTextRequest{
		Config: config, System: abstractionSystemPrompt,
		Messages:  []llm.Message{{Role: "user", Content: string(payload)}},
		MaxTokens: abstractionMaxTokens, Temperature: 0, DisableReasoning: true,
	})
	if err != nil {
		return AbstractionOutput{}, err
	}
	var output AbstractionOutput
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &output); err != nil {
		return AbstractionOutput{}, fmt.Errorf("invalid abstraction JSON: %w", err)
	}
	return output, nil
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "```") {
		value = strings.TrimPrefix(value, "```json")
		value = strings.TrimPrefix(value, "```")
		value = strings.TrimSuffix(strings.TrimSpace(value), "```")
	}
	return strings.TrimSpace(value)
}

type ValidatedAbstraction struct {
	SlashName          string
	Title              string
	Description        string
	Objective          string
	CompletionCriteria []string
	Nodes              []protocol.WorkGraphWorkflowNode
}

func applyAbstraction(sourceNodes []protocol.WorkGraphWorkflowNode, output AbstractionOutput) (ValidatedAbstraction, error) {
	output.SlashName = normalizeSlashName(output.SlashName)
	output.Title = strings.TrimSpace(output.Title)
	output.Description = strings.TrimSpace(output.Description)
	output.Objective = strings.TrimSpace(output.Objective)
	if output.SlashName == "" || output.Title == "" || output.Description == "" || output.Objective == "" || len(output.Nodes) == 0 || len(output.Nodes) > len(sourceNodes) {
		return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction returned incomplete graph", ErrInvalidInput)
	}
	sourceByKey := make(map[string]protocol.WorkGraphWorkflowNode, len(sourceNodes))
	for _, source := range sourceNodes {
		sourceByKey[source.LogicalKey] = source
	}
	byKey := make(map[string]AbstractedNode, len(output.Nodes))
	hasKey := false
	for _, node := range output.Nodes {
		node.LogicalKey = strings.TrimSpace(node.LogicalKey)
		if node.LogicalKey == "" || strings.TrimSpace(node.Subject) == "" || strings.TrimSpace(node.Objective) == "" || strings.TrimSpace(node.Deliverable) == "" {
			return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction returned incomplete node", ErrInvalidInput)
		}
		if node.Role != protocol.WorkGraphWorkflowNodeKey && node.Role != protocol.WorkGraphWorkflowNodeCollaboration {
			return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction returned invalid node role", ErrInvalidInput)
		}
		if _, exists := sourceByKey[node.LogicalKey]; !exists {
			return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction invented a graph node", ErrInvalidInput)
		}
		if _, duplicate := byKey[node.LogicalKey]; duplicate {
			return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction returned duplicate logical_key", ErrInvalidInput)
		}
		hasKey = hasKey || node.Role == protocol.WorkGraphWorkflowNodeKey
		byKey[node.LogicalKey] = node
	}
	if !hasKey {
		return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction omitted the key path", ErrInvalidInput)
	}
	nodes := make([]protocol.WorkGraphWorkflowNode, 0, len(output.Nodes))
	hasTerminal := false
	for _, source := range sourceNodes {
		abstracted, ok := byKey[source.LogicalKey]
		if !ok {
			continue
		}
		source.Role = abstracted.Role
		source.Subject = strings.TrimSpace(abstracted.Subject)
		source.Objective = strings.TrimSpace(abstracted.Objective)
		source.Deliverable = strings.TrimSpace(abstracted.Deliverable)
		source.AcceptanceCriteria = cleanStrings(abstracted.AcceptanceCriteria)
		source.Position = len(nodes)
		nodes = append(nodes, source)
		hasTerminal = hasTerminal || source.Terminal
	}
	if !hasTerminal {
		return ValidatedAbstraction{}, fmt.Errorf("%w: abstraction omitted the terminal delivery", ErrInvalidInput)
	}
	return ValidatedAbstraction{
		SlashName: output.SlashName, Title: output.Title, Description: output.Description,
		Objective: output.Objective, CompletionCriteria: cleanStrings(output.CompletionCriteria), Nodes: nodes,
	}, nil
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
