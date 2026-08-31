package workspace

import (
	"errors"
	"mime"
	"net/http"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

func TestBuildWorkspaceFileDispositionHeader(t *testing.T) {
	t.Parallel()

	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", ""), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", workspaceFileDispositionInline), workspaceFileDispositionInline, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("demo.pdf", "invalid"), workspaceFileDispositionAttachment, "demo.pdf")
	assertWorkspaceFileDispositionHeader(t, buildWorkspaceFileDispositionHeader("报告.pdf", ""), workspaceFileDispositionAttachment, "报告.pdf")
}

func TestWorkspaceFileRevisionConflictIsNotApplied(t *testing.T) {
	t.Parallel()

	spec := workspaceFileRevisionConflict()
	if spec.Code != "workspace.file_revision_conflict" ||
		spec.Category != protocol.FailureCategoryConflict ||
		spec.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("revision conflict = code %q category %q effect %q", spec.Code, spec.Category, spec.Effect)
	}
	if spec.Resolution == nil || spec.Resolution.Action != "workspace.reload_file" {
		t.Fatalf("revision conflict resolution = %#v", spec.Resolution)
	}
}

func TestWorkspaceMutationFailuresOnlyClaimNotAppliedWithServiceEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantEffect protocol.FailureEffect
		wantAction string
	}{
		{
			name:       "validated before mutation",
			err:        errors.Join(workspacepkg.ErrMutationInvalid, errors.New("目标已存在")),
			wantStatus: http.StatusBadRequest,
			wantEffect: protocol.FailureEffectNotApplied,
			wantAction: "workspace.review_request",
		},
		{
			name:       "unclassified storage error",
			err:        errors.New("disk write failed for 路径-like filename"),
			wantStatus: http.StatusInternalServerError,
			wantEffect: protocol.FailureEffectUnknown,
			wantAction: "workspace.reload_files",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			status, spec := workspaceMutationFailure(workspaceMutationCreate, testCase.err)
			if status != testCase.wantStatus || spec.Effect != testCase.wantEffect {
				t.Fatalf("failure = status %d effect %q", status, spec.Effect)
			}
			if spec.Resolution == nil || spec.Resolution.Action != testCase.wantAction {
				t.Fatalf("resolution = %#v", spec.Resolution)
			}
		})
	}
}

func TestWorkspaceMutationRequestFailureIsNotApplied(t *testing.T) {
	t.Parallel()

	spec := workspaceMutationRequestFailure(workspaceMutationRename)
	if spec.Code != "workspace.rename_request_invalid" ||
		spec.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("request failure = code %q effect %q", spec.Code, spec.Effect)
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
