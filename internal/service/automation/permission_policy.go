// INPUT: 任务定义、Agent 工具默认值与运行时 permission request。
// OUTPUT: 任务授权快照、稳定 capability 指纹、脱敏展示摘要与授权匹配结果。
// POS: automation 权限策略核心；只表达产品边界，不依赖 session 是否在线。
package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const taskPermissionPolicyVersion = 1
const scriptPermissionToolName = "__nexus_host_script__"

var sensitivePermissionInputFragments = []string{
	"access_token", "authorization", "bearer", "cookie", "credential", "password",
	"private_key", "refresh_token", "secret", "session_token", "token", "api_key",
}

var permissionResourceKeys = []string{
	"account_id", "app_token", "base_id", "bitable_id", "block_id", "chat_id",
	"document_id", "document_url", "doc_id", "doc_token", "drive_id", "file_id",
	"file_token", "folder_token", "resource_id", "sheet_id", "spreadsheet_token",
	"table_id", "tenant_id", "thread_id", "url", "wiki_token",
}

var readOnlyPermissionToolNames = map[string]struct{}{
	"feishu_docx_bitable_fields":  {},
	"feishu_docx_bitable_records": {},
	"feishu_docx_bitable_tables":  {},
	"feishu_docx_drive_list":      {},
	"feishu_docx_read":            {},
	"feishu_docx_search":          {},
	"feishu_docx_sheet_find":      {},
	"feishu_docx_sheet_sheets":    {},
	"feishu_docx_sheet_values":    {},
	"feishu_docx_wiki_node":       {},
	"feishu_docx_wiki_nodes":      {},
	"feishu_docx_wiki_space":      {},
	"feishu_docx_wiki_spaces":     {},
}

func (s *Service) ensureTaskPermissionPolicy(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) (automationdomain.ScheduledTask, error) {
	job.PermissionPolicy = normalizeTaskPermissionPolicy(job.PermissionPolicy)
	if job.PermissionPolicy.Revision > 0 {
		if strings.TrimSpace(job.PermissionState) == "" || job.PermissionState == automationdomain.TaskPermissionStateUninitialized {
			job.PermissionState = automationdomain.TaskPermissionStateReady
		}
		return job, nil
	}
	policy, err := s.buildInitialTaskPermissionPolicy(ctx, job, true, true)
	if err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	updated, err := s.repository.UpdateTaskPermissionPolicyIfRevision(
		ctx,
		job.OwnerUserID,
		job.JobID,
		0,
		policy,
		automationdomain.TaskPermissionStateReady,
	)
	if err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	if !updated {
		fresh, loadErr := s.repository.GetScheduledTask(ctx, job.OwnerUserID, job.JobID)
		if loadErr != nil {
			return automationdomain.ScheduledTask{}, loadErr
		}
		if fresh == nil {
			return automationdomain.ScheduledTask{}, automationdomain.ErrJobNotFound
		}
		return *fresh, nil
	}
	job.PermissionPolicy = policy
	job.PermissionState = automationdomain.TaskPermissionStateReady
	job.PendingPermissionRequestID = ""
	return job, nil
}

func (s *Service) buildInitialTaskPermissionPolicy(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	allowDirectScript bool,
	legacyCompat bool,
) (automationdomain.TaskPermissionPolicy, error) {
	options, err := s.taskAgentOptions(ctx, job.AgentID)
	if err != nil {
		return automationdomain.TaskPermissionPolicy{}, err
	}
	return s.buildTaskPermissionPolicyFromOptions(ctx, job, options, allowDirectScript, legacyCompat), nil
}

func (s *Service) buildTaskPermissionPolicyFromOptions(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	options protocol.Options,
	allowDirectScript bool,
	legacyCompat bool,
) automationdomain.TaskPermissionPolicy {
	options.AllowedTools = toolpolicy.WithManagedRuntimeAllowedTools(
		options.AllowedTools,
		s.runtimeImagegenDefaultEnabled(ctx),
	)
	tools := normalizedSortedToolNames(options.AllowedTools)
	policy := automationdomain.TaskPermissionPolicy{
		Version:     taskPermissionPolicyVersion,
		Revision:    1,
		Grants:      make([]automationdomain.TaskPermissionGrant, 0, len(tools)+1),
		DeniedTools: normalizedSortedToolNames(options.DisallowedTools),
	}
	seen := make(map[string]struct{}, len(tools)+1)
	for _, toolName := range tools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		key := strings.ToLower(toolName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		policy.Grants = append(policy.Grants, automationdomain.TaskPermissionGrant{
			GrantID: s.idFactory("grant"),
			Capability: automationdomain.PermissionCapability{
				ToolName: toolName,
				Effect:   classifyPermissionEffect(toolName),
			},
			Source: automationdomain.PermissionGrantSourceAgentSnapshot,
		})
	}
	if automationdomain.NormalizeExecutionKind(job.ExecutionKind) == automationdomain.ExecutionKindScript && (allowDirectScript || legacyCompat) {
		source := automationdomain.PermissionGrantSourceDirectUser
		if legacyCompat {
			source = automationdomain.PermissionGrantSourceLegacyCompat
		}
		policy.Grants = append(policy.Grants, s.scriptPermissionGrant(job, source))
	}
	return normalizeTaskPermissionPolicy(policy)
}

func normalizedSortedToolNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Service) taskAgentOptions(ctx context.Context, agentID string) (protocol.Options, error) {
	if s.agents == nil || strings.TrimSpace(agentID) == "" {
		return protocol.Options{}, nil
	}
	agentValue, err := s.requireAgent(ctx, agentID)
	if err != nil {
		return protocol.Options{}, err
	}
	if agentValue == nil {
		return protocol.Options{}, nil
	}
	return agentValue.Options, nil
}

func normalizeTaskPermissionPolicy(policy automationdomain.TaskPermissionPolicy) automationdomain.TaskPermissionPolicy {
	if policy.Version <= 0 {
		policy.Version = taskPermissionPolicyVersion
	}
	if policy.Grants == nil {
		policy.Grants = []automationdomain.TaskPermissionGrant{}
	}
	return policy
}

func taskRuntimeToolPolicy(job automationdomain.ScheduledTask) *protocol.RuntimeToolPolicy {
	policy := normalizeTaskPermissionPolicy(job.PermissionPolicy)
	// 旧策略没有 denied_tools 字段，继续沿用运行时读取 Agent 当前配置的兼容语义。
	// 新任务即使没有 deny 也会持久化 []，因此能够与旧记录可靠区分。
	if policy.DeniedTools == nil {
		return nil
	}
	allowed := make([]string, 0, len(policy.Grants))
	for _, grant := range policy.Grants {
		if grant.Source != automationdomain.PermissionGrantSourceAgentSnapshot ||
			strings.TrimSpace(grant.Capability.ToolName) == "" {
			continue
		}
		allowed = append(allowed, grant.Capability.ToolName)
	}
	return &protocol.RuntimeToolPolicy{
		AllowedTools:    normalizedSortedToolNames(allowed),
		DisallowedTools: normalizedSortedToolNames(policy.DeniedTools),
	}
}

func (s *Service) scriptPermissionGrant(job automationdomain.ScheduledTask, source string) automationdomain.TaskPermissionGrant {
	return automationdomain.TaskPermissionGrant{
		GrantID: s.idFactory("grant"),
		Capability: automationdomain.PermissionCapability{
			ToolName:      scriptPermissionToolName,
			Effect:        automationdomain.PermissionEffectExecute,
			ResourceScope: scriptPermissionScope(job),
		},
		Source: strings.TrimSpace(source),
	}
}

func scriptPermissionScope(job automationdomain.ScheduledTask) string {
	payload := strings.Join([]string{
		strings.TrimSpace(job.OwnerUserID),
		strings.TrimSpace(job.AgentID),
		strings.TrimSpace(job.Instruction),
	}, "\x00")
	return "sha256:" + sha256String(payload)
}

func taskPermissionMutationIsDirectUser(ctx context.Context, sourceKind string) bool {
	if _, ok := automationexec.ActorAgentID(ctx); ok {
		return false
	}
	return strings.TrimSpace(sourceKind) != automationdomain.SourceKindAgent
}

func (s *Service) taskPolicyForDefinitionUpdate(
	ctx context.Context,
	before automationdomain.ScheduledTask,
	after automationdomain.ScheduledTask,
) automationdomain.TaskPermissionPolicy {
	policy := normalizeTaskPermissionPolicy(before.PermissionPolicy)
	if !taskPermissionBoundaryChanged(before, after) {
		return policy
	}
	policy.Revision++
	retained := make([]automationdomain.TaskPermissionGrant, 0, len(policy.Grants)+1)
	for _, grant := range policy.Grants {
		switch grant.Source {
		case automationdomain.PermissionGrantSourceAgentSnapshot,
			automationdomain.PermissionGrantSourceLegacyCompat:
			if grant.Capability.ToolName != scriptPermissionToolName {
				retained = append(retained, grant)
			}
		}
	}
	policy.Grants = retained
	if automationdomain.NormalizeExecutionKind(after.ExecutionKind) == automationdomain.ExecutionKindScript &&
		taskPermissionMutationIsDirectUser(ctx, after.Source.Kind) {
		policy.Grants = append(policy.Grants, s.scriptPermissionGrant(after, automationdomain.PermissionGrantSourceDirectUser))
	}
	return normalizeTaskPermissionPolicy(policy)
}

func taskPermissionBoundaryChanged(before automationdomain.ScheduledTask, after automationdomain.ScheduledTask) bool {
	return strings.TrimSpace(before.AgentID) != strings.TrimSpace(after.AgentID) ||
		strings.TrimSpace(before.Instruction) != strings.TrimSpace(after.Instruction) ||
		automationdomain.NormalizeExecutionKind(before.ExecutionKind) != automationdomain.NormalizeExecutionKind(after.ExecutionKind) ||
		automationdomain.NormalizePermissionMode(before.PermissionMode) != automationdomain.NormalizePermissionMode(after.PermissionMode) ||
		before.SessionTarget.Normalized() != after.SessionTarget.Normalized()
}

func (s *Service) taskPolicyAllowsCapability(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	capability automationdomain.PermissionCapability,
) (bool, bool, error) {
	policy := normalizeTaskPermissionPolicy(job.PermissionPolicy)
	disallowed := toolpolicy.NormalizeSet(policy.DeniedTools)
	if toolpolicy.Contains(disallowed, capability.ToolName) {
		return false, true, nil
	}
	var legacyCurrentDefaults map[string]struct{}
	if policy.DeniedTools == nil {
		options, err := s.taskAgentOptions(ctx, job.AgentID)
		if err != nil {
			return false, false, err
		}
		options.AllowedTools = toolpolicy.WithManagedRuntimeAllowedTools(
			options.AllowedTools,
			s.runtimeImagegenDefaultEnabled(ctx),
		)
		if toolpolicy.Contains(toolpolicy.NormalizeSet(options.DisallowedTools), capability.ToolName) {
			return false, true, nil
		}
		legacyCurrentDefaults = toolpolicy.NormalizeSet(options.AllowedTools)
	}
	for _, grant := range policy.Grants {
		if !permissionGrantMatches(grant, capability) {
			continue
		}
		if legacyCurrentDefaults != nil &&
			grant.Source == automationdomain.PermissionGrantSourceAgentSnapshot &&
			!toolpolicy.Contains(legacyCurrentDefaults, capability.ToolName) {
			continue
		}
		return true, false, nil
	}
	return false, false, nil
}

func permissionGrantMatches(grant automationdomain.TaskPermissionGrant, capability automationdomain.PermissionCapability) bool {
	granted := grant.Capability
	if !toolpolicy.MatchesItem(capability.ToolName, granted.ToolName) {
		return false
	}
	if granted.ConnectorID != "" && granted.ConnectorID != capability.ConnectorID {
		return false
	}
	if granted.Effect != "" && granted.Effect != capability.Effect {
		return false
	}
	if granted.ResourceScope != "" && granted.ResourceScope != capability.ResourceScope {
		return false
	}
	return true
}

func appendTaskPermissionGrant(
	policy automationdomain.TaskPermissionPolicy,
	grant automationdomain.TaskPermissionGrant,
) automationdomain.TaskPermissionPolicy {
	policy = normalizeTaskPermissionPolicy(policy)
	for _, existing := range policy.Grants {
		if permissionGrantMatches(existing, grant.Capability) &&
			existing.Capability.ResourceScope == grant.Capability.ResourceScope {
			return policy
		}
	}
	policy.Revision++
	policy.Grants = append(policy.Grants, grant)
	return policy
}

func buildPermissionCapability(request sdkpermission.Request) automationdomain.PermissionCapability {
	toolName := strings.TrimSpace(request.ToolName)
	return automationdomain.PermissionCapability{
		ToolName:         toolName,
		ConnectorID:      connectorIDForPermissionRequest(toolName, request.Input),
		Effect:           classifyPermissionEffect(toolName),
		ResourceScope:    permissionResourceScope(request.Input),
		InputFingerprint: permissionInputFingerprint(toolName, request.Input),
	}
}

func buildScriptPermissionCapability(job automationdomain.ScheduledTask) automationdomain.PermissionCapability {
	return automationdomain.PermissionCapability{
		ToolName:         scriptPermissionToolName,
		Effect:           automationdomain.PermissionEffectExecute,
		ResourceScope:    scriptPermissionScope(job),
		InputFingerprint: sha256String(strings.TrimSpace(job.Instruction)),
	}
}

func connectorIDForPermissionRequest(toolName string, input map[string]any) string {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(toolName), "mcp__"), "__")
	if len(parts) < 2 || parts[0] != "nexus_connectors" {
		return ""
	}
	leaf := strings.Join(parts[1:], "__")
	if leaf == "connector_call" {
		return strings.TrimSpace(permissionStringValue(input["connector_id"]))
	}
	if strings.HasPrefix(leaf, "feishu_docx_") {
		return "feishu-docx"
	}
	return ""
}

func classifyPermissionEffect(toolName string) string {
	leaf := strings.ToLower(permissionToolLeaf(toolName))
	if _, readOnly := readOnlyPermissionToolNames[leaf]; readOnly {
		return automationdomain.PermissionEffectRead
	}
	for _, fragment := range []string{"delete", "remove", "write", "update", "append", "create", "edit", "send", "post", "put", "patch", "move", "rename"} {
		if strings.Contains(leaf, fragment) {
			return automationdomain.PermissionEffectWrite
		}
	}
	for _, fragment := range []string{"read", "get", "list", "search", "find", "inspect", "query", "status", "fetch", "view"} {
		if strings.Contains(leaf, fragment) {
			return automationdomain.PermissionEffectRead
		}
	}
	return automationdomain.PermissionEffectExecute
}

func permissionToolLeaf(toolName string) string {
	value := strings.TrimSpace(toolName)
	for _, separator := range []string{"__", ".", "/"} {
		if index := strings.LastIndex(value, separator); index >= 0 {
			value = value[index+len(separator):]
		}
	}
	return value
}

func permissionInputFingerprint(toolName string, input map[string]any) string {
	body, _ := json.Marshal(map[string]any{
		"tool_name": strings.TrimSpace(toolName),
		"input":     input,
	})
	return sha256String(string(body))
}

func permissionResourceScope(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	items := make([]string, 0, len(permissionResourceKeys))
	for _, key := range permissionResourceKeys {
		value, ok := permissionLookupValue(input, key)
		if !ok {
			continue
		}
		normalized := normalizePermissionResourceValue(key, value)
		if normalized != "" {
			items = append(items, key+"=sha256:"+sha256String(normalized))
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Strings(items)
	return strings.Join(items, "&")
}

func permissionLookupValue(input map[string]any, target string) (any, bool) {
	for key, value := range input {
		if strings.EqualFold(strings.TrimSpace(key), target) {
			return value, true
		}
	}
	return nil, false
}

func normalizePermissionResourceValue(key string, value any) string {
	text := strings.TrimSpace(permissionStringValue(value))
	if text == "" {
		return ""
	}
	if key == "url" || strings.HasSuffix(key, "_url") {
		parsed, err := url.Parse(text)
		if err == nil && parsed.Scheme != "" {
			parsed.RawQuery = ""
			parsed.Fragment = ""
			text = parsed.String()
		}
	}
	return truncatePermissionText(text, 512)
}

func summarizePermissionInput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		if permissionInputKeySensitive(key) {
			result[key] = "[redacted]"
			continue
		}
		result[key] = summarizePermissionValue(key, value, 0)
	}
	return result
}

func summarizePermissionValue(key string, value any, depth int) any {
	if depth >= 2 {
		return "[nested]"
	}
	switch typed := value.(type) {
	case nil, bool, float64, int, int64:
		return typed
	case string:
		return truncatePermissionText(normalizePermissionResourceValue(strings.ToLower(strings.TrimSpace(key)), typed), 256)
	case []any:
		if len(typed) > 8 {
			return fmt.Sprintf("[%d items]", len(typed))
		}
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, summarizePermissionValue(key, item, depth+1))
		}
		return items
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if permissionInputKeySensitive(key) {
				result[key] = "[redacted]"
			} else {
				result[key] = summarizePermissionValue(key, item, depth+1)
			}
		}
		return result
	default:
		return truncatePermissionText(fmt.Sprint(typed), 256)
	}
}

func permissionInputKeySensitive(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, fragment := range sensitivePermissionInputFragments {
		if normalized == fragment || strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func permissionStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func sha256String(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func truncatePermissionText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func permissionGrantApprovedAt(now time.Time) *time.Time {
	value := now.UTC()
	return &value
}
