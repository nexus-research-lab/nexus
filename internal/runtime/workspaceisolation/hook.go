package workspaceisolation

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const (
	workspacePolicyPublicDenial  = "该操作超出当前用户被授予的工作区范围。"
	mainAgentNexusctlScopeDenial = "主智能体调用 Nexus 控制面 CLI 时已有宿主注入的 owner 作用域；请移除 --global-scope、--scope-user-id 和作用域环境变量覆盖后重试。"
)

var unixShellVariablePattern = regexp.MustCompile(
	`\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[^}]+\})`,
)
var simpleBracedUnixShellVariablePattern = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)
var bracedUnixShellTokenPattern = regexp.MustCompile(
	`[^\s"'` + "`" + `;|&(){}]*\$\{[^}]+\}[^\s"'` + "`" + `;|&(){}]*`,
)
var windowsDriveExpansionPattern = regexp.MustCompile(`(?:^|[-=+?])[A-Za-z]:`)
var rootedExpansionPattern = regexp.MustCompile(`(?:^|[-=+?])(?:[/\\]|~(?:[/\\]|$))`)
var shellRemoteURLPattern = regexp.MustCompile(`(?i)^([A-Za-z][A-Za-z0-9+.-]*)://`)
var shellRemoteURLExpansionPattern = regexp.MustCompile(
	`(?i)(?:[-=+?])([A-Za-z][A-Za-z0-9+.-]*):/{2}`,
)
var shellFileURLExpansionPattern = regexp.MustCompile(`(?i)(?:[-=+?])file:/+`)
var windowsShellVariablePattern = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)
var nexusctlCommandTextPattern = regexp.MustCompile(
	`(?i)(?:^|[^A-Za-z0-9_.-])nexusctl(?:\.(?:bat|cmd|exe|ps1))?(?:$|[^A-Za-z0-9_.-])`,
)
var nexuscfgCommandTextPattern = regexp.MustCompile(
	`(?i)(?:^|[^A-Za-z0-9_.-])nexuscfg(?:\.(?:bat|cmd|exe|ps1))?(?:$|[^A-Za-z0-9_.-])`,
)
var shellEscapeReplacer = strings.NewReplacer(`\`, "", "^", "", "`", "")

var nexusctlScopeEnvironmentNames = map[string]struct{}{
	"CLAUDE_CONFIG_DIR":         {},
	"NEXUS_CONFIG_DIR":          {},
	"NEXUS_RUNTIME_SCOPE_MODE":  {},
	"NEXUS_STATE_ROOT":          {},
	"NEXUSCTL_USER_ID":          {},
	"NEXUSCTL_WORKSPACE_PATH":   {},
	"NEXUSCTL_COMMAND_PATH":     {},
	"NEXUSCFG_COMMAND_PATH":     {},
	"NEXUSCFG_BROKER_URL":       {},
	"NEXUSCFG_CAPABILITY_TOKEN": {},
}

var nexusctlScopeFlags = map[string]struct{}{
	"--global-scope":  {},
	"--scope-user-id": {},
}

var nexusctlExecutableNames = map[string]struct{}{
	"nexusctl":     {},
	"nexusctl.bat": {},
	"nexusctl.cmd": {},
	"nexusctl.exe": {},
	"nexusctl.ps1": {},
}

var nexuscfgExecutableNames = map[string]struct{}{
	"nexuscfg":     {},
	"nexuscfg.bat": {},
	"nexuscfg.cmd": {},
	"nexuscfg.exe": {},
	"nexuscfg.ps1": {},
}

var powerShellEnvironmentMutationCommands = map[string]struct{}{
	"ci":          {},
	"clear-item":  {},
	"new-item":    {},
	"ni":          {},
	"remove-item": {},
	"ri":          {},
	"sc":          {},
	"set-content": {},
	"set-item":    {},
	"si":          {},
}

var windowsSlashOptions = map[string]map[string]struct{}{
	"cmd": {
		"/?": {}, "/a": {}, "/c": {}, "/d": {}, "/e": {}, "/f": {},
		"/k": {}, "/q": {}, "/s": {}, "/u": {}, "/v": {},
	},
	"cmd.exe": {
		"/?": {}, "/a": {}, "/c": {}, "/d": {}, "/e": {}, "/f": {},
		"/k": {}, "/q": {}, "/s": {}, "/u": {}, "/v": {},
	},
	"findstr": {
		"/?": {}, "/b": {}, "/c": {}, "/e": {}, "/f": {}, "/i": {},
		"/l": {}, "/m": {}, "/n": {}, "/o": {}, "/off[line]": {},
		"/p": {}, "/r": {}, "/s": {}, "/v": {}, "/x": {},
	},
	"findstr.exe": {
		"/?": {}, "/b": {}, "/c": {}, "/e": {}, "/f": {}, "/i": {},
		"/l": {}, "/m": {}, "/n": {}, "/o": {}, "/off[line]": {},
		"/p": {}, "/r": {}, "/s": {}, "/v": {}, "/x": {},
	},
	"ping":     {"/?": {}},
	"ping.exe": {"/?": {}},
}

var powerShellEncodedCommandFlags = map[string]struct{}{
	"-e": {}, "-ec": {}, "-en": {}, "-enc": {}, "-enco": {},
	"-encod": {}, "-encode": {}, "-encoded": {}, "-encodedc": {},
	"-encodedco": {}, "-encodedcom": {}, "-encodedcomm": {},
	"-encodedcomma": {}, "-encodedcomman": {}, "-encodedcommand": {},
}

var shellPathArgumentCommands = map[string]struct{}{
	"add-content": {}, "cat": {}, "cd": {}, "chdir": {}, "clear-content": {},
	"copy": {}, "copy-item": {}, "cp": {}, "curl": {}, "del": {}, "dir": {},
	"erase": {}, "export-csv": {}, "find": {}, "gc": {}, "gci": {},
	"get-acl": {}, "get-childitem": {}, "get-content": {}, "get-filehash": {},
	"get-item": {}, "gi": {}, "grep": {}, "head": {}, "import-clixml": {},
	"import-csv": {}, "invoke-item": {}, "less": {}, "ls": {}, "mkdir": {},
	"more": {}, "move": {}, "move-item": {}, "mv": {}, "new-item": {},
	"ni": {}, "node": {}, "out-file": {}, "python": {}, "python3": {},
	"readlink": {}, "realpath": {}, "remove-item": {}, "rename-item": {},
	"resolve-path": {}, "ri": {}, "rm": {}, "rmdir": {}, "set-content": {},
	"set-location": {}, "stat": {}, "tail": {}, "test-path": {}, "touch": {},
	"type": {}, "wget": {},
}

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

// withRawNexusctlDenyHook closes the control-plane gap when path isolation is
// disabled. Audit/enforce use the full workspace policy callback instead.
func withRawNexusctlDenyHook(options agentclient.Options) agentclient.Options {
	hooks := cloneHookMatchers(options.Hooks.Matchers)
	hooks[sdkhook.EventPreToolUse] = append(
		hooks[sdkhook.EventPreToolUse],
		sdkhook.Matcher{Hooks: []sdkhook.Callback{func(
			_ context.Context,
			input sdkhook.Input,
			_ string,
		) (sdkhook.Output, error) {
			toolName := normalizedToolName(input.ToolName)
			if toolName != "bash" && toolName != "shell" && toolName != "powershell" {
				return allowWorkspacePolicyOutput(), nil
			}
			toolInput, ok := input.ToolInput.(map[string]any)
			if !ok {
				return allowWorkspacePolicyOutput(), nil
			}
			command, ok := stringInput(toolInput, "command")
			if !ok || forbiddenNexusctlScope(Policy{}, command, shellSyntaxFor(toolName)) == "" {
				return allowWorkspacePolicyOutput(), nil
			}
			return denyWorkspacePolicyOutput(true, ""), nil
		}}},
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
			"is_main_agent", policy.IsMainAgent,
			"mode", string(mode),
		)
		if mode == ModeAudit && !violation.terminal {
			return allowWorkspacePolicyOutput(), nil
		}
		return denyWorkspacePolicyOutput(violation.terminal, violation.publicReason), nil
	}
}

type policyViolation struct {
	reason       string
	path         string
	publicReason string
	terminal     bool
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
		return inspectShellAccess(policy, cwd, toolName, toolInput)
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
			return &policyViolation{reason: accessReason(write), path: resolved}
		}
	}
	return nil
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
	toolName string,
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
	return inspectShellCommand(policy, cwd, toolName, command, 0)
}

func inspectShellCommand(
	policy Policy,
	cwd string,
	toolName string,
	command string,
	depth int,
) *policyViolation {
	if depth >= 4 {
		return &policyViolation{reason: "Shell 嵌套命令层级过深"}
	}
	syntax := shellSyntaxFor(toolName)
	windowsSlashRoot := runtime.GOOS == "windows" && toolName != "bash"
	if reason := forbiddenNexusctlScope(policy, command, syntax); reason != "" {
		violation := &policyViolation{
			reason:   reason,
			terminal: !policy.IsMainAgent,
		}
		if policy.IsMainAgent {
			// 主智能体已经具备当前 owner 的宿主注入作用域。旧会话仍可能
			// 生成历史 global/scope flags；拒绝该次调用即可，让模型在同一
			// turn 内按明确反馈移除覆盖并重试，不应把整轮变成 hook_stopped。
			violation.publicReason = mainAgentNexusctlScopeDenial
		}
		return violation
	}
	if shellCommandUsesEncodedPowerShell(command, syntax) {
		return &policyViolation{reason: "PowerShell EncodedCommand 无法由宿主静态检查"}
	}
	for _, path := range shellRedirectionPaths(command, syntax) {
		if variableIndex := firstShellVariableIndex(path); variableIndex >= 0 {
			if policy.IsMainAgent {
				return &policyViolation{reason: "主智能体 Shell 重定向目标不能包含动态变量", path: path}
			}
			if shellDynamicPathPrefixAuthorized(policy, cwd, path, true, windowsSlashRoot) {
				continue
			}
			if variableIndex == 0 {
				continue
			}
			return &policyViolation{reason: "Shell 重定向动态路径前缀不可写", path: path}
		}
		resolved, err := resolveShellToolPath(cwd, path, windowsSlashRoot)
		if err != nil {
			return &policyViolation{reason: err.Error(), path: path}
		}
		if resolved == "/dev/null" || resolved == "/dev/tty" {
			continue
		}
		if _, err = policy.authorize(resolved, true); err != nil {
			return &policyViolation{reason: "Shell 重定向目标不可写", path: resolved}
		}
	}
	if path := complexBracedShellPath(command); path != "" {
		return &policyViolation{reason: "Shell 路径包含复杂环境变量展开", path: path}
	}
	pathParts := shellCommandParts(command, syntax)
	staticAssignments := make(map[string]string)
	for index, part := range pathParts {
		if part.operator != 0 || shellNumericEscapeArgument(pathParts, index) ||
			windowsSlashOption(pathParts, index, windowsSlashRoot) {
			continue
		}
		if name, value, assigned := strings.Cut(part.value, "="); assigned && shellAssignmentName(name) {
			delete(staticAssignments, name)
			if !unixShellVariablePattern.MatchString(value) &&
				!windowsShellVariablePattern.MatchString(value) {
				staticAssignments[name] = value
			}
			continue
		}
		if bracedShellRemoteURLValue(part.value) {
			continue
		}
		if policy.IsMainAgent && mainAgentDynamicPathArgument(pathParts, index) {
			return &policyViolation{reason: "主智能体 Shell 路径不能包含动态变量", path: part.value}
		}
		if standaloneComplexBracedShellValue(part.value) {
			continue
		}
		path, ok := shellTokenPath(
			expandStaticShellPath(part.value, staticAssignments),
			windowsSlashRoot,
		)
		if !ok {
			continue
		}
		if policy.IsMainAgent && firstShellVariableIndex(path) >= 0 {
			return &policyViolation{reason: "主智能体 Shell 路径不能包含动态变量", path: path}
		}
		if shellDynamicPathPrefixAuthorized(policy, cwd, path, false, windowsSlashRoot) {
			continue
		}
		resolved, err := resolveShellToolPath(cwd, path, windowsSlashRoot)
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
	for _, nested := range nestedShellCommands(command, syntax) {
		if violation := inspectShellCommand(
			policy,
			cwd,
			nested.toolName,
			nested.command,
			depth+1,
		); violation != nil {
			return violation
		}
	}
	return nil
}

// expandStaticShellPath 只展开当前命令内已知的字面量变量赋值。
func expandStaticShellPath(value string, assignments map[string]string) string {
	match := unixShellVariablePattern.FindStringIndex(value)
	if len(match) != 2 || match[0] != 0 {
		return value
	}
	name := strings.TrimSuffix(strings.TrimPrefix(value[match[0]:match[1]], "${"), "}")
	name = strings.TrimPrefix(name, "$")
	replacement, ok := assignments[name]
	if !ok {
		return value
	}
	return replacement + value[match[1]:]
}

type shellCommandPart struct {
	value    string
	operator rune
	quoted   bool
	unquoted bool
}

type shellSyntax struct {
	backslashEscape bool
	backtickEscape  bool
	caretEscape     bool
}

type nestedShellCommand struct {
	toolName string
	command  string
}

func shellSyntaxFor(toolName string) shellSyntax {
	switch toolName {
	case "bash":
		return shellSyntax{backslashEscape: true}
	case "powershell":
		return shellSyntax{backtickEscape: true}
	case "shell":
		return shellSyntax{caretEscape: runtime.GOOS == "windows"}
	default:
		return shellSyntax{}
	}
}

func shellCommandUsesEncodedPowerShell(command string, syntax shellSyntax) bool {
	parts := shellCommandParts(command, syntax)
	for index, part := range parts {
		if part.operator != 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(part.value)), ".exe")
		if name != "powershell" && name != "pwsh" {
			continue
		}
		for cursor := index + 1; cursor < len(parts) && parts[cursor].operator == 0; cursor++ {
			if _, ok := powerShellEncodedCommandFlags[strings.ToLower(parts[cursor].value)]; ok {
				return true
			}
		}
	}
	return false
}

func nestedShellCommands(command string, syntax shellSyntax) []nestedShellCommand {
	parts := shellCommandParts(command, syntax)
	commands := make([]nestedShellCommand, 0)
	for index, part := range parts {
		if part.operator != 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(part.value)), ".exe")
		toolName, flags, joinRemainder := nestedShellDescriptor(name)
		if toolName == "" {
			continue
		}
		flagIndex := nextMatchingShellWord(parts, index, toolName, flags)
		commandIndex := nextShellWordIndex(parts, flagIndex)
		if flagIndex < 0 || commandIndex < 0 {
			continue
		}
		nestedCommand := parts[commandIndex].value
		if joinRemainder {
			nestedCommand = joinShellWords(parts, commandIndex)
		}
		if strings.TrimSpace(nestedCommand) != "" {
			commands = append(commands, nestedShellCommand{
				toolName: toolName,
				command:  nestedCommand,
			})
		}
	}
	return commands
}

func nestedShellDescriptor(name string) (string, map[string]struct{}, bool) {
	switch name {
	case "bash", "dash", "sh", "zsh":
		return "bash", map[string]struct{}{"-c": {}}, false
	case "cmd":
		return "shell", map[string]struct{}{`/c`: {}, `/k`: {}}, true
	case "powershell", "pwsh":
		return "powershell", map[string]struct{}{"-c": {}, "-command": {}}, true
	default:
		return "", nil, false
	}
}

func nextMatchingShellWord(
	parts []shellCommandPart,
	index int,
	toolName string,
	values map[string]struct{},
) int {
	for cursor := index + 1; cursor < len(parts); cursor++ {
		if parts[cursor].operator != 0 {
			return -1
		}
		value := strings.ToLower(parts[cursor].value)
		if _, ok := values[value]; ok ||
			(toolName == "bash" && len(value) > 2 && strings.HasPrefix(value, "-") &&
				!strings.HasPrefix(value, "--") && strings.HasSuffix(value, "c")) {
			return cursor
		}
	}
	return -1
}

func nextShellWordIndex(parts []shellCommandPart, index int) int {
	if index < 0 || index+1 >= len(parts) || parts[index+1].operator != 0 {
		return -1
	}
	return index + 1
}

func joinShellWords(parts []shellCommandPart, index int) string {
	words := make([]string, 0)
	for ; index < len(parts) && parts[index].operator == 0; index++ {
		words = append(words, parts[index].value)
	}
	return strings.Join(words, " ")
}

// shellRedirectionPaths 从同一份词法结果读取写目标，避免把引号中的 `>`
// 当作重定向，也覆盖带空格目标、追加与 noclobber 覆盖形式。
func shellRedirectionPaths(command string, syntax shellSyntax) []string {
	parts := shellCommandParts(command, syntax)
	paths := make([]string, 0)
	for index, part := range parts {
		if part.operator != '>' ||
			(index > 0 && parts[index-1].operator == '>') {
			continue
		}
		targetIndex := index + 1
		for targetIndex < len(parts) && parts[targetIndex].operator == '>' {
			targetIndex++
		}
		if targetIndex < len(parts) && parts[targetIndex].operator == '|' {
			targetIndex++
		}
		if targetIndex >= len(parts) {
			continue
		}
		if parts[targetIndex].operator == '&' {
			targetIndex++
			if targetIndex >= len(parts) || parts[targetIndex].operator != 0 ||
				parts[targetIndex].value == "-" || digitsOnly(parts[targetIndex].value) {
				continue
			}
		}
		if parts[targetIndex].operator == 0 {
			paths = append(paths, parts[targetIndex].value)
		}
	}
	return paths
}

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func shellCommandParts(command string, syntax shellSyntax) []shellCommandPart {
	parts := make([]shellCommandPart, 0)
	var current strings.Builder
	var quote rune
	quoted := false
	unquoted := false
	flush := func() {
		if current.Len() == 0 {
			quoted = false
			unquoted = false
			return
		}
		parts = append(parts, shellCommandPart{
			value:    current.String(),
			quoted:   quoted,
			unquoted: unquoted,
		})
		current.Reset()
		quoted = false
		unquoted = false
	}
	characters := []rune(command)
	for index := 0; index < len(characters); index++ {
		character := characters[index]
		if quote != 0 {
			if index+1 < len(characters) &&
				quotedShellEscapesNext(syntax, quote, character, characters[index+1]) {
				current.WriteRune(characters[index+1])
				index++
				continue
			}
			if character == quote {
				quote = 0
				continue
			}
			current.WriteRune(character)
			continue
		}
		if index+1 < len(characters) && shellEscapesNext(syntax, character, characters[index+1]) {
			current.WriteRune(characters[index+1])
			unquoted = true
			index++
			continue
		}
		if character == '$' && index+1 < len(characters) && characters[index+1] == '{' {
			current.WriteRune(character)
			current.WriteRune(characters[index+1])
			unquoted = true
			index++
			depth := 1
			for index+1 < len(characters) && depth > 0 {
				index++
				character = characters[index]
				current.WriteRune(character)
				switch character {
				case '{':
					depth++
				case '}':
					depth--
				}
			}
			continue
		}
		switch {
		case character == '\'' || character == '"':
			quote = character
			quoted = true
		case character == '\r' || character == '\n':
			flush()
			parts = append(parts, shellCommandPart{operator: ';'})
		case unicode.IsSpace(character):
			flush()
		case strings.ContainsRune(";|&()<>{}", character):
			flush()
			parts = append(parts, shellCommandPart{operator: character})
		default:
			current.WriteRune(character)
			unquoted = true
		}
	}
	flush()
	return parts
}

func shellEscapesNext(syntax shellSyntax, character rune, next rune) bool {
	if (syntax.backtickEscape && character == '`') ||
		(syntax.caretEscape && character == '^') {
		return true
	}
	return syntax.backslashEscape && character == '\\' &&
		(unicode.IsSpace(next) || strings.ContainsRune("`;'\"|&()<>{}", next))
}

func quotedShellEscapesNext(syntax shellSyntax, quote rune, character rune, next rune) bool {
	if syntax.backtickEscape && quote == '"' && character == '`' {
		return true
	}
	if syntax.caretEscape && character == '^' {
		return true
	}
	if syntax.backtickEscape && quote == '\'' && character == '\'' && next == '\'' {
		return true
	}
	return syntax.backslashEscape && quote == '"' && character == '\\' &&
		strings.ContainsRune("$`\"\\\r\n", next)
}

func shellNumericEscapeArgument(parts []shellCommandPart, index int) bool {
	part := parts[index]
	if !part.quoted || part.unquoted || len(part.value) != 2 || part.value[0] != '\\' ||
		part.value[1] < '0' || part.value[1] > '9' {
		return false
	}
	return shellCommandName(parts, index) == "tr"
}

func windowsSlashOption(parts []shellCommandPart, index int, windowsSlashRoot bool) bool {
	value := parts[index].value
	if !windowsSlashRoot || !isWindowsSlashRootRelativeShellPath(value) ||
		strings.ContainsAny(value[1:], `/\`) {
		return false
	}
	options := windowsSlashOptions[shellCommandName(parts, index)]
	if len(options) == 0 {
		return false
	}
	name, _, _ := strings.Cut(strings.ToLower(value), ":")
	_, ok := options[name]
	return ok
}

func shellCommandName(parts []shellCommandPart, index int) string {
	start := 0
	for cursor := index - 1; cursor >= 0; cursor-- {
		if strings.ContainsRune(";|&()", parts[cursor].operator) {
			start = cursor + 1
			break
		}
	}
	for cursor := start; cursor < index; cursor++ {
		candidate := parts[cursor]
		if candidate.operator != 0 || strings.Contains(candidate.value, "=") {
			continue
		}
		return strings.ToLower(filepath.Base(candidate.value))
	}
	return ""
}

// complexBracedShellPath 只拒绝参与路径或显式携带父目录跳转的复杂展开。
// 普通 `echo ${NAME:-default}` 不属于路径访问，保留给 shell 处理。
func complexBracedShellPath(command string) string {
	for _, token := range bracedUnixShellTokenPattern.FindAllString(command, -1) {
		for _, expansion := range unixShellVariablePattern.FindAllString(token, -1) {
			if simpleBracedUnixShellVariablePattern.MatchString(expansion) {
				continue
			}
			if bracedExpansionRemoteURL(token, expansion) {
				continue
			}
			outside := strings.Replace(token, expansion, "", 1)
			if strings.ContainsAny(outside, `/\`) || strings.Contains(expansion, "..") ||
				windowsDriveExpansionPattern.MatchString(expansion) ||
				rootedExpansionPattern.MatchString(expansion) ||
				shellFileURLExpansionPattern.MatchString(expansion) ||
				bracedReplacementRooted(expansion) {
				return token
			}
		}
	}
	return ""
}

func bracedExpansionRemoteURL(token string, expansion string) bool {
	index := strings.Index(token, expansion)
	if index < 0 {
		return false
	}
	prefix := token[:index]
	if prefix != "" && (!strings.HasSuffix(prefix, "=") || strings.ContainsAny(prefix, `/\\`)) {
		return false
	}
	match := shellRemoteURLExpansionPattern.FindStringSubmatch(expansion)
	return len(match) == 2 && !strings.EqualFold(match[1], "file")
}

func bracedShellRemoteURLValue(value string) bool {
	expansion := unixShellVariablePattern.FindString(value)
	return expansion != "" && bracedExpansionRemoteURL(value, expansion)
}

func bracedReplacementRooted(expansion string) bool {
	body := strings.TrimSuffix(strings.TrimPrefix(expansion, "${"), "}")
	operatorIndex := strings.IndexRune(body, '/')
	if operatorIndex < 0 {
		return false
	}
	patternStart := operatorIndex + 1
	if patternStart < len(body) && body[patternStart] == '/' {
		patternStart++
	}
	replacementOffset := strings.IndexRune(body[patternStart:], '/')
	if replacementOffset < 0 {
		return false
	}
	replacement := body[patternStart+replacementOffset+1:]
	return strings.HasPrefix(replacement, "/") || strings.HasPrefix(replacement, `\`) ||
		strings.HasPrefix(replacement, "~/") || strings.HasPrefix(replacement, `~\`) ||
		isWindowsAbsoluteShellPath(replacement) || isWindowsDriveRelativeShellPath(replacement)
}

func standaloneComplexBracedShellValue(value string) bool {
	if _, assigned, ok := strings.Cut(value, "="); ok {
		value = assigned
	}
	expansion := unixShellVariablePattern.FindString(value)
	return expansion == value && !simpleBracedUnixShellVariablePattern.MatchString(expansion)
}

func mainAgentDynamicPathArgument(parts []shellCommandPart, index int) bool {
	part := parts[index]
	if firstShellVariableIndex(part.value) < 0 || nexusControlCommandVariable(part.value) {
		return false
	}
	if name, _, assigned := strings.Cut(part.value, "="); assigned && shellAssignmentName(name) {
		return false
	}
	commandName := shellCommandName(parts, index)
	if commandName == "" {
		return true
	}
	_, ok := shellPathArgumentCommands[commandName]
	return ok
}

func shellAssignmentName(value string) bool {
	if value == "" || !((value[0] >= 'a' && value[0] <= 'z') ||
		(value[0] >= 'A' && value[0] <= 'Z') || value[0] == '_') {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_') {
			return false
		}
	}
	return true
}

// shellDynamicPathPrefixAuthorized 只判断变量前已经明确出现的静态目录。
// 变量展开后的最终目标仍由 launcher/Landlock 的系统调用边界裁决。
func shellDynamicPathPrefixAuthorized(
	policy Policy,
	cwd string,
	path string,
	write bool,
	windowsSlashRoot bool,
) bool {
	variableIndex := firstShellVariableIndex(path)
	if variableIndex <= 0 || explicitShellTraversal(path) {
		return false
	}
	prefix := strings.TrimRight(path[:variableIndex], `/\`)
	if prefix == "" {
		return false
	}
	resolved, err := resolveShellToolPath(cwd, prefix, windowsSlashRoot)
	if err != nil {
		return false
	}
	if !write && shellSystemPath(resolved) {
		return true
	}
	_, err = policy.authorize(resolved, write)
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

func forbiddenNexusctlScope(policy Policy, command string, syntax shellSyntax) string {
	parts := shellCommandParts(command, syntax)
	usesNexusctl := shellCommandUsesNexusctl(parts) ||
		commandReferencesBracedNexusctlVariable(command) ||
		nexusctlCommandTextPattern.MatchString(command)
	usesNexuscfg := shellCommandUsesNexuscfg(parts) ||
		commandReferencesBracedNexuscfgVariable(command) ||
		nexuscfgCommandTextPattern.MatchString(command)
	if !usesNexusctl && !usesNexuscfg {
		return ""
	}
	if usesNexusctl && !policy.IsMainAgent {
		return "runtime 不向普通 Agent 提供 nexusctl"
	}
	if shellCommandOverridesNexusctlScope(parts, command) {
		return "Nexus 控制面 CLI 必须使用宿主注入的作用域和 capability"
	}
	return ""
}

func shellCommandUsesNexusctl(parts []shellCommandPart) bool {
	for _, part := range parts {
		if part.operator != 0 {
			continue
		}
		for _, candidate := range shellTokenCandidates(part) {
			if _, value, assigned := strings.Cut(candidate, "="); assigned {
				candidate = value
			}
			if nexusctlCommandVariable(candidate) || nexusctlExecutable(candidate) {
				return true
			}
		}
	}
	return false
}

func shellCommandUsesNexuscfg(parts []shellCommandPart) bool {
	for _, part := range parts {
		if part.operator != 0 {
			continue
		}
		for _, candidate := range shellTokenCandidates(part) {
			if _, value, assigned := strings.Cut(candidate, "="); assigned {
				candidate = value
			}
			if nexuscfgCommandVariable(candidate) || nexuscfgExecutable(candidate) {
				return true
			}
		}
	}
	return false
}

func shellCommandOverridesNexusctlScope(parts []shellCommandPart, _ string) bool {
	for index, part := range parts {
		if part.operator != 0 {
			continue
		}
		for _, candidate := range shellTokenCandidates(part) {
			name, value, assigned := strings.Cut(candidate, "=")
			if _, ok := nexusctlScopeFlags[strings.ToLower(name)]; ok {
				return true
			}
			if assigned {
				if _, ok := nexusctlScopeFlags[strings.ToLower(value)]; ok {
					return true
				}
			}
			if assigned &&
				(nexusctlScopeEnvironment(name) || powerShellScopeEnvironment(name)) &&
				shellEnvironmentAssignmentPosition(parts, index) {
				return true
			}
			if powerShellScopeEnvironment(name) &&
				nextShellWordIsEquals(parts, index) &&
				shellEnvironmentAssignmentPosition(parts, index) {
				return true
			}
			if powerShellScopeEnvironment(name) && powerShellEnvironmentMutation(parts, index) {
				return true
			}
		}
	}
	return false
}

// shellEnvironmentAssignmentPosition distinguishes syntax that actually
// mutates a command environment from an ordinary NAME=value argument printed
// or inspected by another command. Prefix assignments and `env NAME=value`
// are authoritative; `echo NAME=value` is only text.
func shellEnvironmentAssignmentPosition(parts []shellCommandPart, index int) bool {
	start := 0
	for cursor := index - 1; cursor >= 0; cursor-- {
		if strings.ContainsRune(";|&()", parts[cursor].operator) {
			start = cursor + 1
			break
		}
	}
	envCommand := false
	for cursor := start; cursor < index; cursor++ {
		part := parts[cursor]
		if part.operator != 0 {
			return false
		}
		name, _, assigned := strings.Cut(part.value, "=")
		if assigned && shellAssignmentName(name) && !part.quoted {
			continue
		}
		if !envCommand && strings.EqualFold(filepath.Base(part.value), "env") {
			envCommand = true
			continue
		}
		if envCommand && strings.HasPrefix(part.value, "-") {
			continue
		}
		if envCommand && assigned && shellAssignmentName(name) {
			continue
		}
		return false
	}
	return envCommand || !parts[index].quoted
}

func commandReferencesBracedNexusctlVariable(command string) bool {
	upper := strings.ToUpper(command)
	return strings.Contains(upper, "${NEXUSCTL_COMMAND_PATH}") ||
		strings.Contains(upper, "${ENV:NEXUSCTL_COMMAND_PATH}")
}

func commandReferencesBracedNexuscfgVariable(command string) bool {
	upper := strings.ToUpper(command)
	return strings.Contains(upper, "${NEXUSCFG_COMMAND_PATH}") ||
		strings.Contains(upper, "${ENV:NEXUSCFG_COMMAND_PATH}")
}

func shellTokenCandidates(part shellCommandPart) []string {
	candidates := []string{part.value}
	unescaped := shellEscapeReplacer.Replace(part.value)
	candidates = append(candidates, unescaped)
	if part.quoted {
		candidates = append(candidates, strings.ReplaceAll(unescaped, "$", ""))
	}
	return slices.Compact(candidates)
}

func nexusctlCommandVariable(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "$NEXUSCTL_COMMAND_PATH",
		"${NEXUSCTL_COMMAND_PATH}",
		"%NEXUSCTL_COMMAND_PATH%",
		"$ENV:NEXUSCTL_COMMAND_PATH",
		"${ENV:NEXUSCTL_COMMAND_PATH}":
		return true
	default:
		return false
	}
}

func nexuscfgCommandVariable(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "$NEXUSCFG_COMMAND_PATH",
		"${NEXUSCFG_COMMAND_PATH}",
		"%NEXUSCFG_COMMAND_PATH%",
		"$ENV:NEXUSCFG_COMMAND_PATH",
		"${ENV:NEXUSCFG_COMMAND_PATH}":
		return true
	default:
		return false
	}
}

func nexusControlCommandVariable(value string) bool {
	return nexusctlCommandVariable(value) || nexuscfgCommandVariable(value)
}

func nexusctlExecutable(value string) bool {
	if index := strings.LastIndexAny(value, `/\`); index >= 0 {
		value = value[index+1:]
	}
	_, ok := nexusctlExecutableNames[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func nexuscfgExecutable(value string) bool {
	if index := strings.LastIndexAny(value, `/\`); index >= 0 {
		value = value[index+1:]
	}
	_, ok := nexuscfgExecutableNames[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

func nexusctlScopeEnvironment(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	_, ok := nexusctlScopeEnvironmentNames[value]
	return ok
}

func powerShellScopeEnvironment(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(value, "ENV:"):
		return nexusctlScopeEnvironment(strings.TrimPrefix(value, "ENV:"))
	case strings.HasPrefix(value, "$ENV:"):
		return nexusctlScopeEnvironment(strings.TrimPrefix(value, "$ENV:"))
	case strings.HasPrefix(value, "${ENV:") && strings.HasSuffix(value, "}"):
		return nexusctlScopeEnvironment(strings.TrimSuffix(strings.TrimPrefix(value, "${ENV:"), "}"))
	default:
		return false
	}
}

func powerShellEnvironmentMutation(parts []shellCommandPart, index int) bool {
	_, ok := powerShellEnvironmentMutationCommands[shellCommandName(parts, index)]
	return ok
}

func nextShellWordIsEquals(parts []shellCommandPart, index int) bool {
	for index++; index < len(parts); index++ {
		if parts[index].operator != 0 {
			return false
		}
		return parts[index].value == "="
	}
	return false
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

func resolveShellToolPath(cwd string, raw string, windowsSlashRoot bool) (string, error) {
	raw = strings.TrimSpace(trimPowerShellFileSystemProvider(strings.TrimSpace(raw)))
	if path, ok, err := localFileURLPath(raw); ok || err != nil {
		if err != nil {
			return "", err
		}
		raw = strings.TrimSpace(path)
	}
	if windowsSlashRoot && isWindowsSlashRootRelativeShellPath(raw) {
		return "", fmt.Errorf("工具路径包含 Windows 当前盘根路径")
	}
	return resolveToolPath(cwd, raw)
}

func resolveToolPath(cwd string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	value = trimPowerShellFileSystemProvider(value)
	if value == "" {
		return "", fmt.Errorf("工具路径为空")
	}
	if strings.HasPrefix(value, "~") {
		return "", fmt.Errorf("工具路径包含未展开的 home 简写")
	}
	if isWindowsDriveRelativeShellPath(value) {
		return "", fmt.Errorf("工具路径包含 Windows 驱动器相对路径")
	}
	if runtime.GOOS == "windows" && isWindowsRootRelativeShellPath(value) {
		return "", fmt.Errorf("工具路径包含 Windows 当前盘根路径")
	}
	if runtime.GOOS != "windows" && isWindowsAbsoluteShellPath(value) {
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

func shellTokenPath(token string, windowsSlashRoot bool) (string, bool) {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	// 单独的 `$` 是 shell 语法，不是路径；真正带路径语义的变量仍会
	// 在下面保留并交给未展开变量检查。
	if token == "" || token == "$" || shellRemoteURL(token) {
		return "", false
	}
	token = trimPowerShellFileSystemProvider(token)
	if _, fileURL, err := localFileURLPath(token); fileURL || err != nil {
		return token, true
	}
	if shellRemoteURL(token) {
		return "", false
	}
	if isWindowsAbsoluteShellPath(token) || isWindowsDriveRelativeShellPath(token) ||
		(windowsSlashRoot && isWindowsSlashRootRelativeShellPath(token)) {
		return token, true
	}
	if filepath.IsAbs(token) || strings.HasPrefix(token, "~") ||
		explicitShellTraversal(token) ||
		((unixShellVariablePattern.MatchString(token) || windowsShellVariablePattern.MatchString(token)) &&
			strings.ContainsAny(token, `/\\`)) {
		return token, true
	}
	if value, ok := shellColonOptionValue(token); ok {
		return shellTokenPath(value, windowsSlashRoot)
	}
	if _, value, ok := strings.Cut(token, "="); ok {
		return shellTokenPath(value, windowsSlashRoot)
	}
	token = strings.Trim(token, ",:")
	switch {
	case filepath.IsAbs(token):
		return token, true
	case strings.HasPrefix(token, "~"):
		// shell 会在执行前把 ~ 展开到 home；宿主 hook 无法安全推断
		// 目标用户，因此宁可拒绝未展开的 home 简写，避免绕过 owner 根。
		return token, true
	case unixShellVariablePattern.MatchString(token) || windowsShellVariablePattern.MatchString(token):
		// 裸变量不携带可静态判断的路径语义，交给系统调用级隔离；
		// 带目录分隔符的变量路径继续检查其静态前缀。
		if strings.ContainsAny(token, `/\`) {
			return token, true
		}
		return "", false
	case runtime.GOOS == "windows" && isWindowsRootRelativeShellPath(token),
		explicitShellTraversal(token):
		return token, true
	default:
		return "", false
	}
}

func shellColonOptionValue(value string) (string, bool) {
	if len(value) < 4 || value[0] != '-' {
		return "", false
	}
	separator := strings.IndexRune(value[1:], ':')
	if separator < 1 {
		return "", false
	}
	separator++
	for _, character := range value[1:separator] {
		if !unicode.IsLetter(character) && character != '-' {
			return "", false
		}
	}
	return value[separator+1:], true
}

func shellRemoteURL(value string) bool {
	match := shellRemoteURLPattern.FindStringSubmatch(strings.TrimSpace(value))
	return len(match) == 2 && !strings.EqualFold(match[1], "file")
}

func localFileURLPath(value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "file:") {
		return "", false, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", true, fmt.Errorf("解析 file URL: %w", err)
	}
	path, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", true, fmt.Errorf("解析 file URL 路径: %w", err)
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' &&
		((path[1] >= 'a' && path[1] <= 'z') || (path[1] >= 'A' && path[1] <= 'Z')) &&
		path[2] == ':' {
		path = path[1:]
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		path = `\\` + parsed.Host + `/` + strings.TrimLeft(path, "/")
	}
	return filepath.FromSlash(path), true, nil
}

func trimPowerShellFileSystemProvider(value string) string {
	for _, prefix := range []string{
		"FileSystem::",
		`Microsoft.PowerShell.Core\FileSystem::`,
	} {
		if len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
			return value[len(prefix):]
		}
	}
	return value
}

func isWindowsDriveRelativeShellPath(value string) bool {
	return len(value) >= 2 &&
		((value[0] >= 'a' && value[0] <= 'z') ||
			(value[0] >= 'A' && value[0] <= 'Z')) &&
		value[1] == ':' &&
		(len(value) == 2 || (value[2] != '\\' && value[2] != '/'))
}

func isWindowsRootRelativeShellPath(value string) bool {
	if !strings.HasPrefix(value, `\`) || strings.HasPrefix(value, `\\`) {
		return false
	}
	return true
}

func isWindowsSlashRootRelativeShellPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")
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

func denyWorkspacePolicyOutput(terminal bool, publicReason string) sdkhook.Output {
	publicReason = strings.TrimSpace(publicReason)
	if publicReason == "" {
		publicReason = workspacePolicyPublicDenial
	}
	output := sdkhook.Output{
		SpecificOutput: &sdkhook.SpecificOutput{
			HookEventName:            sdkhook.EventPreToolUse,
			PermissionDecision:       sdkpermission.BehaviorDeny,
			PermissionDecisionReason: publicReason,
		},
	}
	if terminal {
		shouldContinue := false
		output.Continue = &shouldContinue
		output.StopReason = publicReason
	}
	return output
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
