// INPUT: authenticated HTTP contexts and deterministic lifecycle service responses.
// OUTPUT: owner scoping and stable settings status/error response evidence.
// POS: Computer Use settings transport regression tests.
package computeruse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	computerusesvc "github.com/nexus-research-lab/nexus/internal/service/computeruse"
)

type fakeLifecycleService struct {
	lifecycleService
	owner string
	err   error
}

func (service *fakeLifecycleService) Status(_ context.Context, ownerUserID string) (computerusesvc.Status, error) {
	service.owner = ownerUserID
	return computerusesvc.Status{Enabled: true, Package: computerusesvc.PackageStatus{Available: true}}, service.err
}

func TestHandleStatusUsesAuthenticatedOwner(t *testing.T) {
	service := &fakeLifecycleService{}
	handlers := New(shared.NewAPI(nil), service)
	request := httptest.NewRequest(http.MethodGet, "/settings/computer-use", nil)
	request = request.WithContext(authctx.WithPrincipal(request.Context(), &authctx.Principal{UserID: "owner-42"}))
	recorder := httptest.NewRecorder()
	handlers.HandleStatus(recorder, request)
	if recorder.Code != http.StatusOK || service.owner != "owner-42" || !strings.Contains(recorder.Body.String(), `"enabled":true`) {
		t.Fatalf("response = %d %s, owner = %q", recorder.Code, recorder.Body.String(), service.owner)
	}
}

func TestHandleStatusMapsUnavailableToNotFound(t *testing.T) {
	service := &fakeLifecycleService{err: errors.Join(computerusesvc.ErrUnavailable, errors.New("private /tmp/transport.sock"))}
	handlers := New(shared.NewAPI(nil), service)
	recorder := httptest.NewRecorder()
	handlers.HandleStatus(recorder, httptest.NewRequest(http.MethodGet, "/settings/computer-use", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "/tmp/transport.sock") {
		t.Fatalf("response exposed a private transport path: %s", recorder.Body.String())
	}
}

func TestHandleStatusContainsUnknownInternalErrors(t *testing.T) {
	service := &fakeLifecycleService{err: errors.New("dial /private/token-bearing/service.sock")}
	handlers := New(shared.NewAPI(nil), service)
	recorder := httptest.NewRecorder()
	handlers.HandleStatus(recorder, httptest.NewRequest(http.MethodGet, "/settings/computer-use", nil))
	if recorder.Code != http.StatusUnprocessableEntity || strings.Contains(recorder.Body.String(), "/private/") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}
