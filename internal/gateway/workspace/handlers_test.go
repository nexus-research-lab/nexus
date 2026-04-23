package workspace

import "testing"

func TestBuildWorkspaceFileDispositionHeader(t *testing.T) {
	t.Parallel()

	if got := buildWorkspaceFileDispositionHeader("demo.pdf", ""); got != `attachment; filename="demo.pdf"` {
		t.Fatalf("expected attachment disposition, got %q", got)
	}

	if got := buildWorkspaceFileDispositionHeader("demo.pdf", workspaceFileDispositionInline); got != `inline; filename="demo.pdf"` {
		t.Fatalf("expected inline disposition, got %q", got)
	}

	if got := buildWorkspaceFileDispositionHeader("demo.pdf", "invalid"); got != `attachment; filename="demo.pdf"` {
		t.Fatalf("expected invalid disposition to fallback to attachment, got %q", got)
	}
}
