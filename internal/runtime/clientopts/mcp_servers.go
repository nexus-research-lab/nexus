// INPUT: Nexus 内建 MCP servers 与 Agent 持久化的自由格式 MCP 配置。
// OUTPUT: 经严格校验、且不会覆盖内建名称的 SDK MCP server 集合。
// POS: Agent 配置进入 DM/Room runtime 前的统一 MCP 解析边界。
package clientopts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"sort"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

var persistedMCPServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var reservedAgentMCPServerNames = map[string]struct{}{
	"amap_maps":         {},
	"didi_ride":         {},
	"dingtalk_ai_table": {},
	"tencent_docs":      {},
	"yuque":             {},
}

type persistedStdioMCPServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type persistedRemoteMCPServer struct {
	Type          string                   `json:"type"`
	URL           string                   `json:"url"`
	Headers       map[string]string        `json:"headers,omitempty"`
	HeadersHelper string                   `json:"headersHelper,omitempty"`
	OAuth         *persistedMCPServerOAuth `json:"oauth,omitempty"`
}

type persistedMCPServerOAuth struct {
	ClientID              string `json:"clientId,omitempty"`
	CallbackPort          int    `json:"callbackPort,omitempty"`
	AuthServerMetadataURL string `json:"authServerMetadataUrl,omitempty"`
	XAA                   *bool  `json:"xaa,omitempty"`
}

// MergeAgentMCPServers 严格解析 Agent 配置并合并 Nexus 内建 MCP servers。
func MergeAgentMCPServers(
	builtIn map[string]sdkmcp.ServerConfig,
	configured map[string]any,
) (map[string]sdkmcp.ServerConfig, error) {
	if len(builtIn) == 0 && len(configured) == 0 {
		return nil, nil
	}
	merged := maps.Clone(builtIn)
	if merged == nil {
		merged = make(map[string]sdkmcp.ServerConfig, len(configured))
	}

	names := make([]string, 0, len(configured))
	for name := range configured {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := validateAgentMCPServerName(name, builtIn); err != nil {
			return nil, err
		}
		server, err := parseAgentMCPServer(name, configured[name])
		if err != nil {
			return nil, err
		}
		merged[name] = server
	}
	return merged, nil
}

func validateAgentMCPServerName(name string, builtIn map[string]sdkmcp.ServerConfig) error {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return agentMCPServerError(name, "名称不能为空")
	}
	if normalized != name {
		return agentMCPServerError(name, "名称首尾不能包含空白")
	}
	if !persistedMCPServerNamePattern.MatchString(normalized) {
		return agentMCPServerError(name, "名称只能使用 1-64 位字母、数字、点、下划线或连字符，且必须以字母或数字开头")
	}
	normalizedLower := strings.ToLower(normalized)
	if normalizedLower == "nexus" || strings.HasPrefix(normalizedLower, "nexus_") {
		return agentMCPServerError(name, "不能使用 Nexus 内建 MCP server 命名空间")
	}
	if _, exists := reservedAgentMCPServerNames[strings.ToLower(normalized)]; exists {
		return agentMCPServerError(name, "不能使用 Nexus Connector 管理的 MCP server 名称")
	}
	for builtInName := range builtIn {
		if strings.EqualFold(builtInName, normalized) {
			return agentMCPServerError(name, "不能覆盖 Nexus 管理的 MCP server")
		}
	}
	return nil
}

func parseAgentMCPServer(name string, raw any) (sdkmcp.ServerConfig, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, agentMCPServerError(name, fmt.Sprintf("配置必须是对象，实际为 %T", raw))
	}

	serverType, err := persistedMCPServerType(object)
	if err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	switch serverType {
	case "stdio":
		return parsePersistedStdioMCPServer(name, object)
	case "http", "sse":
		return parsePersistedRemoteMCPServer(name, serverType, object)
	default:
		return nil, agentMCPServerError(name, fmt.Sprintf("不支持 type=%q，仅支持 stdio、http、sse", serverType))
	}
}

func persistedMCPServerType(object map[string]any) (string, error) {
	rawType, hasType := object["type"]
	if !hasType {
		if _, hasCommand := object["command"]; hasCommand {
			return "stdio", nil
		}
		return "", fmt.Errorf("缺少 type；stdio 配置也可通过 command 推断")
	}
	serverType, ok := rawType.(string)
	if !ok {
		return "", fmt.Errorf("type 必须是字符串")
	}
	serverType = strings.TrimSpace(serverType)
	if serverType == "" {
		return "", fmt.Errorf("type 不能为空")
	}
	return serverType, nil
}

func parsePersistedStdioMCPServer(
	name string,
	object map[string]any,
) (sdkmcp.ServerConfig, error) {
	var input persistedStdioMCPServer
	if err := decodeStrictMCPServerObject(object, &input); err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	if input.Type != "" && strings.TrimSpace(input.Type) != "stdio" {
		return nil, agentMCPServerError(name, fmt.Sprintf("不支持 type=%q，仅支持 stdio、http、sse", input.Type))
	}
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return nil, agentMCPServerError(name, "stdio command 不能为空")
	}
	if err := validateNonEmptyMapKeys(input.Env, "env"); err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	return sdkmcp.StdioServerConfig{
		Command: command,
		Args:    input.Args,
		Env:     input.Env,
	}, nil
}

func parsePersistedRemoteMCPServer(
	name string,
	serverType string,
	object map[string]any,
) (sdkmcp.ServerConfig, error) {
	var input persistedRemoteMCPServer
	if err := decodeStrictMCPServerObject(object, &input); err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	if strings.TrimSpace(input.Type) != serverType {
		return nil, agentMCPServerError(name, fmt.Sprintf("type 必须是 %q", serverType))
	}
	serverURL, err := validateRemoteMCPServerURL(input.URL)
	if err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	if err = validateNonEmptyMapKeys(input.Headers, "headers"); err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	oauth, err := parsePersistedMCPServerOAuth(input.OAuth)
	if err != nil {
		return nil, agentMCPServerError(name, err.Error())
	}
	headersHelper := strings.TrimSpace(input.HeadersHelper)
	if serverType == "sse" {
		return sdkmcp.SSEServerConfig{
			URL:           serverURL,
			Headers:       input.Headers,
			HeadersHelper: headersHelper,
			OAuth:         oauth,
		}, nil
	}
	return sdkmcp.HTTPServerConfig{
		URL:           serverURL,
		Headers:       input.Headers,
		HeadersHelper: headersHelper,
		OAuth:         oauth,
	}, nil
}

func parsePersistedMCPServerOAuth(input *persistedMCPServerOAuth) (*sdkmcp.OAuthConfig, error) {
	if input == nil {
		return nil, nil
	}
	if input.CallbackPort < 0 || input.CallbackPort > 65535 {
		return nil, fmt.Errorf("oauth callbackPort 必须在 0 到 65535 之间")
	}
	metadataURL := strings.TrimSpace(input.AuthServerMetadataURL)
	if metadataURL != "" {
		parsed, err := url.Parse(metadataURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("oauth authServerMetadataUrl 必须是有效的 HTTPS URL")
		}
	}
	return &sdkmcp.OAuthConfig{
		ClientID:              strings.TrimSpace(input.ClientID),
		CallbackPort:          input.CallbackPort,
		AuthServerMetadataURL: metadataURL,
		XAA:                   input.XAA,
	}, nil
}

func validateRemoteMCPServerURL(raw string) (string, error) {
	serverURL := strings.TrimSpace(raw)
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("url 必须是有效的 HTTP 或 HTTPS URL")
	}
	return serverURL, nil
}

func validateNonEmptyMapKeys(values map[string]string, field string) error {
	for key := range values {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s 不能包含空键", field)
		}
	}
	return nil
}

func decodeStrictMCPServerObject(object map[string]any, target any) error {
	payload, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("编码配置: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fmt.Errorf("解析配置: %w", err)
	}
	return nil
}

func agentMCPServerError(name string, message string) error {
	return fmt.Errorf("Agent MCP server %q: %s", name, message)
}
