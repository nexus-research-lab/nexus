// INPUT: owner、完成态 ExecutionView、默认对话模型草图、durable Draft 与受管 CLI 确认。
// OUTPUT: 可恢复版本化草图、确认后的命名 WorkGraph、动态 Slash descriptor 与复用 prompt。
// POS: 完成图提炼、UI/对话统一编辑确认和跨 Session 复用的唯一业务入口。
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
	workflowPreviewTTL = 30 * 24 * time.Hour
)

var workflowSlashNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

var reservedWorkflowSlashNames = map[string]struct{}{
	"build-ship":     {},
	"compact":        {},
	"decision-brief": {},
	"deep-research":  {},
	"goal":           {},
	"model":          {},
	"review-improve": {},
	"skills":         {},
	"visualize":      {},
	"workgraph":      {},
}

var (
	// ErrNotFound 表示 owner scope 中不存在目标 Workflow 或源 Execution。
	ErrNotFound = errors.New("workgraph workflow not found")
	// ErrInvalidInput 表示提炼请求不能形成可复用责任图。
	ErrInvalidInput = errors.New("invalid workgraph workflow input")
	// ErrRevisionConflict 表示 exact editor revision 已变化，本次请求尚未应用。
	// 它继续包装 ErrInvalidInput，保留既有调用方的 errors.Is 兼容性。
	ErrRevisionConflict = fmt.Errorf("%w: workgraph workflow revision conflict", ErrInvalidInput)
	// ErrNameConflict 表示命名 Slash 已被固定命令或另一个 Workflow 使用。
	ErrNameConflict = errors.New("workgraph workflow slash name already exists")
)

// Repository 是 Workflow service 所需的最小持久化能力。
type Repository interface {
	Create(context.Context, protocol.WorkGraphWorkflow) (*protocol.WorkGraphWorkflow, error)
	Update(context.Context, protocol.WorkGraphWorkflow) (*protocol.WorkGraphWorkflow, error)
	List(context.Context, string) ([]protocol.WorkGraphWorkflow, error)
	GetByID(context.Context, string, string) (*protocol.WorkGraphWorkflow, error)
	GetBySlashName(context.Context, string, string) (*protocol.WorkGraphWorkflow, error)
	Delete(context.Context, string, string) (bool, error)
}

// DraftRepository 是生产 repository 提供的可恢复草图与不可变版本能力。
// 测试替身可不实现，Service 会保留进程内语义。
type DraftRepository interface {
	CreateDraft(context.Context, protocol.WorkGraphWorkflowDraft) (*protocol.WorkGraphWorkflowDraft, error)
	GetDraftByID(context.Context, string, string) (*protocol.WorkGraphWorkflowDraft, error)
	GetDraftBySource(context.Context, string, string, string) (*protocol.WorkGraphWorkflowDraft, error)
	GetDraftByEditorSession(context.Context, string, string) (*protocol.WorkGraphWorkflowDraft, error)
	GetDraftBySavedWorkflowID(context.Context, string, string) (*protocol.WorkGraphWorkflowDraft, error)
	ListDrafts(context.Context, string, string) ([]protocol.WorkGraphWorkflowDraft, error)
	RenewDraftLease(context.Context, string, string, time.Time, time.Time) error
	AppendDraftVersion(context.Context, string, string, int64, protocol.WorkGraphWorkflowPreview, time.Time, time.Time) (*protocol.WorkGraphWorkflowDraft, error)
	SelectDraftVersion(context.Context, string, string, int64, int64, time.Time) (*protocol.WorkGraphWorkflowDraft, error)
	BindDraftEditor(context.Context, string, string, string, string, string, int64, time.Time) (*protocol.WorkGraphWorkflowDraft, error)
	SetDraftSaveState(context.Context, string, string, bool, string, int64, time.Time) error
}

// MainAgentResolver 为所有草图编辑统一选择 owner 的 Nexus 主智能体。
type MainAgentResolver interface {
	GetDefaultAgent(context.Context) (*protocol.Agent, error)
}

// ExecutionViewer 提供源历史图的 owner/session 安全读取。
type ExecutionViewer interface {
	GetView(context.Context, string, string, string) (*protocol.ExecutionView, error)
}

// Service 编排历史图提炼、目录投影和 prompt 展开。
type Service struct {
	repository      Repository
	executions      ExecutionViewer
	abstractor      Abstractor
	agents          MainAgentResolver
	editorSessions  EditorSessionManager
	saveDispatcher  SaveRoundDispatcher
	onChanged       func(context.Context, string)
	now             func() time.Time
	newID           func() string
	previewMu       sync.Mutex
	previews        map[string]workflowPreviewRecord
	editors         map[string]workflowEditorRecord
	editorBySession map[string]string
	editorByPreview map[string]string
}

type workflowPreviewRecord struct {
	ownerUserID          string
	preview              protocol.WorkGraphWorkflowPreview
	sourceAgentID        string
	sourceConversationID string
	outputLanguage       string
	saveScheduled        bool
	savedWorkflowID      string
	savedRevision        int64
}

// NewService 创建 WorkGraph Workflow service。
func NewService(repository Repository, executions ExecutionViewer) *Service {
	return &Service{
		repository:      repository,
		executions:      executions,
		now:             time.Now,
		newID:           newWorkflowID,
		previews:        make(map[string]workflowPreviewRecord),
		editors:         make(map[string]workflowEditorRecord),
		editorBySession: make(map[string]string),
		editorByPreview: make(map[string]string),
	}
}

// SetAbstractor 注入默认对话模型抽象审查器；未配置时草图生成失败关闭。
func (s *Service) SetAbstractor(abstractor Abstractor) {
	if s != nil {
		s.abstractor = abstractor
	}
}

// SetMainAgentResolver 注入 owner 的 Nexus 主智能体解析器。
func (s *Service) SetMainAgentResolver(resolver MainAgentResolver) {
	if s != nil {
		s.agents = resolver
	}
}

// SetEditorSessionManager 注入隐藏专用 DM Session 的创建与显式丢弃能力。
func (s *Service) SetEditorSessionManager(manager EditorSessionManager) {
	if s != nil {
		s.editorSessions = manager
	}
}

// SetChangeNotifier 注入目录变更通知，用于刷新能力计数与 Session Slash 目录。
func (s *Service) SetChangeNotifier(notifier func(context.Context, string)) {
	if s != nil {
		s.onChanged = notifier
	}
}

// PreviewFromExecution 从完整完成图自动抽取或复用 durable Draft，供用户确认和版本化修订。
func (s *Service) PreviewFromExecution(
	ctx context.Context,
	ownerUserID string,
	request protocol.PreviewWorkGraphWorkflowRequest,
) (*protocol.WorkGraphWorkflowPreview, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.SourceExecutionID = strings.TrimSpace(request.SourceExecutionID)
	request.OutputLanguage = normalizeWorkflowOutputLanguage(request.OutputLanguage)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.SourceExecutionID == "" {
		return nil, fmt.Errorf("%w: source session and execution are required", ErrInvalidInput)
	}
	if request.OutputLanguage == "" {
		return nil, fmt.Errorf("%w: output_language must be zh or en", ErrInvalidInput)
	}
	if s == nil || s.repository == nil || s.executions == nil || s.abstractor == nil {
		return nil, errors.New("workgraph sketch service is unavailable")
	}
	if existing, existingErr := s.findReusableDraft(
		ctx,
		ownerUserID,
		request.SourceSessionKey,
		request.SourceExecutionID,
	); existingErr != nil {
		return nil, existingErr
	} else if existing != nil {
		result := cloneWorkflowPreview(existing.Preview)
		return &result, nil
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
		OutputLanguage: request.OutputLanguage, ExistingSlashNames: unavailableWorkflowSlashNames(workflows),
	})
	if err != nil {
		return nil, fmt.Errorf("workgraph abstraction failed: %w", err)
	}
	validated, err := applyAbstraction(sourceNodes, abstractionNodes, abstracted)
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
	sourceAgentID := strings.TrimSpace(source.CoordinatorAgentID)
	if sourceAgentID == "" {
		sourceAgentID = strings.TrimSpace(protocol.ParseSessionKey(source.SessionKey).AgentID)
	}
	if err = s.storePreview(ctx, ownerUserID, preview, now, request.OutputLanguage, workflowPreviewSource{
		AgentID:        sourceAgentID,
		ConversationID: source.ConversationID,
	}); err != nil {
		if existing, lookupErr := s.findReusableDraft(
			ctx,
			ownerUserID,
			request.SourceSessionKey,
			request.SourceExecutionID,
		); lookupErr == nil && existing != nil {
			result := cloneWorkflowPreview(existing.Preview)
			return &result, nil
		}
		return nil, err
	}
	result := cloneWorkflowPreview(preview)
	return &result, nil
}

func unavailableWorkflowSlashNames(workflows []protocol.WorkGraphWorkflow) []string {
	names := make([]string, 0, len(reservedWorkflowSlashNames)+len(workflows))
	for name := range reservedWorkflowSlashNames {
		names = append(names, name)
	}
	for _, workflow := range workflows {
		if name := normalizeSlashName(workflow.SlashName); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return slices.Compact(names)
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
	draft, err := s.loadDraftByID(ctx, ownerUserID, request.PreviewID)
	if err != nil {
		return nil, err
	}
	if draft != nil && draft.SourceSessionKey != request.SourceSessionKey {
		return nil, ErrNotFound
	}
	workflowID := ""
	if commandID := strings.TrimSpace(request.CommandID); commandID != "" {
		workflowID = workflowIDForCommand(ownerUserID, commandID)
		existing, getErr := s.repository.GetByID(ctx, ownerUserID, workflowID)
		if getErr != nil {
			return nil, getErr
		}
		if existing != nil {
			if drafts, supported := s.repository.(DraftRepository); supported {
				savedRevision := int64(0)
				if draft != nil {
					savedRevision = draft.SelectedRevision
				}
				_ = drafts.SetDraftSaveState(ctx, ownerUserID, request.PreviewID, false, existing.ID, savedRevision, s.now().UTC())
			}
			return existing, nil
		}
	}
	preview, err := s.getPreview(ctx, ownerUserID, request.SourceSessionKey, request.PreviewID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetBySlashName(ctx, ownerUserID, preview.SlashName)
	if err != nil {
		return nil, err
	}
	if _, reserved := reservedWorkflowSlashNames[preview.SlashName]; reserved &&
		!canKeepLegacyBuiltinSlashName(preview.SlashName, existing, savedWorkflowID(draft)) {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	if existing != nil && (draft == nil || existing.ID != draft.SavedWorkflowID) {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
	}
	if draft == nil || draft.SavedWorkflowID == "" {
		workflows, listErr := s.repository.List(ctx, ownerUserID)
		if listErr != nil {
			return nil, listErr
		}
		if len(workflows) >= maxWorkflowCount {
			return nil, fmt.Errorf("%w: at most %d workflows are allowed", ErrInvalidInput, maxWorkflowCount)
		}
	}
	now := s.now().UTC()
	if draft != nil && draft.SavedWorkflowID != "" {
		target, getErr := s.repository.GetByID(ctx, ownerUserID, draft.SavedWorkflowID)
		if getErr != nil {
			return nil, getErr
		}
		if target == nil {
			return nil, ErrNotFound
		}
		if draft.SavedRevision == draft.SelectedRevision {
			if drafts, supported := s.repository.(DraftRepository); supported {
				_ = drafts.SetDraftSaveState(ctx, ownerUserID, preview.PreviewID, false, target.ID, draft.SavedRevision, now)
			}
			s.setPreviewSavedState(ownerUserID, preview.PreviewID, false, target.ID, draft.SavedRevision)
			return target, nil
		}
		updated, updateErr := s.repository.Update(ctx, protocol.WorkGraphWorkflow{
			ID: target.ID, OwnerUserID: ownerUserID,
			SlashName: preview.SlashName, Title: preview.Title, Description: preview.Description,
			SourceExecutionID: preview.SourceExecutionID, SourceSessionKey: preview.SourceSessionKey,
			Objective: preview.Objective, CompletionCriteria: slices.Clone(preview.CompletionCriteria),
			Nodes: cloneWorkflowNodes(preview.Nodes), Dependencies: slices.Clone(preview.Dependencies),
			Version: target.Version + 1, CreatedAt: target.CreatedAt, UpdatedAt: now,
		})
		if updateErr != nil {
			if duplicateWorkflowError(updateErr) {
				return nil, fmt.Errorf("%w: /%s", ErrNameConflict, preview.SlashName)
			}
			return nil, updateErr
		}
		if drafts, supported := s.repository.(DraftRepository); supported {
			if err = drafts.SetDraftSaveState(ctx, ownerUserID, preview.PreviewID, false, updated.ID, draft.SelectedRevision, now); err != nil {
				return nil, err
			}
		}
		s.setPreviewSavedState(ownerUserID, preview.PreviewID, false, updated.ID, draft.SelectedRevision)
		if s.onChanged != nil {
			s.onChanged(ctx, ownerUserID)
		}
		return updated, nil
	}
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
	if drafts, supported := s.repository.(DraftRepository); supported {
		savedRevision := int64(1)
		if draft != nil {
			savedRevision = draft.SelectedRevision
		}
		if err = drafts.SetDraftSaveState(ctx, ownerUserID, preview.PreviewID, false, created.ID, savedRevision, s.now().UTC()); err != nil {
			return nil, err
		}
		s.setPreviewSavedState(ownerUserID, preview.PreviewID, false, created.ID, savedRevision)
	} else {
		s.setPreviewSavedState(ownerUserID, preview.PreviewID, true, created.ID, 1)
	}
	return created, nil
}

func (s *Service) setPreviewSavedState(ownerUserID, previewID string, scheduled bool, workflowID string, savedRevision int64) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if record, ok := s.previews[previewCacheKey(ownerUserID, previewID)]; ok {
		record.savedWorkflowID = strings.TrimSpace(workflowID)
		record.savedRevision = savedRevision
		record.saveScheduled = scheduled
		s.previews[previewCacheKey(ownerUserID, previewID)] = record
	}
}

func workflowIDForCommand(ownerUserID string, commandID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(commandID)))
	return "wgw-" + hex.EncodeToString(digest[:16])
}

func savedWorkflowID(draft *protocol.WorkGraphWorkflowDraft) string {
	if draft == nil {
		return ""
	}
	return strings.TrimSpace(draft.SavedWorkflowID)
}

func canKeepLegacyBuiltinSlashName(
	slashName string,
	existing *protocol.WorkGraphWorkflow,
	savedID string,
) bool {
	return isBuiltinWorkflowSlashName(slashName) && existing != nil &&
		strings.TrimSpace(existing.ID) == strings.TrimSpace(savedID)
}

// List 返回系统内置模板与 owner 保存图合并后的英文 Workflow catalog。
func (s *Service) List(
	ctx context.Context,
	ownerUserID string,
) ([]protocol.WorkGraphWorkflow, error) {
	return s.ListLocalized(ctx, ownerUserID, "en")
}

// ListLocalized 返回按展示语言本地化的系统模板与 owner 保存图；历史同名保存图优先。
func (s *Service) ListLocalized(
	ctx context.Context,
	ownerUserID string,
	locale string,
) ([]protocol.WorkGraphWorkflow, error) {
	if s == nil || s.repository == nil {
		return nil, errors.New("workgraph workflow service is unavailable")
	}
	workflows, err := s.repository.List(ctx, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	usedNames := make(map[string]struct{}, len(workflows))
	for _, workflow := range workflows {
		usedNames[normalizeSlashName(workflow.SlashName)] = struct{}{}
	}
	result := make([]protocol.WorkGraphWorkflow, 0, len(workflows)+len(builtinWorkflowDefinitions))
	for _, workflow := range builtinWorkflows(locale) {
		if _, shadowed := usedNames[workflow.SlashName]; !shadowed {
			result = append(result, workflow)
		}
	}
	return append(result, workflows...), nil
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
	if isBuiltinWorkflowID(workflowID) {
		return false, nil
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
		workflow = builtinWorkflowBySlashName(name, "en")
		if workflow == nil {
			return content, nil
		}
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
	structuralReferences := make(map[string]struct{}, len(abstractionNodes))
	for _, node := range abstractionNodes {
		if node.ParentLogicalKey != "" {
			structuralReferences[node.ParentLogicalKey] = struct{}{}
		}
		for _, dependencyLogicalKey := range node.DependencyLogicalKeys {
			structuralReferences[dependencyLogicalKey] = struct{}{}
		}
	}
	for index := range abstractionNodes {
		node := &abstractionNodes[index]
		_, referenced := structuralReferences[node.LogicalKey]
		node.MustPreserve = node.Required ||
			node.Terminal ||
			node.ParentLogicalKey != "" ||
			len(node.DependencyLogicalKeys) > 0 ||
			referenced ||
			node.Delegated ||
			node.IndependentReview ||
			node.Kind == protocol.WorkItemKindReview ||
			node.Kind == protocol.WorkItemKindVerify
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
	candidates := preferredSlashNameCandidates(base)
	for _, candidate := range candidates {
		if _, reserved := reservedWorkflowSlashNames[candidate]; reserved {
			continue
		}
		existing, err := s.repository.GetBySlashName(ctx, ownerUserID, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: generated slash_name is invalid", ErrInvalidInput)
	}
	base = candidates[0]
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

func preferredSlashNameCandidates(value string) []string {
	parts := strings.Split(normalizeSlashName(value), "-")
	result := make([]string, 0, len(parts)+1)
	seen := make(map[string]struct{}, len(parts)+1)
	appendCandidate := func(candidate string) {
		candidate = normalizeSlashName(candidate)
		if !workflowSlashNamePattern.MatchString(candidate) {
			return
		}
		if _, duplicate := seen[candidate]; duplicate {
			return
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	for index := len(parts) - 1; index >= 0; index-- {
		appendCandidate(parts[index])
	}
	if len(parts) > 1 {
		appendCandidate(strings.Join(parts[len(parts)-2:], "-"))
	}
	return result
}

type workflowPreviewSource struct {
	AgentID        string
	ConversationID string
}

func (s *Service) storePreview(
	ctx context.Context,
	ownerUserID string,
	preview protocol.WorkGraphWorkflowPreview,
	now time.Time,
	outputLanguage string,
	source workflowPreviewSource,
) error {
	if drafts, ok := s.repository.(DraftRepository); ok {
		_, err := drafts.CreateDraft(ctx, protocol.WorkGraphWorkflowDraft{
			PreviewID:            preview.PreviewID,
			OwnerUserID:          ownerUserID,
			SourceExecutionID:    preview.SourceExecutionID,
			SourceSessionKey:     preview.SourceSessionKey,
			SourceAgentID:        strings.TrimSpace(source.AgentID),
			SourceConversationID: strings.TrimSpace(source.ConversationID),
			OutputLanguage:       outputLanguage,
			HeadRevision:         1,
			SelectedRevision:     1,
			Preview:              cloneWorkflowPreview(preview),
			Versions: []protocol.WorkGraphWorkflowPreviewVersion{{
				Revision: 1, Preview: cloneWorkflowPreview(preview), CreatedAt: now,
			}},
			ExpiresAt: preview.ExpiresAt,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			return err
		}
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(now)
	s.previews[previewCacheKey(ownerUserID, preview.PreviewID)] = workflowPreviewRecord{
		ownerUserID:          ownerUserID,
		preview:              cloneWorkflowPreview(preview),
		sourceAgentID:        strings.TrimSpace(source.AgentID),
		sourceConversationID: strings.TrimSpace(source.ConversationID),
		outputLanguage:       outputLanguage,
	}
	return nil
}

func (s *Service) getPreview(ctx context.Context, ownerUserID string, sessionKey string, previewID string) (protocol.WorkGraphWorkflowPreview, error) {
	if _, err := s.loadDraftByID(ctx, ownerUserID, previewID); err != nil {
		return protocol.WorkGraphWorkflowPreview{}, err
	}
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
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	previewID string,
	slashName string,
	title string,
	description string,
) (protocol.WorkGraphWorkflowPreview, string, bool, error) {
	loadedDraft, err := s.loadDraftByID(ctx, ownerUserID, previewID)
	if err != nil {
		return protocol.WorkGraphWorkflowPreview{}, "", false, err
	}
	legacyReservedSlashName := ""
	if savedID := savedWorkflowID(loadedDraft); savedID != "" {
		target, targetErr := s.repository.GetByID(ctx, ownerUserID, savedID)
		if targetErr != nil {
			return protocol.WorkGraphWorkflowPreview{}, "", false, targetErr
		}
		if target != nil && isBuiltinWorkflowSlashName(target.SlashName) {
			legacyReservedSlashName = normalizeSlashName(target.SlashName)
		}
	}
	s.previewMu.Lock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := previewCacheKey(ownerUserID, previewID)
	record, ok := s.previews[key]
	if !ok || record.ownerUserID != ownerUserID || record.preview.SourceSessionKey != sessionKey {
		s.previewMu.Unlock()
		return protocol.WorkGraphWorkflowPreview{}, "", false, ErrNotFound
	}
	if strings.TrimSpace(record.sourceAgentID) == "" {
		s.previewMu.Unlock()
		return protocol.WorkGraphWorkflowPreview{}, "", false, fmt.Errorf("%w: source Execution has no coordinator Agent", ErrInvalidInput)
	}
	if slashName == "" {
		slashName = record.preview.SlashName
	}
	if title == "" {
		title = record.preview.Title
	}
	if description == "" {
		description = record.preview.Description
	}
	if !workflowSlashNamePattern.MatchString(slashName) || title == "" || description == "" || len([]rune(title)) > 120 || len([]rune(description)) > 500 {
		s.previewMu.Unlock()
		return protocol.WorkGraphWorkflowPreview{}, "", false, fmt.Errorf("%w: confirmed workflow metadata is invalid", ErrInvalidInput)
	}
	if _, reserved := reservedWorkflowSlashNames[slashName]; reserved && slashName != legacyReservedSlashName {
		s.previewMu.Unlock()
		return protocol.WorkGraphWorkflowPreview{}, "", false, fmt.Errorf("%w: /%s", ErrNameConflict, slashName)
	}
	alreadyScheduled := record.saveScheduled
	if alreadyScheduled && (record.preview.SlashName != slashName || record.preview.Title != title || record.preview.Description != description) {
		s.previewMu.Unlock()
		return protocol.WorkGraphWorkflowPreview{}, "", false, fmt.Errorf("%w: preview was already confirmed with different metadata", ErrInvalidInput)
	}
	record.preview.SlashName = slashName
	record.preview.Title = title
	record.preview.Description = description
	record.saveScheduled = true
	s.previews[key] = record
	result := cloneWorkflowPreview(record.preview)
	sourceAgentID := strings.TrimSpace(record.sourceAgentID)
	s.previewMu.Unlock()
	if drafts, supported := s.repository.(DraftRepository); supported {
		if loadedDraft != nil &&
			(loadedDraft.Preview.SlashName != result.SlashName ||
				loadedDraft.Preview.Title != result.Title ||
				loadedDraft.Preview.Description != result.Description) {
			updated, appendErr := drafts.AppendDraftVersion(
				ctx, ownerUserID, previewID, loadedDraft.HeadRevision,
				result, s.now().UTC(), s.now().UTC().Add(workflowPreviewTTL),
			)
			if appendErr != nil {
				s.releasePreviewSaveClaim(ctx, ownerUserID, previewID)
				return protocol.WorkGraphWorkflowPreview{}, "", false, appendErr
			}
			s.hydrateDraft(*updated)
			result = cloneWorkflowPreview(updated.Preview)
		}
		savedRevision := record.savedRevision
		if loadedDraft != nil {
			savedRevision = loadedDraft.SavedRevision
		}
		if err := drafts.SetDraftSaveState(ctx, ownerUserID, previewID, true, record.savedWorkflowID, savedRevision, s.now().UTC()); err != nil {
			s.releasePreviewSaveClaim(ctx, ownerUserID, previewID)
			return protocol.WorkGraphWorkflowPreview{}, "", false, err
		}
	}
	s.previewMu.Lock()
	if latest, ok := s.previews[key]; ok {
		latest.saveScheduled = true
		s.previews[key] = latest
	}
	s.previewMu.Unlock()
	return result, sourceAgentID, alreadyScheduled, nil
}

func (s *Service) releasePreviewSaveClaim(ctx context.Context, ownerUserID string, previewID string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	key := previewCacheKey(ownerUserID, previewID)
	record, ok := s.previews[key]
	if !ok {
		return
	}
	record.saveScheduled = false
	s.previews[key] = record
	if drafts, supported := s.repository.(DraftRepository); supported {
		_ = drafts.SetDraftSaveState(ctx, ownerUserID, previewID, false, record.savedWorkflowID, record.savedRevision, s.now().UTC())
	}
}

func (s *Service) cleanupExpiredPreviews(now time.Time) {
	for key, record := range s.previews {
		if !record.preview.ExpiresAt.After(now) {
			delete(s.previews, key)
		}
	}
	for key, record := range s.editors {
		if !record.expiresAt.After(now) {
			delete(s.editorBySession, record.sessionKey)
			delete(s.editorByPreview, previewCacheKey(record.ownerUserID, record.previewID))
			delete(s.editors, key)
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

func normalizeWorkflowOutputLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "zh", "zh-cn", "zh-hans":
		return "zh"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return ""
	}
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

func newWorkflowEditorID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return "workgraph_editor_" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("workgraph_editor_%d", time.Now().UnixNano())
}
