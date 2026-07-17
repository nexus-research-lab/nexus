package workspace

import (
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

func TestBuildWorkspaceFileDispositionHeader(t *testing.T) {
	t.Parallel()

	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", ""), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", workspaceFileDispositionInline), workspaceFileDispositionInline, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", "invalid"), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("报告.pdf", ""), workspaceFileDispositionAttachment, "报告.pdf")
}

func TestServeRawWorkspaceHTMLSupportsStageSitePreview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "index.html")
	content := `<!doctype html><link rel="stylesheet" href="./app.css"><script src="./app.js"></script>`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试 HTML 失败: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/agents/agent-1/workspace/site/demo/index.html", nil)
	handler := &Handlers{}
	handler.serveRawWorkspaceFile(recorder, request, &workspacepkg.RawFile{
		Path:        "demo/index.html",
		FilePath:    filePath,
		Name:        "index.html",
		Size:        int64(len(content)),
		ModifiedAt:  time.Unix(1_700_000_000, 0),
		ContentType: "text/html; charset=utf-8",
		ETag:        `"stage-html-v1"`,
	})

	response := recorder.Result()
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取 HTML 响应失败: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(body) != content {
		t.Fatalf("舞台站点文件响应不正确: status=%d body=%q", response.StatusCode, string(body))
	}
	if !strings.HasPrefix(response.Header.Get("Content-Disposition"), workspaceFileDispositionInline) {
		t.Fatalf("HTML 站点文件必须以内联方式返回: %q", response.Header.Get("Content-Disposition"))
	}
	if !strings.Contains(response.Header.Get("Content-Security-Policy"), "frame-ancestors 'self'") {
		t.Fatalf("HTML 站点文件缺少舞台 iframe CSP: %q", response.Header.Get("Content-Security-Policy"))
	}
}

func assertWorkspaceFileDispositionHeader(t *testing.T, header string, wantDisposition string, wantFilename string) {
	t.Helper()

	disposition, params, err := mime.ParseMediaType(header)
	if err != nil {
		t.Fatalf("解析 Content-Disposition 失败: %v", err)
	}
	if disposition != wantDisposition {
		t.Fatalf("disposition=%q, want %q, header=%q", disposition, wantDisposition, header)
	}
	if params["filename"] != wantFilename {
		t.Fatalf("filename=%q, want %q, header=%q", params["filename"], wantFilename, header)
	}
}
