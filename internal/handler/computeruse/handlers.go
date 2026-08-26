// INPUT: authenticated settings requests with no caller-controlled package or transport identity.
// OUTPUT: owner-safe Computer Use status, doctor, package, and sidecar lifecycle JSON.
// POS: HTTP adapter for the Nexus-owned Computer Use host service.
package computeruse

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	computerusesvc "github.com/nexus-research-lab/nexus/internal/service/computeruse"
)

type lifecycleService interface {
	Status(context.Context, string) (computerusesvc.Status, error)
	Install(context.Context) (computerusesvc.PackageStatus, error)
	Update(context.Context) (computerusesvc.PackageStatus, error)
	Remove(context.Context) error
	Doctor(context.Context) (computerusesvc.DoctorReport, error)
	Start(context.Context, string) (computerusesvc.SidecarStatus, error)
	Stop(context.Context) error
}

type Handlers struct {
	api     *shared.API
	service lifecycleService
}

func New(api *shared.API, service lifecycleService) *Handlers {
	if api == nil || service == nil {
		return nil
	}
	return &Handlers{api: api, service: service}
}

func (handlers *Handlers) HandleStatus(writer http.ResponseWriter, request *http.Request) {
	status, err := handlers.service.Status(request.Context(), authctx.OwnerUserID(request.Context()))
	if handlers.writeError(writer, err) {
		return
	}
	handlers.api.WriteSuccess(writer, status)
}

func (handlers *Handlers) HandleInstall(writer http.ResponseWriter, request *http.Request) {
	status, err := handlers.service.Install(request.Context())
	if handlers.writeError(writer, err) {
		return
	}
	handlers.api.WriteSuccess(writer, status)
}

func (handlers *Handlers) HandleUpdate(writer http.ResponseWriter, request *http.Request) {
	status, err := handlers.service.Update(request.Context())
	if handlers.writeError(writer, err) {
		return
	}
	handlers.api.WriteSuccess(writer, status)
}

func (handlers *Handlers) HandleRemove(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	if handlers.writeError(writer, handlers.service.Remove(ctx)) {
		return
	}
	handlers.api.WriteSuccess(writer, map[string]any{"removed": true})
}

func (handlers *Handlers) HandleDoctor(writer http.ResponseWriter, request *http.Request) {
	report, err := handlers.service.Doctor(request.Context())
	if handlers.writeError(writer, err) {
		return
	}
	handlers.api.WriteSuccess(writer, report)
}

func (handlers *Handlers) HandleStart(writer http.ResponseWriter, request *http.Request) {
	status, err := handlers.service.Start(request.Context(), authctx.OwnerUserID(request.Context()))
	if handlers.writeError(writer, err) {
		return
	}
	handlers.api.WriteSuccess(writer, status)
}

func (handlers *Handlers) HandleStop(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	if handlers.writeError(writer, handlers.service.Stop(ctx)) {
		return
	}
	handlers.api.WriteSuccess(writer, map[string]any{"stopped": true})
}

func (handlers *Handlers) writeError(writer http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusUnprocessableEntity
	switch {
	case errors.Is(err, computerusesvc.ErrUnavailable):
		status = http.StatusNotFound
	case errors.Is(err, computerusesvc.ErrDisabled), errors.Is(err, computerusesvc.ErrExternallyManaged):
		status = http.StatusConflict
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = http.StatusRequestTimeout
	}
	handlers.api.WriteFailure(writer, status, computerusesvc.PublicErrorMessage(err))
	return true
}
