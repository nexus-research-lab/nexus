// INPUT: Canonical workspace of a restricted Windows nxs session.
// OUTPUT: Workspace-keyed disjoint scratch resource policy without persisted leases.
// POS: Windows resource adapter for the stateless desktop host contract.
package clientopts

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func prepareWindowsSandboxResources(workspace, owner string) (*agentclient.SandboxResourcePolicy, error) {
	if !filepath.IsAbs(workspace) {
		return nil, fmt.Errorf("Windows sandbox workspace must be absolute")
	}
	canonical, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, err
	}
	if err := appfs.EnsureUserRuntimeLayout(owner); err != nil {
		return nil, err
	}
	runtimeRoot := appfs.UserRuntimeRoot(owner)
	root, err := confinedfs.Open(runtimeRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	// PSEC requires the synthetic profile directories projected by managedUserRuntimeEnv.
	for _, relative := range []string{"home/AppData/Local", "home/AppData/Roaming"} {
		child, err := root.OpenOrCreateRootNoSymlink(relative, 0700)
		if err != nil {
			return nil, fmt.Errorf("prepare Windows sandbox profile: %w", err)
		}
		if err := child.Close(); err != nil {
			return nil, err
		}
	}
	identity := canonical
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	relative := fmt.Sprintf("tmp/sandbox/%x", sha256.Sum256([]byte(identity)))
	scratchPath := filepath.Join(runtimeRoot, filepath.FromSlash(relative))
	for _, pair := range [][2]string{{canonical, scratchPath}, {scratchPath, canonical}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("Windows sandbox scratch must not overlap workspace")
		}
	}
	scratch, err := root.OpenOrCreateRootNoSymlink(relative, 0700)
	if err != nil {
		return nil, fmt.Errorf("prepare Windows sandbox scratch: %w", err)
	}
	if err := scratch.Close(); err != nil {
		return nil, err
	}
	return &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: scratchPath}, nil
}
