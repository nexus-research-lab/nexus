// INPUT: owner 级自定义 MCP 配置与远程 HTTP/SSE 连接。
// OUTPUT: MCP server 基础信息及其当前 tools/list 目录；stdio 保持 runtime-only。
// POS: 自定义 MCP 详情页的只读能力发现边界。
package connectors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const customMCPDiscoveryTimeout = 12 * time.Second

var ErrCustomMCPCapabilityUnavailable = errors.New("自定义 MCP 能力不可用")

// ErrConnectorMCPCapabilityUnavailable 表示固定 Connector 的 MCP 远程目录暂不可读。
var ErrConnectorMCPCapabilityUnavailable = errors.New("Connector MCP 能力不可用")

// CustomMCPToolArgument 是工具输入 schema 中可供用户识别的顶层参数。
type CustomMCPToolArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// CustomMCPTool 是详情页展示的最小工具定义。
type CustomMCPTool struct {
	Name        string                  `json:"name"`
	Title       string                  `json:"title"`
	Description string                  `json:"description,omitempty"`
	Arguments   []CustomMCPToolArgument `json:"arguments"`
	ReadOnly    bool                    `json:"read_only,omitempty"`
}

// CustomMCPToolCatalog 是一次有界远程工具发现结果。
type CustomMCPToolCatalog struct {
	InspectionState string          `json:"inspection_state"`
	ProtocolVersion string          `json:"protocol_version,omitempty"`
	ServerName      string          `json:"server_name,omitempty"`
	ServerTitle     string          `json:"server_title,omitempty"`
	ServerVersion   string          `json:"server_version,omitempty"`
	Instructions    string          `json:"instructions,omitempty"`
	SupportsTools   bool            `json:"supports_tools"`
	Tools           []CustomMCPTool `json:"tools"`
}

// DiscoverCustomMCPCapabilities 读取远程 MCP 的 server 信息与工具目录。
// stdio 只能由用户 runtime 身份启动，详情页不会以宿主身份执行其命令。
func (s *Service) DiscoverCustomMCPCapabilities(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*CustomMCPToolCatalog, error) {
	server, err := s.loadStoredCustomMCPServer(ctx, ownerUserID, connectorID)
	if err != nil {
		return nil, err
	}
	if !server.Enabled {
		return emptyCustomMCPToolCatalog("disabled"), nil
	}
	if server.Type == "stdio" {
		return emptyCustomMCPToolCatalog("runtime_only"), nil
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, customMCPDiscoveryTimeout)
	defer cancel()
	catalog, err := discoverRemoteCustomMCPTools(discoveryCtx, *server)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCustomMCPCapabilityUnavailable, err)
	}
	return catalog, nil
}

// DiscoverConnectorMCPCapabilities 读取支持详情探测的固定 Connector 工具目录。
// 当前只有 RichMail 通过固定 loopback URL 与已保存 Bearer Token 进入该边界。
func (s *Service) DiscoverConnectorMCPCapabilities(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*CustomMCPToolCatalog, error) {
	entry, err := requireRichMailConnector(connectorID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.LoadActiveConnection(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || strings.TrimSpace(snapshot.AccessToken) == "" {
		return nil, errors.New("RichMail 尚未连接")
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, customMCPDiscoveryTimeout)
	defer cancel()
	catalog, err := discoverRemoteMCPTools(discoveryCtx, storedCustomMCPServer{
		Type:        "http",
		URL:         s.richMailMCPURL,
		AuthType:    customMCPAuthBearer,
		BearerToken: snapshot.AccessToken,
	}, richMailLoopbackTransport())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectorMCPCapabilityUnavailable, err)
	}
	return catalog, nil
}

func discoverRemoteCustomMCPTools(
	ctx context.Context,
	server storedCustomMCPServer,
) (*CustomMCPToolCatalog, error) {
	return discoverRemoteMCPTools(ctx, server, http.DefaultTransport)
}

func discoverRemoteMCPTools(
	ctx context.Context,
	server storedCustomMCPServer,
	baseTransport http.RoundTripper,
) (*CustomMCPToolCatalog, error) {
	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: customMCPDiscoveryTimeout,
		Transport: customMCPHeaderTransport{
			base:    baseTransport,
			headers: server.runtimeHeaders(),
		},
	}
	var transport mcp.Transport
	switch server.Type {
	case "http":
		transport = &mcp.StreamableClientTransport{
			DisableStandaloneSSE: true,
			Endpoint:             server.URL,
			HTTPClient:           httpClient,
			MaxRetries:           -1,
		}
	case "sse":
		transport = &mcp.SSEClientTransport{
			Endpoint:   server.URL,
			HTTPClient: httpClient,
		}
	default:
		return nil, fmt.Errorf("不支持从详情页探测 %q transport", server.Type)
	}

	client := mcp.NewClient(
		&mcp.Implementation{Name: "nexus", Title: "Nexus", Version: "1"},
		&mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}},
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	result := emptyCustomMCPToolCatalog("connected")
	initialized := session.InitializeResult()
	if initialized == nil {
		return nil, errors.New("MCP server 未返回初始化信息")
	}
	result.ProtocolVersion = boundedCustomMCPText(initialized.ProtocolVersion, 128)
	result.Instructions = boundedCustomMCPText(initialized.Instructions, 4000)
	if initialized.ServerInfo != nil {
		result.ServerName = boundedCustomMCPText(initialized.ServerInfo.Name, 256)
		result.ServerTitle = boundedCustomMCPText(initialized.ServerInfo.Title, 256)
		result.ServerVersion = boundedCustomMCPText(initialized.ServerInfo.Version, 128)
	}
	result.SupportsTools = initialized.Capabilities != nil && initialized.Capabilities.Tools != nil
	if !result.SupportsTools {
		return result, nil
	}
	for tool, toolErr := range session.Tools(ctx, nil) {
		if toolErr != nil {
			return nil, toolErr
		}
		if tool == nil {
			continue
		}
		result.Tools = append(result.Tools, projectCustomMCPTool(tool))
		if len(result.Tools) >= 500 {
			break
		}
	}
	return result, nil
}

func emptyCustomMCPToolCatalog(state string) *CustomMCPToolCatalog {
	return &CustomMCPToolCatalog{
		InspectionState: state,
		Tools:           []CustomMCPTool{},
	}
}

func (server storedCustomMCPServer) runtimeHeaders() map[string]string {
	if server.AuthType == customMCPAuthBearer {
		return map[string]string{"Authorization": "Bearer " + server.BearerToken}
	}
	result := make(map[string]string, len(server.Headers))
	for key, value := range server.Headers {
		result[key] = value
	}
	return result
}

type customMCPHeaderTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (transport customMCPHeaderTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header = request.Header.Clone()
	for key, value := range transport.headers {
		cloned.Header.Set(key, value)
	}
	return transport.base.RoundTrip(cloned)
}

func projectCustomMCPTool(tool *mcp.Tool) CustomMCPTool {
	title := strings.TrimSpace(tool.Title)
	readOnly := false
	if tool.Annotations != nil {
		if title == "" {
			title = strings.TrimSpace(tool.Annotations.Title)
		}
		readOnly = tool.Annotations.ReadOnlyHint
	}
	if title == "" {
		title = tool.Name
	}
	return CustomMCPTool{
		Name:        boundedCustomMCPText(tool.Name, 256),
		Title:       boundedCustomMCPText(title, 256),
		Description: boundedCustomMCPText(tool.Description, 2000),
		Arguments:   customMCPToolArguments(tool.InputSchema),
		ReadOnly:    readOnly,
	}
}

func customMCPToolArguments(schema any) []CustomMCPToolArgument {
	root, ok := schema.(map[string]any)
	if !ok {
		return []CustomMCPToolArgument{}
	}
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		return []CustomMCPToolArgument{}
	}
	required := map[string]struct{}{}
	if values, ok := root["required"].([]any); ok {
		for _, value := range values {
			if name, stringOK := value.(string); stringOK {
				required[name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]CustomMCPToolArgument, 0, len(names))
	for _, name := range names {
		description := ""
		if definition, definitionOK := properties[name].(map[string]any); definitionOK {
			description, _ = definition["description"].(string)
		}
		_, isRequired := required[name]
		result = append(result, CustomMCPToolArgument{
			Name:        boundedCustomMCPText(name, 256),
			Description: boundedCustomMCPText(description, 1000),
			Required:    isRequired,
		})
	}
	return result
}

func boundedCustomMCPText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	characters := []rune(value)
	if len(characters) <= maximum {
		return value
	}
	return string(characters[:maximum])
}
