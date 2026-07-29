package workspaceisolation

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const workspacePolicyPublicDenial = "该操作超出当前用户被授予的工作区范围。"

var shellTokenPattern = regexp.MustCompile(`[^\s"'` + "`" + `;|&()<>{}]+`)
var shellRedirectionPathPattern = regexp.MustCompile(
	`(?:^|[\s;|&])(?:&>|[012]?>>|[012]?>)\s*([^\s"'` + "`" + `;|&()<>{}]+)`,
)
var unixShellVariablePattern = regexp.MustCompile(
	`\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[^}]+\})`,
)
var windowsShellVariablePattern = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)

var readPathToolNames = map[string]struct{}{
	"glob": {}, "grep": {}, "lsp": {}, "ls": {}, "read": {}, "viewimage": {},
}

var writePathToolNames = map[string]struct{}{
	"delete": {}, "edit": {}, "multiedit": {}, "notebookedit": {},
	"rename": {}, "move": {}, "write": {}, "worktreecreate": {}, "worktreeremove": {},
}

var pathInputKeys = map[string]struct{}{
	"cwd": {}, "directory": {}, "file": {}, "file_path": {}, "notebook_path": {},
	"path": {}, "root": {}, "source": {}, "src": {}, "target": {}, "worktree_path": {},
}

var writePathInputKeys = map[string]struct{}{
	"destination": {}, "dst": {}, "new_path": {}, "output": {}, "target": {},
	"worktree_path": {},
}

func withWorkspacePolicyHook(
	options agentclient.Options,
	mode Mode,
	policy Policy,
) agentclient.Options {
	hooks := cloneHookMatchers(options.Hooks.Matchers)
	// SDK 按 matcher 顺序合并 permission decision；mandatory policy 在宿主
	// 初始化时已知的 PreToolUse hook 之后执行，检查其更新后的输入并保留否决权。
	// 运行期动态 hook 的顺序不作为安全边界，最终仍由 launcher/Landlock 收口。
	hooks[sdkhook.EventPreToolUse] = append(
		hooks[sdkhook.EventPreToolUse],
		sdkhook.Matcher{
			Hooks: []sdkhook.Callback{workspacePolicyCallback(mode, policy)},
			// 路径判断只做本地 stat/EvalSymlinks，超时意味着宿主状态异常。
			Timeout: 2 * time.Second,
		},
	)
	options.Hooks.Matchers = hooks
	return options
}

func workspacePolicyCallback(mode Mode, policy Policy) sdkhook.Callback {
	return func(_ context.Context, input sdkhook.Input, toolUseID string) (sdkhook.Output, error) {
		violation := inspectToolAccess(policy, input)
		if violation == nil {
			return allowWorkspacePolicyOutput(), nil
		}
		slog.Error("runtime workspace policy 拒绝越界工具调用",
			"owner_user_id", policy.OwnerUserID,
			"runtime_kind", policy.RuntimeKind,
			"policy_generation", policy.Generation,
			"tool_name", strings.TrimSpace(input.ToolName),
			"tool_use_id", strings.TrimSpace(toolUseID),
			"reason", violation.reason,
			"path", violation.path,
			"mode", string(mode),
		)
		if mode == ModeAudit {
			return allowWorkspacePolicyOutput(), nil
		}
		return denyWorkspacePolicyOutput(), nil
	}
}

type policyViolation struct {
	reason string
	path   string
}

func inspectToolAccess(policy Policy, input sdkhook.Input) *policyViolation {
	toolName := normalizedToolName(input.ToolName)
	if toolName == "" {
		return &policyViolation{reason: "tool name 为空"}
	}
	cwd := strings.TrimSpace(input.CWD)
	if cwd == "" {
		cwd = policy.CWD
	}
	if _, err := policy.authorize(cwd, false); err != nil {
		return &policyViolation{reason: "runtime cwd 越界", path: cwd}
	}
	toolInput, ok := input.ToolInput.(map[string]any)
	if !ok {
		if toolNeedsPathPolicy(toolName) {
			return &policyViolation{reason: "tool input 不是对象"}
		}
		return nil
	}
	if toolName == "bash" || toolName == "shell" || toolName == "powershell" {
		return inspectShellAccess(policy, cwd, toolInput)
	}

	writeTool := false
	if _, ok = writePathToolNames[toolName]; ok {
		writeTool = true
	}
	if _, ok = readPathToolNames[toolName]; !ok && !writeTool {
		return inspectGenericPathFields(policy, cwd, toolInput)
	}
	paths := collectPathFields(toolInput)
	if len(paths) == 0 {
		if toolName == "glob" || toolName == "grep" {
			paths = []pathCandidate{{path: cwd}}
		} else {
			return &policyViolation{reason: "路径工具缺少路径字段"}
		}
	}
	for _, candidate := range paths {
		write := writeTool || candidate.write
		resolved, err := resolveToolPath(cwd, candidate.path)
		if err != nil {
			return &policyViolation{reason: err.Error(), path: candidate.path}
		}
		if _, err = policy.authorize(resolved, write); err != nil {
			if write && sessionSummaryEditAuthorized(policy, toolName, resolved) {
				continue
			}
			return &policyViolation{reason: accessReason(write), path: resolved}
		}
	}
	return nil
}

// sessionSummaryEditAuthorized 只放行 nxs 内部会话摘要的单文件 Edit。
// runtime 目录仍不是普通工具写根，其他内部状态继续由宿主和 Landlock 管理。
func sessionSummaryEditAuthorized(policy Policy, toolName string, path string) bool {
	if toolName != "edit" ||
		!strings.EqualFold(strings.TrimSpace(policy.RuntimeKind), "nxs") {
		return false
	}
	projectsRoot, err := canonicalPolicyPath(
		filepath.Join(appfs.UserRuntimeRoot(policy.OwnerUserID), "projects"),
	)
	if err != nil {
		return false
	}
	target, err := canonicalPolicyPath(path)
	if err != nil || !pathWithinPolicyRoot(target, projectsRoot) {
		return false
	}
	relative, err := filepath.Rel(projectsRoot, target)
	if err != nil {
		return false
	}
	segments := strings.Split(relative, string(os.PathSeparator))
	return len(segments) == 4 &&
		validSessionSummarySegment(segments[0]) &&
		validSessionSummarySegment(segments[1]) &&
		segments[2] == "session-memory" &&
		segments[3] == "summary.md"
}

func validSessionSummarySegment(segment string) bool {
	segment = strings.TrimSpace(segment)
	return segment != "" && segment != "." && segment != ".."
}

func inspectGenericPathFields(
	policy Policy,
	cwd string,
	input map[string]any,
) *policyViolation {
	for _, candidate := range collectPathFields(input) {
		resolved, err := resolveToolPath(cwd, candidate.path)
		if err != nil {
			return &policyViolation{reason: err.Error(), path: candidate.path}
		}
		if _, err = policy.authorize(resolved, candidate.write); err != nil {
			return &policyViolation{reason: accessReason(candidate.write), path: resolved}
		}
	}
	return nil
}

func inspectShellAccess(
	policy Policy,
	cwd string,
	input map[string]any,
) *policyViolation {
	for _, candidate := range collectPathFields(input) {
		resolved, err := resolveToolPath(cwd, candidate.path)
		if err != nil {
			return &policyViolation{reason: err.Error(), path: candidate.path}
		}
		if _, err = policy.authorize(resolved, false); err != nil {
			return &policyViolation{reason: "Shell 工作目录越界", path: resolved}
		}
	}
	command, ok := stringInput(input, "command")
	if !ok || strings.TrimSpace(command) == "" {
		return &policyViolation{reason: "Shell command 为空"}
	}
	if reason := forbiddenNexusctlScope(command); reason != "" {
		return &policyViolation{reason: reason}
	}
	for _, match := range shellRedirectionPathPattern.FindAllStringSubmatch(command, -1) {
		if len(match) != 2 || strings.ContainsRune(match[1], '$') {
			continue
		}
		resolved, err := resolveToolPath(cwd, match[1])
		if err != nil {
			return &policyViolation{reason: err.Error(), path: match[1]}
		}
		if resolved == "/dev/null" || resolved == "/dev/tty" {
			continue
		}
		if _, err = policy.authorize(resolved, true); err != nil {
			return &policyViolation{reason: "Shell 重定向目标不可写", path: resolved}
		}
	}
	for _, token := range shellTokenPattern.FindAllString(command, -1) {
		path, ok := shellTokenPath(token)
		if !ok {
			continue
		}
		if shellDynamicPathPrefixAuthorized(policy, cwd, path) {
			continue
		}
		resolved, err := resolveToolPath(cwd, path)
		if err != nil {
			return &policyViolation{reason: err.Error(), path: path}
		}
		if shellSystemPath(resolved) {
			continue
		}
		if _, err = policy.authorize(resolved, false); err != nil {
			return &policyViolation{reason: "Shell 显式路径越界", path: resolved}
		}
	}
	return nil
}

// shellDynamicPathPrefixAuthorized 只判断变量前已经明确出现的静态目录。
// 变量展开后的最终目标仍由 launcher/Landlock 的系统调用边界裁决。
func shellDynamicPathPrefixAuthorized(policy Policy, cwd string, path string) bool {
	variableIndex := firstShellVariableIndex(path)
	if variableIndex <= 0 || explicitShellTraversal(path) {
		return false
	}
	prefix := strings.TrimRight(path[:variableIndex], `/\`)
	if prefix == "" {
		return false
	}
	resolved, err := resolveToolPath(cwd, prefix)
	if err != nil {
		return false
	}
	if shellSystemPath(resolved) {
		return true
	}
	_, err = policy.authorize(resolved, false)
	return err == nil
}

func firstShellVariableIndex(value string) int {
	index := -1
	for _, pattern := range []*regexp.Regexp{
		unixShellVariablePattern,
		windowsShellVariablePattern,
	} {
		match := pattern.FindStringIndex(value)
		if len(match) == 2 && (index < 0 || match[0] < index) {
			index = match[0]
		}
	}
	return index
}

func explicitShellTraversal(value string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(value, `\`, "/"), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func shellSystemPath(path string) bool {
	for _, root := range []string{
		"/bin", "/dev", "/etc", "/lib", "/lib64", "/proc", "/sbin", "/sys", "/usr",
	} {
		if pathWithinPolicyRoot(path, root) {
			return true
		}
	}
	return false
}

func forbiddenNexusctlScope(command string) string {
	lower := strings.ToLower(command)
	if !strings.Contains(lower, "nexusctl") {
		return ""
	}
	return "runtime 暂不提供直接 nexusctl 控制面 broker"
}

type pathCandidate struct {
	path  string
	write bool
}

func collectPathFields(input map[string]any) []pathCandidate {
	candidates := make([]pathCandidate, 0)
	for key, value := range input {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		_, isPath := pathInputKeys[normalizedKey]
		_, isWritePath := writePathInputKeys[normalizedKey]
		if !isPath && !isWritePath && normalizedKey != "paths" && normalizedKey != "files" {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				candidates = append(candidates, pathCandidate{path: trimmed, write: isWritePath})
			}
		case []string:
			for _, item := range typed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					candidates = append(candidates, pathCandidate{path: trimmed, write: isWritePath})
				}
			}
		case []any:
			for _, item := range typed {
				if path, ok := item.(string); ok && strings.TrimSpace(path) != "" {
					candidates = append(candidates, pathCandidate{
						path:  strings.TrimSpace(path),
						write: isWritePath,
					})
				}
			}
		}
	}
	slices.SortFunc(candidates, func(left pathCandidate, right pathCandidate) int {
		if comparison := strings.Compare(left.path, right.path); comparison != 0 {
			return comparison
		}
		if left.write == right.write {
			return 0
		}
		if left.write {
			return 1
		}
		return -1
	})
	return slices.Compact(candidates)
}

func resolveToolPath(cwd string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("工具路径为空")
	}
	if strings.HasPrefix(value, "~") {
		return "", fmt.Errorf("工具路径包含未展开的 home 简写")
	}
	if isWindowsAbsoluteShellPath(value) {
		return "", fmt.Errorf("工具路径包含 Windows 绝对路径")
	}
	if strings.ContainsRune(value, '$') {
		return "", fmt.Errorf("工具路径包含未解析的环境变量")
	}
	if windowsShellVariablePattern.MatchString(value) {
		return "", fmt.Errorf("工具路径包含未解析的环境变量")
	}
	value = nonGlobPrefix(value)
	if value == "" {
		value = "."
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	return value, nil
}

func nonGlobPrefix(path string) string {
	index := strings.IndexFunc(path, func(character rune) bool {
		return character == '*' || character == '?' || character == '[' || character == '{'
	})
	if index < 0 {
		return path
	}
	prefix := path[:index]
	if prefix == "" {
		return "."
	}
	if strings.HasSuffix(prefix, string(filepath.Separator)) {
		return strings.TrimSuffix(prefix, string(filepath.Separator))
	}
	return filepath.Dir(prefix)
}

func shellTokenPath(token string) (string, bool) {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	// shellTokenPattern 会把命令替换 `$(...)` 的 `$` 单独切出来。
	// 单独的 `$` 是 shell 语法，不是路径；把它当成环境变量会误拒绝
	// 合法的命令（例如 `ps -u $(whoami)`）。真正带路径语义的变量
	// 仍会在下面保留并交给未展开变量检查。
	if token == "" || token == "$" || strings.Contains(token, "://") {
		return "", false
	}
	if _, value, ok := strings.Cut(token, "="); ok {
		token = value
	}
	token = strings.Trim(token, ",:")
	switch {
	case filepath.IsAbs(token):
		return token, true
	case strings.HasPrefix(token, "~"):
		// shell 会在执行前把 ~ 展开到 home；宿主 hook 无法安全推断
		// 目标用户，因此宁可拒绝未展开的 home 简写，避免绕过 owner 根。
		return token, true
	case unixShellVariablePattern.MatchString(token) ||
		windowsShellVariablePattern.MatchString(token):
		// 裸变量不携带可静态判断的路径语义，交给系统调用级隔离；
		// 带目录分隔符的变量路径继续检查其静态前缀。
		if strings.ContainsAny(token, `/\`) {
			return token, true
		}
		return "", false
	case isWindowsAbsoluteShellPath(token):
		return token, true
	case token == "..", strings.HasPrefix(token, "../"), strings.Contains(token, "/../"):
		return token, true
	default:
		return "", false
	}
}

func isWindowsAbsoluteShellPath(value string) bool {
	return len(value) >= 3 &&
		((value[0] >= 'a' && value[0] <= 'z') ||
			(value[0] >= 'A' && value[0] <= 'Z')) &&
		value[1] == ':' &&
		(value[2] == '\\' || value[2] == '/')
}

func normalizedToolName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "__"); index >= 0 {
		name = name[index+2:]
	}
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, name)
}

func toolNeedsPathPolicy(toolName string) bool {
	if toolName == "bash" || toolName == "shell" || toolName == "powershell" {
		return true
	}
	if _, ok := readPathToolNames[toolName]; ok {
		return true
	}
	_, ok := writePathToolNames[toolName]
	return ok
}

func accessReason(write bool) string {
	if write {
		return "目标不在可写根内"
	}
	return "目标不在可读根内"
}

func stringInput(input map[string]any, key string) (string, bool) {
	value, ok := input[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func allowWorkspacePolicyOutput() sdkhook.Output {
	// 安全 hook 放行时返回空响应，不能覆盖后续用户 hook 或权限判断。
	return sdkhook.Output{}
}

func denyWorkspacePolicyOutput() sdkhook.Output {
	return sdkhook.Output{
		SpecificOutput: &sdkhook.SpecificOutput{
			HookEventName:            sdkhook.EventPreToolUse,
			PermissionDecision:       sdkpermission.BehaviorDeny,
			PermissionDecisionReason: workspacePolicyPublicDenial,
		},
	}
}

func cloneHookMatchers(
	input map[sdkhook.Event][]sdkhook.Matcher,
) map[sdkhook.Event][]sdkhook.Matcher {
	output := make(map[sdkhook.Event][]sdkhook.Matcher, len(input)+1)
	for event, matchers := range input {
		copied := make([]sdkhook.Matcher, 0, len(matchers))
		for _, matcher := range matchers {
			next := matcher
			next.Hooks = slices.Clone(matcher.Hooks)
			copied = append(copied, next)
		}
		output[event] = copied
	}
	return output
}
