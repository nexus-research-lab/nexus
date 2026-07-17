// INPUT: SDK PreToolUse 的 Bash 输入、runtime 会话键与 Agent 工作区。
// OUTPUT: 仅在舞台在线时生效的无副作用打开命令，以及可供前端还原的内部元数据。
// POS: operation service 的宿主打开隔离层；不允许舞台模式把 open 继续泄漏到用户本机。
package operation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

const stageOpenRedirectMarker = "__NEXUS_STAGE_OPEN_V1__"

type stageOpenRedirectPayload struct {
	Command string `json:"command"`
	Target  string `json:"target"`
}

// WithStageOpenRoutingHook 为 Bash 注册舞台感知的打开命令路由。
func (s *Service) WithStageOpenRoutingHook(
	options agentclient.Options,
	sessionKey string,
	workspacePath string,
) agentclient.Options {
	if strings.TrimSpace(sessionKey) == "" {
		return options
	}
	return options.AddHookMatcher(sdkhook.EventPreToolUse, sdkhook.Matcher{
		Matcher: "Bash",
		Hooks: []sdkhook.Callback{
			s.stageOpenRoutingHook(sessionKey, workspacePath),
		},
		Timeout: 2 * time.Second,
	})
}

func (s *Service) stageOpenRoutingHook(sessionKey string, workspacePath string) sdkhook.Callback {
	return func(_ context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.EventName != "" && input.EventName != sdkhook.EventPreToolUse {
			return stageOpenNoopOutput(), nil
		}
		if input.ToolName != "" && !strings.EqualFold(strings.TrimSpace(input.ToolName), "Bash") {
			return stageOpenNoopOutput(), nil
		}
		if !s.IsStageActive(sessionKey) {
			return stageOpenNoopOutput(), nil
		}

		toolInput, command, commandKey := readStageOpenToolInput(input.ToolInput)
		target, ok := parseStageOpenCommand(command, input.CWD, workspacePath)
		if !ok {
			return stageOpenNoopOutput(), nil
		}
		redirectedCommand, err := encodeStageOpenRedirectCommand(stageOpenRedirectPayload{
			Command: command,
			Target:  target,
		})
		if err != nil {
			return sdkhook.Output{}, err
		}
		updatedInput := cloneStageToolInput(toolInput)
		updatedInput[commandKey] = redirectedCommand
		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName: sdkhook.EventPreToolUse,
				UpdatedInput:  updatedInput,
			},
		}, nil
	}
}

func stageOpenNoopOutput() sdkhook.Output {
	continueValue := true
	return sdkhook.Output{Continue: &continueValue}
}

func readStageOpenToolInput(value any) (map[string]any, string, string) {
	input, ok := value.(map[string]any)
	if !ok {
		return nil, "", "command"
	}
	for _, key := range []string{"command", "cmd"} {
		if command, ok := input[key].(string); ok && strings.TrimSpace(command) != "" {
			return input, command, key
		}
	}
	return input, "", "command"
}

func cloneStageToolInput(input map[string]any) map[string]any {
	output := make(map[string]any, len(input)+1)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func encodeStageOpenRedirectCommand(payload stageOpenRedirectPayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	marker := base64.RawURLEncoding.EncodeToString(encoded)
	return ": # " + stageOpenRedirectMarker + marker, nil
}

func decodeStageOpenRedirectCommand(command string) (stageOpenRedirectPayload, bool) {
	index := strings.Index(command, stageOpenRedirectMarker)
	if index < 0 {
		return stageOpenRedirectPayload{}, false
	}
	encoded := strings.TrimSpace(command[index+len(stageOpenRedirectMarker):])
	if encoded == "" {
		return stageOpenRedirectPayload{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return stageOpenRedirectPayload{}, false
	}
	var payload stageOpenRedirectPayload
	if json.Unmarshal(decoded, &payload) != nil || strings.TrimSpace(payload.Command) == "" || strings.TrimSpace(payload.Target) == "" {
		return stageOpenRedirectPayload{}, false
	}
	return payload, true
}

func parseStageOpenCommand(command string, cwd string, workspacePath string) (string, bool) {
	tokens, ok := splitSimpleShellCommand(command)
	if !ok || len(tokens) < 2 {
		return "", false
	}
	tool := strings.ToLower(filepath.Base(tokens[0]))
	if tool != "open" && tool != "xdg-open" && tool != "start" {
		return "", false
	}

	target := stageOpenTargetToken(tool, tokens[1:])
	if target == "" {
		return "", false
	}
	target = resolveStageOpenTarget(target, cwd, workspacePath)
	if !isStagePreviewTarget(target) {
		return "", false
	}
	return target, true
}

func stageOpenTargetToken(tool string, args []string) string {
	var target string
	skipNext := false
	optionsEnded := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if !optionsEnded && arg == "--" {
			optionsEnded = true
			continue
		}
		if !optionsEnded && strings.HasPrefix(arg, "-") && arg != "-" {
			if tool == "open" && (arg == "-a" || arg == "-b" || arg == "--app" || arg == "--bundle-id") {
				skipNext = true
			}
			continue
		}
		if strings.TrimSpace(arg) != "" {
			target = arg
		}
	}
	return strings.TrimSpace(target)
}

func resolveStageOpenTarget(target string, cwd string, workspacePath string) string {
	target = strings.TrimSpace(strings.TrimPrefix(target, "file://"))
	if looksLikeStageWebURL(target) {
		return target
	}
	normalizedCWD := strings.TrimSpace(cwd)
	target = strings.ReplaceAll(target, "$(pwd)", normalizedCWD)
	target = strings.ReplaceAll(target, "`pwd`", normalizedCWD)
	target = strings.ReplaceAll(target, "${PWD}", normalizedCWD)
	target = strings.ReplaceAll(target, "$PWD", normalizedCWD)
	if strings.HasPrefix(target, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			target = filepath.Join(home, strings.TrimPrefix(target, "~/"))
		}
	}
	if !filepath.IsAbs(target) && normalizedCWD != "" {
		target = filepath.Join(normalizedCWD, target)
	}
	target = filepath.Clean(target)
	root := strings.TrimSpace(workspacePath)
	if root == "" {
		return filepath.ToSlash(target)
	}
	relative, err := filepath.Rel(root, target)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	if err == nil && relative == "." {
		return "."
	}
	return filepath.ToSlash(target)
}

func isStagePreviewTarget(target string) bool {
	if looksLikeStageWebURL(target) {
		return true
	}
	extension := strings.ToLower(filepath.Ext(strings.SplitN(target, "?", 2)[0]))
	switch extension {
	case ".html", ".htm", ".xhtml",
		".md", ".mdx", ".markdown", ".txt", ".log",
		".json", ".yaml", ".yml", ".toml", ".xml",
		".c", ".cc", ".cpp", ".h", ".hpp", ".go", ".rs", ".py",
		".js", ".jsx", ".ts", ".tsx", ".css", ".scss", ".less",
		".java", ".kt", ".swift", ".sh", ".bash", ".zsh", ".sql",
		".vue", ".svelte", ".graphql",
		".ipynb", ".ps1", ".lua", ".dart", ".cs", ".fs", ".fsx",
		".doc", ".docx", ".rtf", ".odt", ".pdf",
		".csv", ".tsv", ".xls", ".xlsx", ".ods", ".ppt", ".pptx", ".odp",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico", ".avif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func looksLikeStageWebURL(target string) bool {
	normalized := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(normalized, "https://") || strings.HasPrefix(normalized, "http://")
}

func splitSimpleShellCommand(command string) ([]string, bool) {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 64*1024 {
		return nil, false
	}
	var tokens []string
	var token strings.Builder
	var quote rune
	escaped := false
	started := false
	flush := func() {
		if started {
			tokens = append(tokens, token.String())
			token.Reset()
			started = false
		}
	}
	for _, char := range command {
		if escaped {
			token.WriteRune(char)
			started = true
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
				started = true
				continue
			}
			token.WriteRune(char)
			started = true
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			started = true
			continue
		}
		if char == ';' || char == '&' || char == '|' || char == '\n' || char == '\r' {
			return nil, false
		}
		if char == ' ' || char == '\t' {
			flush()
			continue
		}
		token.WriteRune(char)
		started = true
	}
	if escaped || quote != 0 {
		return nil, false
	}
	flush()
	return tokens, len(tokens) > 0
}
