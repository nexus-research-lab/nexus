package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

func TestBuildAllIncludesFeishuDocxTools(t *testing.T) {
	names := map[string]bool{}
	for _, item := range BuildAll(stubConnectorService{}, contract.ServerContext{OwnerUserID: "user-1"}) {
		names[item.Name] = true
	}
	for _, name := range []string{
		"feishu_docx_read",
		"feishu_docx_search",
		"feishu_docx_sheet_sheets",
		"feishu_docx_sheet_values",
		"feishu_docx_sheet_find",
		"feishu_docx_bitable_tables",
		"feishu_docx_bitable_fields",
		"feishu_docx_bitable_records",
		"feishu_docx_create",
		"feishu_docx_append_markdown",
		"feishu_docx_update_block",
		"feishu_docx_drive_list",
		"feishu_docx_wiki_spaces",
		"feishu_docx_wiki_space",
		"feishu_docx_wiki_nodes",
		"feishu_docx_wiki_node",
	} {
		if !names[name] {
			t.Fatalf("缺少飞书 MCP 工具: %s", name)
		}
	}
}

func TestFeishuDocxSearchTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/suite/docs-api/search/object" {
			http.NotFound(writer, request)
			return
		}
		if request.Method != http.MethodPost {
			t.Fatalf("搜索工具请求方法不正确: %s", request.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析搜索请求失败: %v", err)
		}
		if body["search_key"] != "操作文档" {
			t.Fatalf("搜索关键词不正确: %+v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "Success",
			"data": map[string]any{
				"docs_entities": []map[string]any{{"docs_token": "doc123", "docs_type": "docx", "title": "操作文档"}},
				"has_more":      false,
				"total":         1,
			},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_search", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"query": "操作文档", "docs_types": []any{"doc", "sheet"}})
	if result.IsError {
		t.Fatalf("飞书搜索工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	data := output["data"].(map[string]any)
	entities := data["docs_entities"].([]any)
	if len(entities) != 1 || entities[0].(map[string]any)["docs_token"] != "doc123" {
		t.Fatalf("搜索工具输出不正确: %+v", output)
	}
}

func TestFeishuDocxSheetValuesTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/sheets/v2/spreadsheets/sht123/values/Sheet1!A1:B2" {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Sheet 工具请求未带 token: %s", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"valueRange": map[string]any{
					"range":    "Sheet1!A1:B2",
					"revision": 3,
					"values": [][]any{
						{"标题", "状态"},
						{"Nexus", "OK"},
					},
				},
			},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_sheet_values", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"url": "sht123", "range": "Sheet1!A1:B2"})
	if result.IsError {
		t.Fatalf("飞书 Sheet 取值工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	values := output["values"].([]any)
	second := values[1].([]any)
	if second[0] != "Nexus" || second[1] != "OK" {
		t.Fatalf("Sheet 工具输出不正确: %+v", output)
	}
}

func TestFeishuDocxBitableRecordsTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/bitable/v1/apps/base123/tables/tbl123/records" {
			http.NotFound(writer, request)
			return
		}
		query := request.URL.Query()
		if query.Get("page_size") != "20" || query.Get("field_names") != `["标题"]` {
			t.Fatalf("Bitable 记录工具查询参数不正确: %s", request.URL.RawQuery)
		}
		if !strings.Contains(query.Get("filter"), "CurrentValue") {
			t.Fatalf("Bitable 记录工具 filter 不正确: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"items": []map[string]any{{
					"record_id": "rec123",
					"fields":    map[string]any{"标题": "Nexus"},
				}},
				"has_more":   false,
				"page_token": "",
				"total":      1,
			},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_bitable_records", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{
		"url":         "https://acme.feishu.cn/base/base123?table=tbl123",
		"field_names": []any{"标题"},
		"filter":      "CurrentValue.[标题]=\"Nexus\"",
		"page_size":   20,
	})
	if result.IsError {
		t.Fatalf("飞书 Bitable 记录工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	records := output["records"].([]any)
	first := records[0].(map[string]any)
	if first["record_id"] != "rec123" {
		t.Fatalf("Bitable 记录工具输出不正确: %+v", output)
	}
}

func TestFeishuDocxCreateDocumentTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/docx/v1/documents" {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("飞书工具请求未带 token: %s", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{"document": map[string]any{"document_id": "doc123", "title": "标题"}},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_create", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"title": "标题"})
	if result.IsError {
		t.Fatalf("飞书创建文档工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	if output["document_id"] != "doc123" {
		t.Fatalf("工具输出不正确: %+v", output)
	}
}

func TestFeishuDocxWikiNodesTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/wiki/v2/spaces/sp1/nodes" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("parent_node_token") != "PARENT" {
			t.Fatalf("知识库节点工具 parent_node_token 不正确: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"items": []map[string]any{{"space_id": "sp1", "node_token": "node1", "obj_token": "doc123", "obj_type": "docx", "title": "操作文档"}},
			},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_wiki_nodes", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"space_id": "sp1", "parent_node_token": "https://acme.feishu.cn/wiki/PARENT"})
	if result.IsError {
		t.Fatalf("飞书知识库节点工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	items := output["items"].([]any)
	first := items[0].(map[string]any)
	if first["node_token"] != "node1" || first["document_url"] == "" {
		t.Fatalf("知识库节点工具输出不正确: %+v", output)
	}
}

func TestFeishuDocxDriveListFiltersFileType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/drive/v1/files" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("option") != "all" {
			t.Fatalf("云空间列表 option 参数不正确: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"files": []map[string]any{
					{"name": "操作文档", "type": "docx", "token": "doc123"},
					{"name": "素材", "type": "file", "token": "file123"},
				},
			},
		})
	}))
	defer server.Close()

	result := callNamedTool(t, "feishu_docx_drive_list", stubConnectorService{item: &connectordomain.ConnectionSnapshot{
		ConnectorID: "feishu-docx",
		AuthType:    "oauth2",
		APIBaseURL:  server.URL,
		AccessToken: "token",
	}}, map[string]any{"option": "all", "file_type": "docx"})
	if result.IsError {
		t.Fatalf("飞书云空间列表工具失败: %+v", result)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &output); err != nil {
		t.Fatalf("解析工具输出失败: %v", err)
	}
	files := output["files"].([]any)
	if len(files) != 1 || files[0].(map[string]any)["type"] != "docx" {
		t.Fatalf("云空间列表过滤结果不正确: %+v", output)
	}
}

func callNamedTool(t *testing.T, name string, svc contract.Service, args map[string]any) sdkmcp.ToolResult {
	t.Helper()
	for _, item := range BuildAll(svc, contract.ServerContext{OwnerUserID: "user-1"}) {
		if item.Name == name {
			result, err := item.Handler(context.Background(), args)
			if err != nil {
				t.Fatalf("tool handler returned transport error: %v", err)
			}
			return result
		}
	}
	t.Fatalf("%s tool not found", name)
	return sdkmcp.ToolResult{}
}
