package imagegenmcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

type stubImagegenService struct {
	generateInput       imagegensvc.GenerateInput
	generateOwnerUserID string
	editInput           imagegensvc.EditInput
	editOwnerUserID     string
}

func (s *stubImagegenService) GenerateImage(
	ctx context.Context,
	input imagegensvc.GenerateInput,
) (*imagegensvc.Result, []byte, error) {
	s.generateInput = input
	s.generateOwnerUserID = authctx.OwnerUserID(ctx)
	return &imagegensvc.Result{
		Provider: "openai",
		Model:    "gpt-image",
		Path:     "output/imagegen/fox.png",
		MIMEType: "image/png",
		Size:     input.Size,
		Markdown: "![generated image](output/imagegen/fox.png)",
	}, []byte("png"), nil
}

func (s *stubImagegenService) EditImage(
	ctx context.Context,
	input imagegensvc.EditInput,
) (*imagegensvc.Result, []byte, error) {
	s.editInput = input
	s.editOwnerUserID = authctx.OwnerUserID(ctx)
	return &imagegensvc.Result{
		Provider: "openai",
		Model:    "gpt-image",
		Path:     "output/imagegen/edited.png",
		MIMEType: "image/png",
		Markdown: "![edited image](output/imagegen/edited.png)",
	}, []byte("edited"), nil
}

func TestToolsListIncludesImagegenTools(t *testing.T) {
	server := NewServer(&stubImagegenService{}, contract.ServerContext{WorkspacePath: "/tmp/workspace"})
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]map[string]any)
	names := map[string]bool{}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		names[name] = true
		meta, ok := tool["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing _meta", name)
		}
		if hint, _ := meta["anthropic/searchHint"].(string); strings.TrimSpace(hint) == "" {
			t.Fatalf("%s missing search hint", name)
		}
		if alwaysLoad, _ := meta["anthropic/alwaysLoad"].(bool); !alwaysLoad {
			t.Fatalf("%s should always load", name)
		}
	}
	for _, name := range []string{"generate_image", "edit_image"} {
		if !names[name] {
			t.Fatalf("missing imagegen tool %s: %+v", name, tools)
		}
	}
}

func TestGenerateImageUsesInjectedWorkspaceAndOwner(t *testing.T) {
	svc := &stubImagegenService{}
	result, isError := callImagegenTool(t, svc, "generate_image", map[string]any{
		"prompt":        "A golden fox",
		"size":          "1024x1024",
		"file_name":     "fox",
		"output_format": "png",
	}, contract.ServerContext{
		OwnerUserID:   "user-1",
		WorkspacePath: "/workspace/agent",
	})
	if isError {
		t.Fatalf("generate_image 不应失败: %s", extractText(t, result))
	}
	if svc.generateOwnerUserID != "user-1" {
		t.Fatalf("owner user 未注入: %s", svc.generateOwnerUserID)
	}
	if svc.generateInput.WorkspacePath != "/workspace/agent" || svc.generateInput.Prompt != "A golden fox" {
		t.Fatalf("生成入参未使用注入 workspace: %+v", svc.generateInput)
	}
	payload := decodeToolJSON(t, result)
	if payload["domain"] != "imagegen" || payload["action"] != "generate_image" {
		t.Fatalf("工具输出 envelope 不正确: %+v", payload)
	}
	item := payload["item"].(map[string]any)
	if item["path"] != "output/imagegen/fox.png" {
		t.Fatalf("工具输出路径不正确: %+v", item)
	}
	if result["structuredContent"] == nil {
		t.Fatalf("应返回 structuredContent: %+v", result)
	}
}

func TestEditImageUsesWorkspaceRelativePaths(t *testing.T) {
	svc := &stubImagegenService{}
	result, isError := callImagegenTool(t, svc, "edit_image", map[string]any{
		"prompt":     "Replace the background",
		"image_path": "input/photo.png",
		"mask_path":  "input/mask.png",
		"file_name":  "edited",
	}, contract.ServerContext{
		OwnerUserID:   "user-1",
		WorkspacePath: "/workspace/agent",
	})
	if isError {
		t.Fatalf("edit_image 不应失败: %s", extractText(t, result))
	}
	if svc.editInput.WorkspacePath != "/workspace/agent" ||
		svc.editInput.ImagePath != "input/photo.png" ||
		svc.editInput.MaskPath != "input/mask.png" {
		t.Fatalf("编辑入参不正确: %+v", svc.editInput)
	}
	if svc.editOwnerUserID != "user-1" {
		t.Fatalf("owner user 未注入: %s", svc.editOwnerUserID)
	}
}

func callImagegenTool(
	t *testing.T,
	svc contract.Service,
	name string,
	args map[string]any,
	sctx contract.ServerContext,
) (map[string]any, bool) {
	t.Helper()
	server := NewServer(svc, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	isError, _ := result["isError"].(bool)
	return result, isError
}

func extractText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content 格式不正确: %+v", result)
	}
	text, _ := content[0]["text"].(string)
	return text
}

func decodeToolJSON(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(extractText(t, result)), &payload); err != nil {
		t.Fatalf("工具输出不是 JSON: %v", err)
	}
	return payload
}
