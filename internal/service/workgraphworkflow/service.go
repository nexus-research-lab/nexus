// INPUT: owner、完成态 ExecutionView、默认后台模型草图与受管 CLI 确认。
// OUTPUT: 非持久化结构草图、确认后的命名 WorkGraph、动态 Slash descriptor 与复用 prompt。
// POS: 完成图提炼、用户预览确认和跨 Session 复用的唯一业务入口。
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
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
)

const (
	maxWorkflowCount   = 128
	workflowPreviewTTL = 2 * time.Hour
)

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
	repository     Repository
	executions     ExecutionViewer
	abstractor     Abstractor
	saveDispatcher SaveRoundDispatcher
	onChanged      func(context.Context, string)
	now            func() time.Time
	newID          func() string
	previewMu      sync.Mutex
	previews       map[string]workflowPreviewRecord
}

type workflowPreviewRecord struct {
	ownerUserID   string
	preview       protocol.WorkGraphWorkflowPreview
	saveScheduled bool
}

// NewService 创建 WorkGraph Workflow service。
func NewService(repository Repository, executions ExecutionViewer) *Service {
	return &Service{
		repository: repository,
		executions: executions,
		now:        time.Now,
		newID:      newWorkflowID,
		previews:   make(map[string]workflowPreviewRecord),
	}
}

// SetAbstractor 注入默认后台模型抽象审查器；未配置时草图生成失败关闭。
func (s *Service) SetAbstractor(abstractor Abstractor) {
	if s != nil {
		s.abstractor = abstractor
	}
}

// SetChangeNotifier 注入目录变更通知，用于刷新能力计数与 Session Slash 目录。
func (s *Service) SetChangeNotifier(notifier func(context.Context, string)) {
	if s != nil {
		s.onChanged = notifier
	}
}

// PreviewFromExecution 从完整完成图自动抽取非持久化草图，供用户只读确认。
func (s *Service) PreviewFromExecution(
	ctx context.Context,
	ownerUserID string,
	request protocol.PreviewWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflowPreview, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.SourceExecutionID = strings.TrimSpace(request.SourceExecutionID)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.SourceExecutionID == "" {
		return nil, fmt.Errorf("%w: source session and execution are required", ErrInvalidInput)
	}
	if s == nil || s.repository == nil || s.executions == nil || s.abstractor == nil {
		return nil, errors.New("workgraph sketch service is unavailable")
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
	if source.Status != protocol.ExecutionStatusCompleted {
		return nil, fmt.Errorf("%w: only completed WorkGraphs can be saved", ErrInvalidInput)
	}
	sourceNodes, abstractionNodes := buildSourceWorkflowGraph(source)
	abstracted, err := s.abstractor.Abstract(ctx, ownerUserID, AbstractionInput{
		Objective: source.Objective, CompletionCriteria: slices.Clone(source.CompletionCriteria), Nodes: abstractionNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("workgraph abstraction failed: %w", err)
	}
	validated, err := applyAbstraction(sourceNodes, abstracted)
	if err != nil {
		return nil, err
	}
	if !workflowSlashNamePattern.MatchString(validated.SlashName) {
		return nil, fmt.Errorf("%w: generated slash_name is invalid", ErrInvalidInput)
	}
	validated.SlashName, err = s.availableSlashName(ctx, ownerUserID, validated.SlashName)
	if err != nil {
		return nil, err
	}
	nodes, dependencies := projectExtractedWorkflowGraph(source, validated.Nodes)
	now := s.now().UTC()
	preview := protocol.WorkGraphWorkflowPreview{
		PreviewID: newPreviewID(), SlashName: validated.SlashName,
		Title: validated.Title, Description: validated.Description,
		SourceExecutionID: source.ID, SourceSessionKey: source.SessionKey,
		Objective: validated.Objective, CompletionCriteria: validated.CompletionCriteria,
		Nodes: nodes, Dependencies: dependencies, ExpiresAt: now.Add(workflowPreviewTTL),
	}
	s.storePreview(ownerUserID, preview, now)
	result := cloneWorkflowPreview(preview)
	return &result, nil
}

// SavePreview 只持久化同 owner、同 Session 中用户已经预览确认的 exact 草图。
func (s *Service) SavePreview(
	ctx context.Context,
	ownerUserID string,
	request protocol.SaveWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflow, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.PreviewID == "" {
		return nil, fmt.Errorf("%w: source session and preview_id are required", ErrInvalidInput)
	}
	if s == nil || s.repository == nil {
		return nil, errors.New("workgraph sketch service is unavailable")
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
	preview, err := s.getPreview(ownerUserID, request.SourceSessionKey, request.PreviewID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetBySlashName(ctx, ownerUserID, preview.SlashName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	workflows, err := s.repository.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if len(workflows) >= maxWorkflowCount {
		return nil, fmt.Errorf("%w: at most %d workflows are allowed", ErrInvalidInput, maxWorkflowCount)
	}
	now := s.now().UTC()
	workflow := protocol.WorkGraphWorkflow{
		ID: workflowID, OwnerUserID: ownerUserID,
		SlashName: preview.SlashName, Title: preview.Title, Description: preview.Description,
		SourceExecutionID: preview.SourceExecutionID, SourceSessionKey: preview.SourceSessionKey,
		Objective: preview.Objective, CompletionCriteria: slices.Clone(preview.CompletionCriteria),
		Nodes: cloneWorkflowNodes(preview.Nodes), Dependencies: slices.Clone(preview.Dependencies),
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if workflow.ID == "" {
		workflow.ID = s.newID()
	}
	created, err := s.repository.Create(ctx, workflow)
	if err != nil {
		if duplicateWorkflowError(err) {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
		}
		return nil, err
	}
	if s.onChanged != nil {
		s.onChanged(ctx, ownerUserID)
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

func buildSourceWorkflowGraph(source *protocol.ExecutionView) ([]protocol.WorkGraphWorkflowNode, []AbstractionSourceNode) {
	items := slices.Clone(source.WorkItems)
	sort.Slice(items, func(left, right int) bool {
		if items[left].Position != items[right].Position {
			return items[left].Position < items[right].Position
		}
		return items[left].LogicalKey < items[right].LogicalKey
	})
	logicalKeyByID := make(map[string]string, len(items))
	for _, item := range items {
		logicalKeyByID[item.ID] = item.LogicalKey
	}
	sourceNodes := make([]protocol.WorkGraphWorkflowNode, 0, len(items))
	abstractionNodes := make([]AbstractionSourceNode, 0, len(items))
	for _, item := range items {
		dependencies := make([]string, 0, len(item.DependencyIDs))
		for _, dependencyID := range item.DependencyIDs {
			if logicalKey := logicalKeyByID[dependencyID]; logicalKey != "" {
				dependencies = append(dependencies, logicalKey)
			}
		}
		parentLogicalKey := logicalKeyByID[item.ParentWorkItemID]
		sourceNodes = append(sourceNodes, protocol.WorkGraphWorkflowNode{
			LogicalKey: item.LogicalKey, SourceWorkItemID: item.ID,
			Role: protocol.WorkGraphWorkflowNodeKey, Kind: item.Kind,
			Subject: item.Subject, Objective: item.Objective, Deliverable: item.Deliverable,
			AcceptanceCriteria: slices.Clone(item.AcceptanceCriteria), Required: item.Required,
			Terminal: item.Terminal, ParentLogicalKey: parentLogicalKey, Position: item.Position,
		})
		abstractionNodes = append(abstractionNodes, AbstractionSourceNode{
			LogicalKey: item.LogicalKey, Kind: item.Kind,
			Subject: item.Subject, Objective: item.Objective, Deliverable: item.Deliverable,
			AcceptanceCriteria: slices.Clone(item.AcceptanceCriteria), Required: item.Required,
			Terminal: item.Terminal, ParentLogicalKey: parentLogicalKey,
			DependencyLogicalKeys: dependencies, Status: item.Status,
			AssignmentStrategy: item.AssignmentStrategy,
			Delegated:          item.AssignmentStrategy == protocol.AssignmentStrategyRoomMember,
			IndependentReview:  item.ReviewAgentID != "" || item.ReviewDispatchID != "" || item.ReviewStatus != "",
			AttemptCount:       len(item.Attempts),
		})
	}
	return sourceNodes, abstractionNodes
}

func projectExtractedWorkflowGraph(source *protocol.ExecutionView, nodes []protocol.WorkGraphWorkflowNode) ([]protocol.WorkGraphWorkflowNode, []protocol.WorkGraphWorkflowDependency) {
	itemByID := make(map[string]protocol.ExecutionWorkItemView, len(source.WorkItems))
	logicalKeyByID := make(map[string]string, len(source.WorkItems))
	itemIDByLogicalKey := make(map[string]string, len(source.WorkItems))
	for _, item := range source.WorkItems {
		itemByID[item.ID] = item
		logicalKeyByID[item.ID] = item.LogicalKey
		itemIDByLogicalKey[item.LogicalKey] = item.ID
	}
	selected := make(map[string]struct{}, len(nodes))
	position := make(map[string]int, len(nodes))
	for index, node := range nodes {
		selected[node.LogicalKey] = struct{}{}
		position[node.LogicalKey] = index
	}
	resultNodes := cloneWorkflowNodes(nodes)
	for index := range resultNodes {
		resultNodes[index].Position = index
		resultNodes[index].ParentLogicalKey = nearestSelectedParent(itemByID, logicalKeyByID, selected, itemIDByLogicalKey[resultNodes[index].LogicalKey])
	}
	type dependencyKey struct{ target, source string }
	edges := make(map[dependencyKey]struct{})
	for _, node := range resultNodes {
		item := itemByID[itemIDByLogicalKey[node.LogicalKey]]
		for _, dependencyID := range item.DependencyIDs {
			for _, dependencyLogicalKey := range nearestSelectedDependencies(itemByID, logicalKeyByID, selected, dependencyID, map[string]struct{}{}) {
				if dependencyLogicalKey != node.LogicalKey {
					edges[dependencyKey{target: node.LogicalKey, source: dependencyLogicalKey}] = struct{}{}
				}
			}
		}
	}
	dependencies := make([]protocol.WorkGraphWorkflowDependency, 0, len(edges))
	for edge := range edges {
		dependencies = append(dependencies, protocol.WorkGraphWorkflowDependency{
			LogicalKey: edge.target, DependsOnLogicalKey: edge.source, Kind: protocol.WorkDependencyHard,
		})
	}
	sort.Slice(dependencies, func(left, right int) bool {
		if position[dependencies[left].LogicalKey] != position[dependencies[right].LogicalKey] {
			return position[dependencies[left].LogicalKey] < position[dependencies[right].LogicalKey]
		}
		return position[dependencies[left].DependsOnLogicalKey] < position[dependencies[right].DependsOnLogicalKey]
	})
	return resultNodes, dependencies
}

func nearestSelectedParent(itemByID map[string]protocol.ExecutionWorkItemView, logicalKeyByID map[string]string, selected map[string]struct{}, itemID string) string {
	visited := make(map[string]struct{})
	for parentID := itemByID[itemID].ParentWorkItemID; parentID != ""; parentID = itemByID[parentID].ParentWorkItemID {
		if _, loop := visited[parentID]; loop {
			return ""
		}
		visited[parentID] = struct{}{}
		logicalKey := logicalKeyByID[parentID]
		if _, ok := selected[logicalKey]; ok {
			return logicalKey
		}
	}
	return ""
}

func nearestSelectedDependencies(itemByID map[string]protocol.ExecutionWorkItemView, logicalKeyByID map[string]string, selected map[string]struct{}, itemID string, visited map[string]struct{}) []string {
	if itemID == "" {
		return nil
	}
	if _, loop := visited[itemID]; loop {
		return nil
	}
	visited[itemID] = struct{}{}
	logicalKey := logicalKeyByID[itemID]
	if _, ok := selected[logicalKey]; ok {
		return []string{logicalKey}
	}
	item, exists := itemByID[itemID]
	if !exists {
		return nil
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, dependencyID := range item.DependencyIDs {
		for _, candidate := range nearestSelectedDependencies(itemByID, logicalKeyByID, selected, dependencyID, visited) {
			if _, duplicate := seen[candidate]; !duplicate {
				seen[candidate] = struct{}{}
				result = append(result, candidate)
			}
		}
	}
	return result
}

func (s *Service) availableSlashName(ctx context.Context, ownerUserID string, base string) (string, error) {
	base = normalizeSlashName(base)
	if _, reserved := reservedWorkflowSlashNames[base]; reserved {
		base = strings.TrimSuffix(base, "-") + "-graph"
	}
	for suffix := 1; suffix <= 99; suffix++ {
		candidate := base
		if suffix > 1 {
			tail := fmt.Sprintf("-%d", suffix)
			limit := 64 - len(tail)
			trimmed := base
			if len(trimmed) > limit {
				trimmed = trimmed[:limit]
			}
			candidate = strings.TrimRight(trimmed, "-") + tail
		}
		if !workflowSlashNamePattern.MatchString(candidate) {
			return "", fmt.Errorf("%w: generated slash_name is invalid", ErrInvalidInput)
		}
		existing, err := s.repository.GetBySlashName(ctx, ownerUserID, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: /%s", ErrNameConflict, base)
}

func (s *Service) storePreview(ownerUserID string, preview protocol.WorkGraphWorkflowPreview, now time.Time) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(now)
	s.previews[previewCacheKey(ownerUserID, preview.PreviewID)] = workflowPreviewRecord{ownerUserID: ownerUserID, preview: cloneWorkflowPreview(preview)}
}

func (s *Service) getPreview(ownerUserID string, sessionKey string, previewID string) (protocol.WorkGraphWorkflowPreview, error) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	now := s.now().UTC()
	s.cleanupExpiredPreviews(now)
	record, ok := s.previews[previewCacheKey(ownerUserID, previewID)]
	if !ok || record.ownerUserID != ownerUserID || record.preview.SourceSessionKey != sessionKey {
		return protocol.WorkGraphWorkflowPreview{}, ErrNotFound
	}
	return cloneWorkflowPreview(record.preview), nil
}

func (s *Service) claimPreviewForSave(
	ownerUserID string,
	sessionKey string,
	previewID string,
) (protocol.WorkGraphWorkflowPreview, bool, error) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := previewCacheKey(ownerUserID, previewID)
	record, ok := s.previews[key]
	if !ok || record.ownerUserID != ownerUserID || record.preview.SourceSessionKey != sessionKey {
		return protocol.WorkGraphWorkflowPreview{}, false, ErrNotFound
	}
	alreadyScheduled := record.saveScheduled
	record.saveScheduled = true
	s.previews[key] = record
	return cloneWorkflowPreview(record.preview), alreadyScheduled, nil
}

func (s *Service) releasePreviewSaveClaim(ownerUserID string, previewID string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	key := previewCacheKey(ownerUserID, previewID)
	record, ok := s.previews[key]
	if !ok {
		return
	}
	record.saveScheduled = false
	s.previews[key] = record
}

func (s *Service) cleanupExpiredPreviews(now time.Time) {
	for key, record := range s.previews {
		if !record.preview.ExpiresAt.After(now) {
			delete(s.previews, key)
		}
	}
}

func previewCacheKey(ownerUserID string, previewID string) string {
	return strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(previewID)
}

func cloneWorkflowPreview(preview protocol.WorkGraphWorkflowPreview) protocol.WorkGraphWorkflowPreview {
	preview.CompletionCriteria = slices.Clone(preview.CompletionCriteria)
	preview.Nodes = cloneWorkflowNodes(preview.Nodes)
	preview.Dependencies = slices.Clone(preview.Dependencies)
	return preview
}

func cloneWorkflowNodes(nodes []protocol.WorkGraphWorkflowNode) []protocol.WorkGraphWorkflowNode {
	result := slices.Clone(nodes)
	for index := range result {
		result[index].AcceptanceCriteria = slices.Clone(result[index].AcceptanceCriteria)
	}
	return result
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
		"Use Nexus WorkGraph collaboration to run the reusable WorkGraph command /%s. Load the execution-orchestrator skill and materialize a fresh managed WorkGraph in the current Session. Reuse only the abstract semantic nodes and dependency topology below; mint new Execution, Plan and Work Item identities. Never copy source assignments, attempts, tool calls, submissions, reviews, acceptances, statuses, results or artifacts. Adapt each node to the current request without weakening its deliverable or acceptance contract.\n\nCurrent request:\n%s\n\nReusable objective pattern:\n%s\n\nWorkGraph nodes:\n",
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

func newPreviewID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return "workgraph_preview_" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("workgraph_preview_%d", time.Now().UnixNano())
}
