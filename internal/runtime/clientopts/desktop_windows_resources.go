// INPUT: Canonical workspace of a restricted Windows nxs session.
// OUTPUT: Workspace-scoped scratch resource policy without persisted leases.
// POS: Windows resource adapter for the stateless desktop host contract.
package clientopts

import (
	"fmt"
	"path/filepath"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func prepareWindowsSandboxResources(workspace string) (*agentclient.SandboxResourcePolicy, error) {
	if !filepath.IsAbs(workspace) {
		return nil, fmt.Errorf("Windows sandbox workspace must be absolute")
	}
	root, err := confinedfs.Open(workspace)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	const relative = ".nexus/sandbox-tmp"
	scratch, err := root.OpenOrCreateRootNoSymlink(relative, 0700)
	if err != nil {
		return nil, fmt.Errorf("prepare Windows sandbox scratch: %w", err)
	}
	if err := scratch.Close(); err != nil {
		return nil, err
	}
	return &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: filepath.Join(workspace, filepath.FromSlash(relative))}, nil
}
