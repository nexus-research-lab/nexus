package server

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
)

func TestAppendAmapMCPServerUsesOfficialHTTPConfig(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"amap": {
				ConnectorID: "amap",
				AccessToken: "amap key/with+symbols",
			},
		},
	}

	appendAmapMCPServer(context.Background(), servers, svc, "owner-1")

	config, ok := servers["amap_maps"].(sdkmcp.HTTPServerConfig)
	if !ok {
		t.Fatalf("高德 MCP server 未注入 HTTP 配置: %+v", servers["amap_maps"])
	}
	if !strings.HasPrefix(config.URL, "https://mcp.amap.com/mcp?key=") {
		t.Fatalf("高德 MCP server 地址不正确: %s", config.URL)
	}
	if !strings.Contains(config.URL, url.QueryEscape("amap key/with+symbols")) {
		t.Fatalf("高德 API Key 未安全编码: %s", config.URL)
	}
}

func TestAppendDidiMCPServerUsesOfficialHTTPConfig(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"didi": {
				ConnectorID: "didi",
				AccessToken: "didi key/with+symbols",
			},
		},
	}

	appendDidiMCPServer(context.Background(), servers, svc, "owner-1")

	config, ok := servers["didi_ride"].(sdkmcp.HTTPServerConfig)
	if !ok {
		t.Fatalf("滴滴 MCP server 未注入 HTTP 配置: %+v", servers["didi_ride"])
	}
	if !strings.HasPrefix(config.URL, "https://mcp.didichuxing.com/mcp-servers?key=") {
		t.Fatalf("滴滴 MCP server 地址不正确: %s", config.URL)
	}
	if !strings.Contains(config.URL, url.QueryEscape("didi key/with+symbols")) {
		t.Fatalf("滴滴 MCP Key 未安全编码: %s", config.URL)
	}
}

func TestAppendDingTalkAITableMCPServerUsesUserHTTPURL(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"dingtalk-ai-table": {
				ConnectorID: "dingtalk-ai-table",
				AccessToken: "https://mcp.dingtalk.com/sse?token=secret",
			},
		},
	}

	appendDingTalkAITableMCPServer(context.Background(), servers, svc, "owner-1")

	config, ok := servers["dingtalk_ai_table"].(sdkmcp.HTTPServerConfig)
	if !ok {
		t.Fatalf("钉钉 AI 表格 MCP server 未注入 HTTP 配置: %+v", servers["dingtalk_ai_table"])
	}
	if config.URL != "https://mcp.dingtalk.com/sse?token=secret" {
		t.Fatalf("钉钉 AI 表格 MCP server 地址不正确: %s", config.URL)
	}
}

func TestAppendDingTalkAITableMCPServerSkipsInvalidURL(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"dingtalk-ai-table": {
				ConnectorID: "dingtalk-ai-table",
				AccessToken: "not-a-url",
			},
		},
	}

	appendDingTalkAITableMCPServer(context.Background(), servers, svc, "owner-1")

	if _, ok := servers["dingtalk_ai_table"]; ok {
		t.Fatalf("钉钉 AI 表格 MCP server URL 无效时不应注入: %+v", servers)
	}
}

func TestAppendTencentDocsMCPServerUsesAuthorizationHeader(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"tencent-docs": {
				ConnectorID: "tencent-docs",
				AccessToken: "tencent docs token",
			},
		},
	}

	appendTencentDocsMCPServer(context.Background(), servers, svc, "owner-1")

	config, ok := servers["tencent_docs"].(sdkmcp.HTTPServerConfig)
	if !ok {
		t.Fatalf("腾讯文档 MCP server 未注入 HTTP 配置: %+v", servers["tencent_docs"])
	}
	if config.URL != "https://docs.qq.com/openapi/mcp" {
		t.Fatalf("腾讯文档 MCP server 地址不正确: %s", config.URL)
	}
	if config.Headers["Authorization"] != "tencent docs token" {
		t.Fatalf("腾讯文档 Token 未写入 Authorization header: %+v", config.Headers)
	}
}

func TestAppendYuqueMCPServerUsesStdioConfig(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	svc := &stubConnectorMCPService{
		snapshots: map[string]*connectordomain.ConnectionSnapshot{
			"yuque": {
				ConnectorID: "yuque",
				AccessToken: "yuque token",
			},
		},
	}

	appendYuqueMCPServer(context.Background(), servers, svc, "owner-1")

	config, ok := servers["yuque"].(sdkmcp.StdioServerConfig)
	if !ok {
		t.Fatalf("语雀 MCP server 未注入 stdio 配置: %+v", servers["yuque"])
	}
	if config.Command != "npx" || strings.Join(config.Args, " ") != "-y yuque-mcp" {
		t.Fatalf("语雀 MCP server 启动命令不正确: %+v", config)
	}
	if config.Env["YUQUE_PERSONAL_TOKEN"] != "yuque token" {
		t.Fatalf("语雀 Personal Token 未写入环境变量: %+v", config.Env)
	}
}

func TestAppendAmapMCPServerSkipsMissingConnection(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"nexus_connectors": sdkmcp.HTTPServerConfig{URL: "http://localhost/internal"},
	}
	appendAmapMCPServer(context.Background(), servers, &stubConnectorMCPService{}, "owner-1")
	if _, ok := servers["amap_maps"]; ok {
		t.Fatalf("未连接高德时不应注入 amap_maps: %+v", servers)
	}

	appendAmapMCPServer(context.Background(), servers, &stubConnectorMCPService{err: errors.New("boom")}, "owner-1")
	if _, ok := servers["amap_maps"]; ok {
		t.Fatalf("读取高德连接失败时不应注入 amap_maps: %+v", servers)
	}
}

type stubConnectorMCPService struct {
	snapshots map[string]*connectordomain.ConnectionSnapshot
	err       error
}

func (s *stubConnectorMCPService) ListActiveConnections(
	context.Context,
	string,
) ([]connectordomain.ConnectionSnapshot, error) {
	return nil, nil
}

func (s *stubConnectorMCPService) LoadActiveConnection(
	_ context.Context,
	ownerUserID string,
	connectorID string,
) (*connectordomain.ConnectionSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	if ownerUserID != "owner-1" {
		return nil, nil
	}
	return s.snapshots[connectorID], nil
}
