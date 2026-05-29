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
