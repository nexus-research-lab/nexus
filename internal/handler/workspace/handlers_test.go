package workspace

import (
	"mime"
	"testing"
)

func TestBuildWorkspaceFileDispositionHeader(t *testing.T) {
	t.Parallel()

	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", ""), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", workspaceFileDispositionInline), workspaceFileDispositionInline, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", "invalid"), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("报告.pdf", ""), workspaceFileDispositionAttachment, "报告.pdf")
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
