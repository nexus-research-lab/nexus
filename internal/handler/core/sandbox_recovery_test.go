package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

func TestSandboxResourceInspectionIsOwnerScoped(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	runtimeRoot := filepath.Join(stateRoot, "users", "owner", "runtime")
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := runtimectx.Acquire(t.Context(), runtimectx.Input{
		OwnerUserID: "owner",
		SessionKey:  "sandbox-inspect",
		Root:        runtimeRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()

	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)
	request := newSandboxRecoveryRequest(http.MethodGet, "/settings/runtime/sandbox/resources", "owner", "")
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceInspection(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("inspection status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data sandboxRecoveryInspection `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.OwnerUserID != "owner" || len(response.Data.Resources) != 1 {
		t.Fatalf("inspection = %#v, want one owner resource", response.Data)
	}
	if response.Data.Resources[0].Marker.SessionKey != "sandbox-inspect" || !response.Data.Resources[0].ProcessActive {
		t.Fatalf("inspection resource = %#v", response.Data.Resources[0])
	}
	if body := recorder.Body.String(); strings.Contains(body, runtimeRoot) || strings.Contains(body, `"runtime_root"`) {
		t.Fatalf("inspection exposed host filesystem identity: %s", body)
	}

	otherRequest := newSandboxRecoveryRequest(http.MethodGet, "/settings/runtime/sandbox/resources", "other", "")
	otherRecorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceInspection(otherRecorder, otherRequest)
	if otherRecorder.Code != http.StatusOK {
		t.Fatalf("other-owner inspection status = %d: %s", otherRecorder.Code, otherRecorder.Body.String())
	}
	var otherResponse struct {
		Data sandboxRecoveryInspection `json:"data"`
	}
	if err := json.Unmarshal(otherRecorder.Body.Bytes(), &otherResponse); err != nil {
		t.Fatal(err)
	}
	if otherResponse.Data.OwnerUserID != "other" || len(otherResponse.Data.Resources) != 0 {
		t.Fatalf("other-owner inspection = %#v, want empty", otherResponse.Data)
	}
}

func TestSandboxRecoveryDisablesCaching(t *testing.T) {
	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)
	request := newSandboxRecoveryRequest(http.MethodGet, "/settings/runtime/sandbox/resources", "owner", "")
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceInspection(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("inspection status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("inspection cache policy = %q, want no-store", got)
	}

	reconcileRequest := newSandboxRecoveryRequest(http.MethodPost, "/settings/runtime/sandbox/reconcile", "owner", `{"older_than_seconds":1}`)
	reconcileRecorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceReconcile(reconcileRecorder, reconcileRequest)
	if reconcileRecorder.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d, want 200: %s", reconcileRecorder.Code, reconcileRecorder.Body.String())
	}
	if got := reconcileRecorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("reconcile cache policy = %q, want no-store", got)
	}
}

func TestSandboxResourceReconcileDefaultsToDryRunAndRequiresAge(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	runtimeRoot := filepath.Join(stateRoot, "users", "owner", "runtime")
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := runtimectx.Acquire(t.Context(), runtimectx.Input{
		OwnerUserID: "owner",
		SessionKey:  "sandbox-reconcile",
		Root:        runtimeRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()
	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)

	request := newSandboxRecoveryRequest(
		http.MethodPost,
		"/settings/runtime/sandbox/reconcile",
		"owner",
		`{"older_than_seconds":1}`,
	)
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceReconcile(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data sandboxRecoveryResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Apply || len(response.Data.Candidates) != 0 || len(response.Data.Removed) != 0 || len(response.Data.Skipped) != 1 {
		t.Fatalf("dry-run result = %#v", response.Data)
	}

	applyRequest := newSandboxRecoveryRequest(
		http.MethodPost,
		"/settings/runtime/sandbox/reconcile",
		"owner",
		`{"older_than_seconds":1,"apply":true}`,
	)
	applyRecorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceReconcile(applyRecorder, applyRequest)
	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("apply status = %d: %s", applyRecorder.Code, applyRecorder.Body.String())
	}
	if err := json.Unmarshal(applyRecorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.Apply || len(response.Data.Removed) != 0 || len(response.Data.Skipped) != 1 {
		t.Fatalf("active apply result = %#v", response.Data)
	}

	invalidRequest := newSandboxRecoveryRequest(
		http.MethodPost,
		"/settings/runtime/sandbox/reconcile",
		"owner",
		`{"older_than_seconds":0,"apply":true}`,
	)
	invalidRecorder := httptest.NewRecorder()
	handlers.HandleSandboxResourceReconcile(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid age status = %d, want 400: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func TestSandboxPolicyReceiptRequiresAnOwnerScopedConnectedSession(t *testing.T) {
	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)
	request := newSandboxRecoveryRequest(
		http.MethodGet,
		"/settings/runtime/sandbox/receipt?session_key=agent:owner:dm:missing",
		"owner",
		"",
	)
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxPolicyReceipt(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing runtime receipt status = %d, want 404: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("missing runtime receipt cache policy = %q, want no-store", got)
	}
}

type persistentSandboxReceiptStore struct {
	snapshot protocol.SandboxPolicyReceiptSnapshot
}

func (s *persistentSandboxReceiptStore) Save(_ context.Context, snapshot protocol.SandboxPolicyReceiptSnapshot) error {
	s.snapshot = snapshot
	return nil
}

func (s *persistentSandboxReceiptStore) UpdatePhase(_ context.Context, _ string, _ string, _ uint64, _ protocol.SandboxPolicyReceiptPhase, _ string) error {
	return nil
}

func (s *persistentSandboxReceiptStore) Latest(_ context.Context, owner, session string) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	if owner != s.snapshot.OwnerUserID || session != s.snapshot.SessionKey {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, nil
	}
	return s.snapshot, true, nil
}

func TestSandboxPolicyReceiptFallsBackToDurableOwnerScopedSnapshot(t *testing.T) {
	store := &persistentSandboxReceiptStore{snapshot: protocol.SandboxPolicyReceiptSnapshot{
		Version: 1, OwnerUserID: "owner", SessionKey: "agent:owner:dm:persisted", SessionID: "bridge-session",
		RuntimeKind: "nxs", Generation: 2, PolicyDigest: "sha256:persisted",
		RequiredCapabilities: []string{"sandbox"}, AcknowledgedCapabilities: []string{"sandbox"},
		CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested",
		ResourcePolicyJSON: `{"version":1,"write_scope":"workspace-write","scratch_root":"/private/owner/runtime/sandbox"}`,
		Phase:              protocol.SandboxPolicyReceiptRetired,
	}}
	manager := runtimectx.NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)
	handlers.SetRuntimeManager(manager)
	request := newSandboxRecoveryRequest(
		http.MethodGet,
		"/settings/runtime/sandbox/receipt?session_key=agent:owner:dm:persisted",
		"owner",
		"",
	)
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxPolicyReceipt(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "sha256:persisted") ||
		strings.Contains(recorder.Body.String(), "/private/owner/runtime/sandbox") {
		t.Fatalf("durable receipt status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	other := newSandboxRecoveryRequest(
		http.MethodGet,
		"/settings/runtime/sandbox/receipt?session_key=agent:owner:dm:persisted",
		"other",
		"",
	)
	otherRecorder := httptest.NewRecorder()
	handlers.HandleSandboxPolicyReceipt(otherRecorder, other)
	if otherRecorder.Code != http.StatusNotFound {
		t.Fatalf("cross-owner durable receipt status=%d body=%s", otherRecorder.Code, otherRecorder.Body.String())
	}
}

func TestSandboxPolicyReceiptRedactsUnknownReason(t *testing.T) {
	store := &persistentSandboxReceiptStore{snapshot: protocol.SandboxPolicyReceiptSnapshot{
		Version: 1, OwnerUserID: "owner", SessionKey: "agent:owner:dm:unknown", SessionID: "bridge-session",
		RuntimeKind: "nxs", Generation: 3, PolicyDigest: "sha256:unknown",
		RequiredCapabilities: []string{"sandbox"}, AcknowledgedCapabilities: []string{"sandbox"},
		CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested",
		Phase:         protocol.SandboxPolicyReceiptUnknown,
		UnknownReason: "disconnect failed at /private/owner/runtime/sandbox with secret command output",
	}}
	manager := runtimectx.NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	handlers := New(shared.NewAPI(logx.NewDiscardLogger()), nil, nil)
	handlers.SetRuntimeManager(manager)
	request := newSandboxRecoveryRequest(
		http.MethodGet,
		"/settings/runtime/sandbox/receipt?session_key=agent:owner:dm:unknown",
		"owner",
		"",
	)
	recorder := httptest.NewRecorder()
	handlers.HandleSandboxPolicyReceipt(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "cleanup_unknown") ||
		strings.Contains(recorder.Body.String(), "/private/owner/runtime/sandbox") ||
		strings.Contains(recorder.Body.String(), "secret command output") {
		t.Fatalf("redacted receipt status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newSandboxRecoveryRequest(method, path, owner, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request = request.WithContext(authsvc.WithPrincipal(request.Context(), &authsvc.Principal{UserID: owner}))
	if body == "" {
		request.Body = io.NopCloser(strings.NewReader(""))
	}
	return request
}
