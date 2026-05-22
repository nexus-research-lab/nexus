package feishudocx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDocumentTargetSupportsDocxWikiAndID(t *testing.T) {
	docx, err := ParseDocumentTarget("https://acme.feishu.cn/docx/ABC123")
	if err != nil {
		t.Fatalf("解析 docx URL 失败: %v", err)
	}
	if docx.DocumentID != "ABC123" || docx.SourceType != "docx" {
		t.Fatalf("docx 解析结果不正确: %+v", docx)
	}

	wiki, err := ParseDocumentTarget("https://acme.feishu.cn/wiki/WIKI123")
	if err != nil {
		t.Fatalf("解析 wiki URL 失败: %v", err)
	}
	if wiki.WikiToken != "WIKI123" || wiki.SourceType != "wiki" {
		t.Fatalf("wiki 解析结果不正确: %+v", wiki)
	}

	direct, err := ParseDocumentTarget("DOCID")
	if err != nil {
		t.Fatalf("解析 document_id 失败: %v", err)
	}
	if direct.DocumentID != "DOCID" {
		t.Fatalf("document_id 解析结果不正确: %+v", direct)
	}
}

func TestClientExportMarkdownRendersBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("飞书请求未带 token: %s", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/open-apis/docx/v1/documents/doc123":
			writeFeishuJSON(writer, map[string]any{"document": map[string]any{"document_id": "doc123", "title": "测试文档"}})
		case "/open-apis/docx/v1/documents/doc123/blocks":
			writeFeishuJSON(writer, map[string]any{
				"items": []map[string]any{
					{"block_id": "doc123", "block_type": 1, "children": []string{"heading", "body"}},
					{"block_id": "heading", "block_type": 3, "heading1": textPayload("标题")},
					{"block_id": "body", "block_type": 2, "text": textPayload("正文")},
				},
				"has_more": false,
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", server.Client())
	result, err := client.ExportMarkdown(context.Background(), "doc123", true)
	if err != nil {
		t.Fatalf("导出 Markdown 失败: %v", err)
	}
	if result.DocumentID != "doc123" || !strings.Contains(result.Markdown, "# 标题") || !strings.Contains(result.Markdown, "feishu-docx:block_id=body") {
		t.Fatalf("Markdown 导出结果不正确: %+v", result)
	}
}

func TestMarkdownRendererDoesNotDuplicateTableChildren(t *testing.T) {
	renderer := newMarkdownRenderer([]Block{
		{"block_id": "doc123", "block_type": 1, "children": []string{"table"}},
		{"block_id": "table", "block_type": 31, "children": []string{"cell1", "cell2"}, "table": map[string]any{"property": map[string]any{"row_size": 1, "column_size": 2}}},
		{"block_id": "cell1", "block_type": 32, "children": []string{"text1"}},
		{"block_id": "cell2", "block_type": 32, "children": []string{"text2"}},
		{"block_id": "text1", "block_type": 2, "text": textPayload("A")},
		{"block_id": "text2", "block_type": 2, "text": textPayload("B")},
	}, "表格", false)

	markdown := renderer.Render("doc123")
	if strings.Count(markdown, "A") != 1 || strings.Count(markdown, "B") != 1 {
		t.Fatalf("表格单元格内容不应被重复渲染: %s", markdown)
	}
}

func TestClientAppendMarkdownUsesDescendantAPI(t *testing.T) {
	var descendantCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/docx/v1/documents/blocks/convert":
			writeFeishuJSON(writer, map[string]any{
				"first_level_block_ids": []string{"tmp1"},
				"blocks": []map[string]any{
					{"block_id": "tmp1", "block_type": 12, "bullet": textPayload("父项"), "children": []string{"tmp2"}},
					{"block_id": "tmp2", "block_type": 2, "text": textPayload("子项")},
				},
			})
		case "/open-apis/docx/v1/documents/doc123/blocks/doc123/descendant":
			descendantCalled = true
			if request.URL.Query().Get("document_revision_id") != "-1" {
				t.Fatalf("descendant 请求未带最新 revision: %s", request.URL.RawQuery)
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("解析 descendant 请求体失败: %v", err)
			}
			if len(body["children_id"].([]any)) != 1 || len(body["descendants"].([]any)) != 2 {
				t.Fatalf("descendant 请求体未保留层级关系: %+v", body)
			}
			writeFeishuJSON(writer, map[string]any{
				"children":        []map[string]any{{"block_id": "real1", "block_type": 12}},
				"revision_id":     12,
				"client_token":    "client",
				"block_relations": []map[string]any{},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", server.Client())
	result, err := client.AppendMarkdown(context.Background(), "doc123", "- 父项\n  - 子项")
	if err != nil {
		t.Fatalf("追加 Markdown 失败: %v", err)
	}
	if !descendantCalled || result.CreatedBlocks != 2 {
		t.Fatalf("descendant 未调用或创建数量不正确: called=%v result=%+v", descendantCalled, result)
	}
}

func TestClientResolveWikiDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/open-apis/wiki/v2/spaces/get_node" || request.URL.Query().Get("token") != "WIKI123" {
			t.Fatalf("wiki 节点请求不正确: %s", request.URL.String())
		}
		writeFeishuJSON(writer, map[string]any{"node": map[string]any{"node_token": "WIKI123", "obj_token": "doc123", "obj_type": "docx"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", server.Client())
	target, err := client.ResolveDocument(context.Background(), "https://acme.feishu.cn/wiki/WIKI123")
	if err != nil {
		t.Fatalf("解析 wiki 文档失败: %v", err)
	}
	if target.DocumentID != "doc123" || target.SourceType != "wiki" {
		t.Fatalf("wiki 文档解析结果不正确: %+v", target)
	}
}

func TestClientWikiBrowsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/wiki/v2/spaces":
			if request.URL.Query().Get("page_token") != "pt1" || request.URL.Query().Get("page_size") != "2" {
				t.Fatalf("知识库列表分页参数不正确: %s", request.URL.RawQuery)
			}
			writeFeishuJSON(writer, map[string]any{
				"items":      []map[string]any{{"space_id": "sp1", "name": "操作文档", "description": "SOP"}},
				"page_token": "pt2",
				"has_more":   true,
			})
		case "/open-apis/wiki/v2/spaces/sp1":
			writeFeishuJSON(writer, map[string]any{"space": map[string]any{"space_id": "sp1", "name": "操作文档"}})
		case "/open-apis/wiki/v2/spaces/sp1/nodes":
			if request.URL.Query().Get("parent_node_token") != "PARENT" || request.URL.Query().Get("page_size") != "2" {
				t.Fatalf("知识库节点列表参数不正确: %s", request.URL.RawQuery)
			}
			writeFeishuJSON(writer, map[string]any{
				"items": []map[string]any{{
					"space_id":          "sp1",
					"node_token":        "NODE1",
					"obj_token":         "doc123",
					"obj_type":          "docx",
					"parent_node_token": "PARENT",
					"title":             "上线 SOP",
					"has_child":         true,
				}},
				"has_more": false,
			})
		case "/open-apis/wiki/v2/spaces/get_node":
			if request.URL.Query().Get("token") != "NODE1" || request.URL.Query().Get("obj_type") != "wiki" {
				t.Fatalf("知识库节点详情参数不正确: %s", request.URL.RawQuery)
			}
			writeFeishuJSON(writer, map[string]any{"node": map[string]any{
				"space_id":   "sp1",
				"node_token": "NODE1",
				"obj_token":  "doc123",
				"obj_type":   "docx",
				"title":      "上线 SOP",
			}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", server.Client())
	spaces, err := client.ListWikiSpaces(context.Background(), "pt1", 2)
	if err != nil {
		t.Fatalf("列出知识库失败: %v", err)
	}
	if len(spaces.Items) != 1 || spaces.Items[0].SpaceID != "sp1" || !spaces.HasMore || spaces.PageToken != "pt2" {
		t.Fatalf("知识库列表结果不正确: %+v", spaces)
	}

	space, err := client.GetWikiSpace(context.Background(), "sp1")
	if err != nil {
		t.Fatalf("获取知识库详情失败: %v", err)
	}
	if space.Name != "操作文档" {
		t.Fatalf("知识库详情不正确: %+v", space)
	}

	nodes, err := client.ListWikiNodes(context.Background(), "sp1", "https://acme.feishu.cn/wiki/PARENT", "", 2)
	if err != nil {
		t.Fatalf("列出知识库节点失败: %v", err)
	}
	if len(nodes.Items) != 1 || nodes.Items[0].SpaceID != "sp1" || nodes.Items[0].DocumentURL == "" || nodes.Items[0].NodeURL == "" {
		t.Fatalf("知识库节点列表结果不正确: %+v", nodes)
	}

	node, err := client.GetWikiNode(context.Background(), "https://acme.feishu.cn/wiki/NODE1")
	if err != nil {
		t.Fatalf("获取知识库节点失败: %v", err)
	}
	if node.ObjToken != "doc123" || node.SpaceID != "sp1" || node.DocumentURL == "" {
		t.Fatalf("知识库节点详情不正确: %+v", node)
	}
}

func writeFeishuJSON(writer http.ResponseWriter, data map[string]any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

func textPayload(content string) map[string]any {
	return map[string]any{
		"elements": []map[string]any{
			{"text_run": map[string]any{"content": content}},
		},
	}
}
