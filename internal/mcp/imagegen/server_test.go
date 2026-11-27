package imagegenmcp

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
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
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1.0.0", BuildTools(svc, sctx))
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
