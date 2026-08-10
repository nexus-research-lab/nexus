package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestOpenWorkspaceRejectsOwnerWorkspaceSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	ownerBWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(ownerBWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerAWorkspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspace := filepath.Join(ownerAWorkspaceRoot, "agent-a")
	if err := os.Symlink(
		filepath.Join("..", "..", "user-b", "workspace", "agent-b"),
		ownerAWorkspace,
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	service := NewService(nil, filepath.Join(stateRoot, "users"))
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-a",
	})
	root, err := service.openWorkspace(ctx, ownerAWorkspace, false)
	if root != nil {
		_ = root.Close()
	}
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("图片服务不能借 owner workspace symlink 跨用户: %v", err)
	}
}

func TestGenerateImageSupportsAzureDeploymentURL(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		expectedPath := "/openai/deployments/gpt-image-2/images/generations"
		if request.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("api-version") != "2024-02-01" {
			t.Fatalf("missing api-version: %s", request.URL.RawQuery)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := body["model"]; ok {
			t.Fatalf("azure deployment request must not include model: %+v", body)
		}
		if body["output_compression"].(float64) != 100 {
			t.Fatalf("unexpected output_compression: %+v", body)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}},
		})
	}))
	defer server.Close()

	compression := 100
	workspacePath := newImagegenWorkspace(t)
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}}, "")
	result, _, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:            "A photograph of a red fox in an autumn forest",
		WorkspacePath:     workspacePath,
		Quality:           "low",
		OutputFormat:      "png",
		OutputCompression: &compression,
		FileName:          "fox",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if result.Path != "output/imagegen/fox.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
}

func TestEditImageSupportsAzureMultipartAPI(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/openai/deployments/gpt-image-2/images/edits" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("api-version") != "2024-02-01" {
			t.Fatalf("missing api-version: %s", request.URL.RawQuery)
		}
		reader, err := request.MultipartReader()
		if err != nil {
			t.Fatalf("expected multipart request: %v", err)
		}
		seen := map[string]string{}
		for {
			part, partErr := reader.NextPart()
			if partErr == io.EOF {
				break
			}
			if partErr != nil {
				t.Fatalf("read multipart: %v", partErr)
			}
			data, _ := io.ReadAll(part)
			seen[part.FormName()] = string(data)
		}
		if seen["prompt"] != "Make this black and white" || seen["image"] == "" || seen["mask"] == "" {
			t.Fatalf("unexpected multipart fields: %+v", seen)
		}
		if _, ok := seen["model"]; ok {
			t.Fatalf("azure edit request must not include model: %+v", seen)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}},
		})
	}))
	defer server.Close()

	workspacePath := newImagegenWorkspace(t)
	writeTestPNG(t, filepath.Join(workspacePath, "image_to_edit.png"))
	writeTestPNG(t, filepath.Join(workspacePath, "mask.png"))
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}}, "")
	result, _, err := service.EditImage(context.Background(), EditInput{
		Prompt:        "Make this black and white",
		WorkspacePath: workspacePath,
		ImagePath:     "image_to_edit.png",
		MaskPath:      "mask.png",
		FileName:      "edited",
	})
	if err != nil {
		t.Fatalf("EditImage returned error: %v", err)
	}
	if result.Path != "output/imagegen/edited.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
}

func TestGenerateImageCallsOpenAICompatibleProviderAndWritesFile(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected auth: %q", request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "gpt-image-1" || body["prompt"] != "a clean product photo" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if body["response_format"] != "b64_json" {
			t.Fatalf("provider_options 未透传到 OpenAI-compatible 请求体: %+v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{
				"b64_json":       base64.StdEncoding.EncodeToString(imageBytes),
				"revised_prompt": "revised",
			}},
		})
	}))
	defer server.Close()

	workspacePath := newImagegenWorkspace(t)
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "openai",
		AuthToken: "test-token",
		BaseURL:   server.URL + "/v1",
		Model:     "gpt-image-1",
		ProviderOptions: map[string]any{
			"response_format": "b64_json",
		},
	}}, "")
	service.now = fixedNow

	result, payload, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "a clean product photo",
		WorkspacePath: workspacePath,
		FileName:      "hero-image",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if string(payload) != string(imageBytes) {
		t.Fatalf("payload mismatch")
	}
	if result.Path != "output/imagegen/hero-image.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
	if result.MIMEType != "image/png" {
		t.Fatalf("unexpected mime: %s", result.MIMEType)
	}
	stored, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(result.Path)))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(stored) != string(imageBytes) {
		t.Fatalf("stored file mismatch")
	}
}

func TestGenerateImageNormalizesImage2PixelSizeBeforeRequest(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "gpt-image-2" {
			t.Fatalf("unexpected model: %+v", body)
		}
		if body["size"] != "1920x1088" {
			t.Fatalf("image2 请求前应规整到 16 倍数: %+v", body)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}},
		})
	}))
	defer server.Close()

	workspacePath := newImagegenWorkspace(t)
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "openai",
		AuthToken: "test-token",
		BaseURL:   server.URL + "/v1",
		Model:     "gpt-image-2",
	}}, "")

	result, _, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "cinematic wide scene",
		WorkspacePath: workspacePath,
		Size:          "1920x1080",
		FileName:      "wide-scene",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if result.Size != "1920x1088" {
		t.Fatalf("result size 未同步规整后尺寸: %+v", result)
	}
}

func TestResolveImageConfigSelectsProviderModel(t *testing.T) {
	for _, test := range []struct {
		name       string
		preference bool
	}{
		{name: "preference default", preference: true},
		{name: "explicit selection"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := &fakeProviderResolver{config: &providercfg.ImageConfig{
				Provider:  "image-provider",
				AuthToken: "token",
				BaseURL:   "https://image.example.com/v1/images",
				Model:     "image-model",
			}}
			service := NewService(resolver, "")
			provider, model := "image-provider", "image-model"
			if test.preference {
				provider, model = "", ""
				service.SetPreferences(fakePreferencesService{prefs: preferencessvc.Preferences{
					DefaultImageModelSelection: preferencessvc.ModelSelection{
						Provider: "image-provider",
						Model:    "image-model",
					},
				}})
			}
			config, err := service.resolveImageConfig(context.Background(), provider, model)
			if err != nil {
				t.Fatalf("解析图片模型失败: %v", err)
			}
			if config.Model != "image-model" || resolver.provider != "image-provider" || resolver.model != "image-model" {
				t.Fatalf("图片模型选择错误: config=%+v provider=%s model=%s", config, resolver.provider, resolver.model)
			}
		})
	}
}
