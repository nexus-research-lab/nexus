// INPUT: owner preferences, verified package manager, and private sidecar supervisor.
// OUTPUT: settings lifecycle operations and owner/round-gated Computer Use admission.
// POS: Nexus Computer Use product service; owns policy and composition, not desktop execution.
package computeruse

import (
	"context"
	"errors"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type preferenceReader interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type Service struct {
	available   bool
	preferences preferenceReader
	packages    *PackageManager
	supervisor  *Supervisor

	adminMu  sync.RWMutex
	roundsMu sync.Mutex
	rounds   map[string]*roundState
}

func NewService(
	available bool,
	preferences preferenceReader,
	packages *PackageManager,
	supervisor *Supervisor,
) *Service {
	return &Service{
		available: available, preferences: preferences, packages: packages,
		supervisor: supervisor, rounds: make(map[string]*roundState),
	}
}

func (service *Service) Enabled(ctx context.Context, ownerUserID string) (bool, error) {
	if service == nil || !service.available {
		return false, nil
	}
	if service.preferences == nil {
		return false, nil
	}
	preferences, err := service.preferences.Get(ctx, ownerUserID)
	if err != nil {
		return false, err
	}
	return preferences.ComputerUseEnabled, nil
}

func (service *Service) Status(ctx context.Context, ownerUserID string) (Status, error) {
	enabled, err := service.Enabled(ctx, ownerUserID)
	if err != nil {
		return Status{}, err
	}
	status := Status{Enabled: enabled}
	if service == nil || service.packages == nil || service.supervisor == nil {
		status.Package = PackageStatus{Available: false, Message: ErrUnavailable.Error()}
		status.Sidecar = SidecarStatus{State: SidecarStopped}
		status.NextActions = []string{"use a supported Nexus desktop host"}
		return status, nil
	}
	status.Package = service.packages.Status(ctx)
	status.Sidecar = service.supervisor.Status()
	switch {
	case !status.Package.Available:
		status.NextActions = []string{"use a supported Nexus desktop host"}
	case !status.Package.Installed && status.Package.CanInstall:
		status.NextActions = []string{"install the pinned Computer Use runtime"}
	case !status.Package.Installed:
		status.NextActions = []string{"configure a signed Computer Use release manifest"}
	case !enabled:
		status.NextActions = []string{"enable Computer Use in Settings"}
	case status.Sidecar.State == SidecarFailed:
		status.NextActions = []string{"run Computer Use doctor", "restart the Computer Use runtime"}
	case status.Sidecar.State == SidecarStopped:
		status.NextActions = []string{"start Computer Use or let the first Agent operation start it"}
	}
	return status, nil
}

func (service *Service) Install(ctx context.Context) (PackageStatus, error) {
	if service == nil || service.packages == nil || service.supervisor == nil {
		return PackageStatus{}, ErrUnavailable
	}
	service.adminMu.Lock()
	defer service.adminMu.Unlock()
	if err := service.stopLocked(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return PackageStatus{}, err
	}
	return service.packages.Install(ctx)
}

func (service *Service) Update(ctx context.Context) (PackageStatus, error) {
	return service.Install(ctx)
}

func (service *Service) Remove(ctx context.Context) error {
	if service == nil || service.packages == nil || service.supervisor == nil {
		return ErrUnavailable
	}
	service.adminMu.Lock()
	defer service.adminMu.Unlock()
	if err := service.stopLocked(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return service.packages.Remove()
}

func (service *Service) Doctor(ctx context.Context) (DoctorReport, error) {
	if service == nil || service.packages == nil {
		return DoctorReport{}, ErrUnavailable
	}
	service.adminMu.RLock()
	defer service.adminMu.RUnlock()
	return service.packages.Doctor(ctx)
}

func (service *Service) Start(ctx context.Context, ownerUserID string) (SidecarStatus, error) {
	enabled, err := service.Enabled(ctx, ownerUserID)
	if err != nil {
		return SidecarStatus{}, err
	}
	if !enabled {
		return SidecarStatus{}, ErrDisabled
	}
	if service.supervisor == nil {
		return SidecarStatus{}, ErrUnavailable
	}
	service.adminMu.RLock()
	defer service.adminMu.RUnlock()
	if _, _, _, err = service.supervisor.EnsureReady(ctx); err != nil {
		return service.supervisor.Status(), err
	}
	return service.supervisor.Status(), nil
}

func (service *Service) admitOperation() func() {
	service.adminMu.RLock()
	return service.adminMu.RUnlock
}

func (service *Service) Stop(ctx context.Context) error {
	if service == nil || service.supervisor == nil {
		return nil
	}
	service.adminMu.Lock()
	defer service.adminMu.Unlock()
	return service.stopLocked(ctx)
}

func (service *Service) stopLocked(ctx context.Context) error {
	service.closeAllRounds()
	return service.supervisor.Stop(ctx)
}

func (service *Service) Close(ctx context.Context) error {
	return service.Stop(ctx)
}

func (service *Service) authorizeActor(ctx context.Context, actor runtimecommand.Actor) error {
	if service == nil || !service.available || service.supervisor == nil {
		return ErrUnavailable
	}
	if !actor.Valid() {
		return errors.New("Computer Use command actor is invalid")
	}
	enabled, err := service.Enabled(ctx, actor.OwnerUserID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrDisabled
	}
	return nil
}
