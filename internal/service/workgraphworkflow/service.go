// INPUT: owner、历史 ExecutionView、显式节点角色选择与原始 Slash 文本。
// OUTPUT: 命名 Workflow aggregate、动态 Slash descriptor 与不含旧运行事实的 WorkGraph runtime prompt。
// POS: 历史图沉淀和跨 Session 复用的唯一业务入口。
package workgraphworkflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
)

const maxWorkflowCount = 128

var workflowSlashNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

var reservedWorkflowSlashNames = map[string]struct{}{
	"compact":   {},
	"goal":      {},
	"model":     {},
	"skills":    {},
	"visualize": {},
	"workgraph": {},
}

var (
	// ErrNotFound 表示 owner scope 中不存在目标 Workflow 或源 Execution。
	ErrNotFound = errors.New("workgraph workflow not found")
	// ErrInvalidInput 表示提炼请求不能形成可复用责任图。
	ErrInvalidInput = errors.New("invalid workgraph workflow input")
	// ErrNameConflict 表示命名 Slash 已被固定命令或另一个 Workflow 使用。
	ErrNameConflict = errors.New("workgraph workflow slash name already exists")
)

// Repository 是 Workflow service 所需的最小持久化能力。
type Repository interface {
	Create(context.Context, protocol.WorkGraphWorkflow) (*protocol.WorkGraphWorkflow, error)
	List(context.Context, string) ([]protocol.WorkGraphWorkflow, error)
	GetByID(context.Context, string, string) (*protocol.WorkGraphWorkflow, error)
	GetBySlashName(context.Context, string, string) (*protocol.WorkGraphWorkflow, error)
	Delete(context.Context, string, string) (bool, error)
}

// ExecutionViewer 提供源历史图的 owner/session 安全读取。
type ExecutionViewer interface {
	GetView(context.Context, string, string, string) (*protocol.ExecutionView, error)
}

// Service 编排历史图提炼、目录投影和 prompt 展开。
type Service struct {
	repository Repository
	executions ExecutionViewer
	now        func() time.Time
	newID      func() string
}

// NewService 创建 WorkGraph Workflow service。
func NewService(repository Repository, executions ExecutionViewer) *Service {
	return &Service{
		repository: repository,
		executions: executions,
		now:        time.Now,
		newID:      newWorkflowID,
	}
}

// CreateFromExecution 从源 Execution 的所选 Work Item 创建命名 Workflow。
func (s *Service) CreateFromExecution(
	ctx context.Context,
	ownerUserID string,
	request protocol.CreateWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflow, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.SourceExecutionID = strings.TrimSpace(request.SourceExecutionID)
	request.SlashName = normalizeSlashName(request.SlashName)
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	if ownerUserID == "" || request.SourceSessionKey == "" ||
		request.SourceExecutionID == "" || len(request.Nodes) == 0 {
		return nil, fmt.Errorf("%w: source session, execution and nodes are required", ErrInvalidInput)
	}
	if !workflowSlashNamePattern.MatchString(request.SlashName) {
		return nil, fmt.Errorf("%w: slash_name must use lowercase letters, numbers and hyphens", ErrInvalidInput)
	}
	if _, reserved := reservedWorkflowSlashNames[request.SlashName]; reserved {
		return nil, fmt.Errorf("%w: /%s is reserved", ErrNameConflict, request.SlashName)
	}
	if s == nil || s.repository == nil || s.executions == nil {
		return nil, errors.New("workgraph workflow service is unavailable")
	}
	workflowID := ""
	if commandID := strings.TrimSpace(request.CommandID); commandID != "" {
		workflowID = workflowIDForCommand(ownerUserID, commandID)
		existing, getErr := s.repository.GetByID(ctx, ownerUserID, workflowID)
		if getErr != nil {
			return nil, getErr
		}
		if existing != nil {
			return existing, nil
		}
	}
	existing, err := s.repository.GetBySlashName(ctx, ownerUserID, request.SlashName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, request.SlashName)
	}
	workflows, err := s.repository.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if len(workflows) >= maxWorkflowCount {
		return nil, fmt.Errorf("%w: at most %d workflows are allowed", ErrInvalidInput, maxWorkflowCount)
	}
	source, err := s.executions.GetView(
		ctx,
		ownerUserID,
		request.SourceSessionKey,
		request.SourceExecutionID,
	)
	if err != nil {
		return nil, err
	}
	if source == nil || source.Plan == nil || len(source.WorkItems) == 0 {
		return nil, ErrNotFound
	}
	nodes, dependencies, err := buildWorkflowGraph(source, request.Nodes)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	title := request.Title
	if title == "" {
		title = humanizeSlashName(request.SlashName)
	}
	description := request.Description
	if description == "" {
		description = "Reuse selected responsibility and collaboration nodes from " + source.Objective
	}
	workflow := protocol.WorkGraphWorkflow{
		ID:                 workflowID,
		OwnerUserID:        ownerUserID,
		SlashName:          request.SlashName,
		Title:              title,
		Description:        description,
		SourceExecutionID:  source.ID,
		SourceSessionKey:   source.SessionKey,
		Objective:          source.Objective,
		CompletionCriteria: slices.Clone(source.CompletionCriteria),
		Nodes:              nodes,
		Dependencies:       dependencies,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if workflow.ID == "" {
		workflow.ID = s.newID()
	}
	created, err := s.repository.Create(ctx, workflow)
	if err != nil {
		if duplicateWorkflowError(err) {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, request.SlashName)
		}
		return nil, err
	}
	return created, nil
}

func workflowIDForCommand(ownerUserID string, commandID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(commandID)))
	return "wgw-" + hex.EncodeToString(digest[:16])
}

// List 返回 owner 的 Workflow catalog。
func (s *Service) List(
	ctx context.Context,
	ownerUserID string,
) ([]protocol.WorkGraphWorkflow, error) {
	if s == nil || s.repository == nil {
		return nil, errors.New("workgraph workflow service is unavailable")
	}
	return s.repository.List(ctx, strings.TrimSpace(ownerUserID))
}

// Delete 删除 owner scope 内的 Workflow。
func (s *Service) Delete(
	ctx context.Context,
	ownerUserID string,
	workflowID string,
) (bool, error) {
	if s == nil || s.repository == nil {
		return false, errors.New("workgraph workflow service is unavailable")
	}
	return s.repository.Delete(
		ctx,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(workflowID),
	)
}

// CommandDescriptors 把 owner 的 Workflow 投影为动态 runtime Slash commands。
func (s *Service) CommandDescriptors(
	ctx context.Context,
	ownerUserID string,
) ([]protocol.CommandDescriptor, error) {
	workflows, err := s.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	commands := make([]protocol.CommandDescriptor, 0, len(workflows))
	for _, workflow := range workflows {
		commands = append(commands, protocol.CommandDescriptor{
			Name:         workflow.SlashName,
			Description:  workflow.Description,
			ArgumentHint: "<request>",
			Execution:    protocol.CommandExecutionRuntime,
			Enabled:      true,
		})
	}
	return commands, nil
}

// ExpandRuntimePrompt 展开固定 /workgraph 或 owner 的命名 Workflow。
// 未命中的 Slash 原样返回，继续由当前 runtime 自己解释。
func (s *Service) ExpandRuntimePrompt(
	ctx context.Context,
	ownerUserID string,
	content string,
) (string, error) {
	expanded := slashcommandsvc.ExpandProductPrompt(content)
	if expanded != content {
		return expanded, nil
	}
	name, arguments, ok := parseSlashInvocation(content)
	if !ok || s == nil || s.repository == nil {
		return content, nil
	}
	workflow, err := s.repository.GetBySlashName(
		ctx,
		strings.TrimSpace(ownerUserID),
		name,
	)
	if err != nil {
		return "", err
	}
	if workflow == nil {
		return content, nil
	}
	return renderWorkflowPrompt(*workflow, arguments), nil
}

func buildWorkflowGraph(
	source *protocol.ExecutionView,
	selections []protocol.WorkGraphWorkflowNodeSelection,
) ([]protocol.WorkGraphWorkflowNode, []protocol.WorkGraphWorkflowDependency, error) {
	selectionByID := make(map[string]protocol.WorkGraphWorkflowNodeRole, len(selections))
	for _, selection := range selections {
		workItemID := strings.TrimSpace(selection.WorkItemID)
		if workItemID == "" {
			return nil, nil, fmt.Errorf("%w: selected work_item_id is required", ErrInvalidInput)
		}
		if selection.Role != protocol.WorkGraphWorkflowNodeKey &&
			selection.Role != protocol.WorkGraphWorkflowNodeCollaboration {
			return nil, nil, fmt.Errorf("%w: node role must be key or collaboration", ErrInvalidInput)
		}
		if _, duplicate := selectionByID[workItemID]; duplicate {
			return nil, nil, fmt.Errorf("%w: a work item was selected more than once", ErrInvalidInput)
		}
		selectionByID[workItemID] = selection.Role
	}
	workItemByID := make(map[string]protocol.ExecutionWorkItemView, len(source.WorkItems))
	logicalKeyByID := make(map[string]string, len(source.WorkItems))
	for _, item := range source.WorkItems {
		workItemByID[item.ID] = item
		logicalKeyByID[item.ID] = item.LogicalKey
	}
	nodes := make([]protocol.WorkGraphWorkflowNode, 0, len(selectionByID))
	for workItemID, role := range selectionByID {
		item, exists := workItemByID[workItemID]
		if !exists {
			return nil, nil, fmt.Errorf("%w: selected work item is outside the source graph", ErrInvalidInput)
		}
		parentLogicalKey := ""
		if _, parentSelected := selectionByID[item.ParentWorkItemID]; parentSelected {
			parentLogicalKey = logicalKeyByID[item.ParentWorkItemID]
		}
		nodes = append(nodes, protocol.WorkGraphWorkflowNode{
			LogicalKey:         item.LogicalKey,
			SourceWorkItemID:   item.ID,
			Role:               role,
			Kind:               item.Kind,
			Subject:            item.Subject,
			Objective:          item.Objective,
			Deliverable:        item.Deliverable,
			AcceptanceCriteria: slices.Clone(item.AcceptanceCriteria),
			Required:           item.Required,
			Terminal:           item.Terminal,
			ParentLogicalKey:   parentLogicalKey,
			Position:           item.Position,
		})
	}
	sort.Slice(nodes, func(left, right int) bool {
		if nodes[left].Position != nodes[right].Position {
			return nodes[left].Position < nodes[right].Position
		}
		return nodes[left].LogicalKey < nodes[right].LogicalKey
	})
	terminal := false
	for index := range nodes {
		terminal = terminal || nodes[index].Terminal
	}
	if !terminal && len(nodes) > 0 {
		nodes[len(nodes)-1].Terminal = true
	}
	dependencies := make([]protocol.WorkGraphWorkflowDependency, 0)
	for _, item := range source.WorkItems {
		if _, selected := selectionByID[item.ID]; !selected {
			continue
		}
		for _, dependencyID := range item.DependencyIDs {
			if _, selected := selectionByID[dependencyID]; !selected {
				continue
			}
			dependencies = append(dependencies, protocol.WorkGraphWorkflowDependency{
				LogicalKey:          item.LogicalKey,
				DependsOnLogicalKey: logicalKeyByID[dependencyID],
				Kind:                protocol.WorkDependencyHard,
			})
		}
	}
	return nodes, dependencies, nil
}

func renderWorkflowPrompt(
	workflow protocol.WorkGraphWorkflow,
	arguments string,
) string {
	request := strings.TrimSpace(arguments)
	if request == "" {
		request = "Use the actionable request already established in this conversation."
	}
	var output strings.Builder
	fmt.Fprintf(
		&output,
		"Use Nexus WorkGraph collaboration to run the reusable workflow /%s. Load the execution-orchestrator skill and materialize a fresh managed WorkGraph in the current Session. Reuse only the semantic nodes and dependency topology below; mint new Execution, Plan and Work Item identities. Never copy source assignments, attempts, tool calls, submissions, reviews, acceptances, statuses, results or artifacts. Adapt each node to the current request without weakening its deliverable or acceptance contract.\n\nCurrent request:\n%s\n\nSource objective pattern:\n%s\n\nWorkflow nodes:\n",
		workflow.SlashName,
		request,
		workflow.Objective,
	)
	for _, node := range workflow.Nodes {
		fmt.Fprintf(
			&output,
			"- %s [%s, %s]: %s | objective: %s | deliverable: %s",
			node.LogicalKey,
			node.Role,
			node.Kind,
			node.Subject,
			node.Objective,
			node.Deliverable,
		)
		if len(node.AcceptanceCriteria) > 0 {
			fmt.Fprintf(&output, " | acceptance: %s", strings.Join(node.AcceptanceCriteria, "; "))
		}
		if node.ParentLogicalKey != "" {
			fmt.Fprintf(&output, " | parent: %s", node.ParentLogicalKey)
		}
		if node.Terminal {
			output.WriteString(" | terminal")
		}
		output.WriteByte('\n')
	}
	if len(workflow.Dependencies) > 0 {
		output.WriteString("\nDependencies:\n")
		for _, dependency := range workflow.Dependencies {
			fmt.Fprintf(
				&output,
				"- %s depends on %s (%s)\n",
				dependency.LogicalKey,
				dependency.DependsOnLogicalKey,
				dependency.Kind,
			)
		}
	}
	output.WriteString("\nAfter materializing the graph, assign and execute work only through current DM/Room authority and the fresh graph state.")
	return output.String()
}

func normalizeSlashName(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimLeft(value, "/")))
}

func humanizeSlashName(value string) string {
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	for index := range parts {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, " ")
}

func parseSlashInvocation(content string) (string, string, bool) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/") {
		return "", "", false
	}
	withoutSlash := strings.TrimPrefix(content, "/")
	fields := strings.Fields(withoutSlash)
	if len(fields) == 0 {
		return "", "", false
	}
	name := normalizeSlashName(fields[0])
	if !workflowSlashNamePattern.MatchString(name) {
		return "", "", false
	}
	arguments := strings.TrimSpace(strings.TrimPrefix(withoutSlash, fields[0]))
	return name, arguments, true
}

func duplicateWorkflowError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}

func newWorkflowID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return "workflow_" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("workflow_%d", time.Now().UnixNano())
}
