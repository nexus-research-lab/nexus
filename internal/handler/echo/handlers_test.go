package echo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	echosvc "github.com/nexus-research-lab/nexus/internal/service/echo"
)

type echoServiceStub struct {
	settings        echodomain.Settings
	err             error
	expectedVersion int64
	usedCAS         bool
}

func (stub *echoServiceStub) GetSettings(context.Context) (echodomain.Settings, error) {
	return stub.settings, stub.err
}

func (stub *echoServiceStub) UpdateSettings(
	context.Context,
	echodomain.Settings,
) (echodomain.Settings, error) {
	return stub.settings, stub.err
}

func (stub *echoServiceStub) UpdateSettingsAtVersion(
	_ context.Context,
	_ echodomain.Settings,
	expectedVersion int64,
) (echodomain.Settings, error) {
	stub.usedCAS = true
	stub.expectedVersion = expectedVersion
	return stub.settings, stub.err
}

func TestGetEchoReturnsRevisionETag(t *testing.T) {
	t.Parallel()
	stub := &echoServiceStub{settings: echodomain.Settings{Enabled: true, Version: 7}}
	handler := New(handlershared.NewAPI(nil), stub)
	recorder := httptest.NewRecorder()
	handler.HandleGetEcho(recorder, httptest.NewRequest(http.MethodGet, "/settings/echo", nil))

	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"echo-7"` {
		t.Fatalf("status=%d ETag=%q", recorder.Code, recorder.Header().Get("ETag"))
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q", recorder.Header().Get("Cache-Control"))
	}
}

func TestUpdateEchoProjectsVersionConflictAsNotApplied(t *testing.T) {
	t.Parallel()
	assertEchoFailure(t, echosvc.ErrSettingsVersionConflict, http.StatusPreconditionFailed,
		"echo.version_conflict", protocol.FailureEffectNotApplied)
}

func TestUpdateEchoProjectsUnclassifiedFailureAsUnknown(t *testing.T) {
	t.Parallel()
	assertEchoFailure(t, errors.New("write interrupted"), http.StatusInternalServerError,
		"echo.update_result_unknown", protocol.FailureEffectUnknown)
}

func assertEchoFailure(
	t *testing.T,
	serviceErr error,
	wantStatus int,
	wantCode string,
	wantEffect protocol.FailureEffect,
) {
	t.Helper()
	stub := &echoServiceStub{err: serviceErr}
	handler := New(handlershared.NewAPI(nil), stub)
	request := httptest.NewRequest(http.MethodPut, "/settings/echo", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("If-Match", `"echo-3"`)
	recorder := httptest.NewRecorder()
	handler.HandleUpdateEcho(recorder, request)

	var payload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != wantStatus || payload.Data.Failure.Code != wantCode ||
		payload.Data.Failure.Effect != wantEffect {
		t.Fatalf("status=%d failure=%+v", recorder.Code, payload.Data.Failure)
	}
}
