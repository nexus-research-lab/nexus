package agent

import (
	"context"
	"errors"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type permissionModeRecorder struct {
	ids   []string
	modes []sdkpermission.Mode
	err   error
}

func (r *permissionModeRecorder) SetPermissionModeForAgent(_ context.Context, id string, mode sdkpermission.Mode) error {
	r.ids = append(r.ids, id)
	r.modes = append(r.modes, mode)
	return r.err
}

func TestPermissionModeUpdateContinuesAfterDomainFailure(t *testing.T) {
	dmError, roomError := errors.New("DM cleanup pending"), errors.New("Room cleanup pending")
	dm, room := &permissionModeRecorder{err: dmError}, &permissionModeRecorder{err: roomError}
	err := syncAgentPermissionMode(context.Background(), "agent-a", sdkpermission.ModeDefault, dm, room)
	if !errors.Is(err, dmError) || !errors.Is(err, roomError) {
		t.Fatalf("domain failure was lost: %v", err)
	}
	for _, setter := range []*permissionModeRecorder{dm, room} {
		if len(setter.ids) != 1 || setter.ids[0] != "agent-a" || setter.modes[0] != sdkpermission.ModeDefault {
			t.Fatalf("domain missed mode change: %+v", setter)
		}
	}
}
