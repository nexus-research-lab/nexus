//go:build linux

package runtimeidentity

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func preparePolicy(
	config launcherConfig,
	current *registry,
	ownerUserID string,
	runtimeKind string,
	cwd string,
	requestedReadRoots []string,
	environmentNames []string,
) (preparedPolicy, bool, error) {
	runtimeKind = strings.ToLower(strings.TrimSpace(runtimeKind))
	if _, ok := config.RuntimeExecutables[runtimeKind]; !ok {
		return preparedPolicy{}, false, fmt.Errorf("runtime %q 不在 executable allowlist", runtimeKind)
	}
	identityValue, changed, err := ensureIdentity(config, current, ownerUserID)
	if err != nil {
		return preparedPolicy{}, false, err
	}
	cwd, err = canonicalExistingOrPendingPath(cwd)
	if err != nil {
		return preparedPolicy{}, false, err
	}
	ownerRoot := appfs.UserDataRootAt(config.StateRoot, ownerUserID)
	workspaceRoot := appfs.UserWorkspaceRootAt(config.StateRoot, ownerUserID)
	projectValue, projectAccess := projectForPath(current, ownerUserID, cwd)
	if cwd == workspaceRoot {
		return preparedPolicy{}, false, errors.New("runtime cwd 必须是 owner workspace 的子目录")
	}
	if !pathWithin(cwd, workspaceRoot) && projectValue == nil {
		return preparedPolicy{}, false, errors.New("runtime cwd 不属于当前 owner 或已授权 project")
	}
	if pathWithin(cwd, workspaceRoot) {
		if filepath.Dir(cwd) != workspaceRoot {
			return preparedPolicy{}, false, errors.New("runtime cwd 必须是 owner workspace 的直接子目录")
		}
		if err = ensureWorkspaceBoundaryACL(cwd, identityValue, config.HostUID); err != nil {
			return preparedPolicy{}, false, fmt.Errorf("修复 runtime workspace ACL: %w", err)
		}
	}
	if projectValue != nil {
		if err = verifyOSGroup(projectValue.GroupName, projectValue.GID); err != nil {
			return preparedPolicy{}, false, err
		}
	}

	readRoots, writeRoots := baseRuntimePolicyRoots(ownerRoot, config)
	supplementaryGIDs := make([]int, 0)
	if projectValue != nil {
		readRoots = append(readRoots, projectValue.Root)
		if projectAccess == projectAccessWrite {
			writeRoots = append(writeRoots, projectValue.Root)
			supplementaryGIDs = append(supplementaryGIDs, projectValue.GID)
		}
	}
	normalizedRequested := make([]string, 0, len(requestedReadRoots))
	for _, root := range requestedReadRoots {
		root, err = canonicalExistingOrPendingPath(root)
		if err != nil {
			return preparedPolicy{}, false, err
		}
		projectValue, _ := projectForPath(current, ownerUserID, root)
		switch {
		case pathWithin(root, ownerRoot):
		case pathWithinAny(root, config.ReadOnlyRoots):
		case pathWithinSharedTemp(root):
		case projectValue != nil:
			if err = verifyOSGroup(projectValue.GroupName, projectValue.GID); err != nil {
				return preparedPolicy{}, false, err
			}
		default:
			return preparedPolicy{}, false, fmt.Errorf("read root 未被授权: %s", root)
		}
		readRoots = append(readRoots, root)
		normalizedRequested = append(normalizedRequested, root)
	}
	readRoots = compactPaths(readRoots)
	writeRoots = compactPaths(writeRoots)
	slices.Sort(supplementaryGIDs)
	supplementaryGIDs = slices.Compact(supplementaryGIDs)
	environmentNames, err = normalizeEnvironmentNames(environmentNames)
	if err != nil {
		return preparedPolicy{}, false, err
	}

	ticketID, err := newOpaqueID()
	if err != nil {
		return preparedPolicy{}, false, err
	}
	now := time.Now().UTC()
	generation := max(current.Generation, identityValue.Generation)
	ticket := launchTicket{
		TicketID:           ticketID,
		OwnerUserID:        ownerUserID,
		RuntimeKind:        runtimeKind,
		CWD:                cwd,
		RequestedReadRoots: compactPaths(normalizedRequested),
		EnvironmentNames:   environmentNames,
		Generation:         generation,
		CreatedAt:          now,
		ExpiresAt:          now.Add(time.Duration(config.TicketTTLSeconds) * time.Second),
	}
	if err = writeLaunchTicket(config, ticket); err != nil {
		return preparedPolicy{}, false, err
	}
	cleanupExpiredTickets(config, now)
	return preparedPolicy{
		OwnerUserID:       ownerUserID,
		RuntimeKind:       runtimeKind,
		CWD:               cwd,
		ReadRoots:         readRoots,
		WriteRoots:        writeRoots,
		RuntimeReadRoots:  readRoots,
		RuntimeWriteRoots: writeRoots,
		EnvironmentNames:  environmentNames,
		Generation:        generation,
		Ticket:            ticketID,
		Identity: preparedIdentity{
			Username:          identityValue.Username,
			UID:               identityValue.UID,
			PrivateGID:        identityValue.PrivateGID,
			SupplementaryGIDs: supplementaryGIDs,
			HomeDir:           identityValue.HomeDir,
			TempDir:           identityValue.TempDir,
		},
	}, changed, nil
}

func policyForTicket(
	config launcherConfig,
	current *registry,
	ticket launchTicket,
) (preparedPolicy, error) {
	policy, _, err := preparePolicyWithoutTicket(
		config,
		current,
		ticket.OwnerUserID,
		ticket.RuntimeKind,
		ticket.CWD,
		ticket.RequestedReadRoots,
		ticket.EnvironmentNames,
	)
	return policy, err
}

func preparePolicyWithoutTicket(
	config launcherConfig,
	current *registry,
	ownerUserID string,
	runtimeKind string,
	cwd string,
	requestedReadRoots []string,
	environmentNames []string,
) (preparedPolicy, bool, error) {
	// 复用完整授权逻辑，但不生成第二张票据。
	runtimeKind = strings.ToLower(strings.TrimSpace(runtimeKind))
	if _, ok := config.RuntimeExecutables[runtimeKind]; !ok {
		return preparedPolicy{}, false, fmt.Errorf("runtime %q 不在 executable allowlist", runtimeKind)
	}
	identityValue := current.Identities[ownerUserID]
	if identityValue == nil || identityValue.Status != "active" {
		return preparedPolicy{}, false, errors.New("runtime identity 不存在或已停用")
	}
	if err := ensureOSAccount(*identityValue); err != nil {
		return preparedPolicy{}, false, err
	}
	cwd, err := canonicalExistingOrPendingPath(cwd)
	if err != nil {
		return preparedPolicy{}, false, err
	}
	ownerRoot := appfs.UserDataRootAt(config.StateRoot, ownerUserID)
	workspaceRoot := appfs.UserWorkspaceRootAt(config.StateRoot, ownerUserID)
	projectValue, projectAccess := projectForPath(current, ownerUserID, cwd)
	if cwd == workspaceRoot {
		return preparedPolicy{}, false, errors.New("runtime cwd 必须是 owner workspace 的子目录")
	}
	if !pathWithin(cwd, workspaceRoot) && projectValue == nil {
		return preparedPolicy{}, false, errors.New("runtime cwd 授权已失效")
	}
	if pathWithin(cwd, workspaceRoot) {
		if filepath.Dir(cwd) != workspaceRoot {
			return preparedPolicy{}, false, errors.New("runtime cwd 不再是 owner workspace 的直接子目录")
		}
		if err = ensureWorkspaceBoundaryACL(cwd, identityValue, config.HostUID); err != nil {
			return preparedPolicy{}, false, fmt.Errorf("修复 runtime workspace ACL: %w", err)
		}
	}
	if projectValue != nil {
		if err = verifyOSGroup(projectValue.GroupName, projectValue.GID); err != nil {
			return preparedPolicy{}, false, err
		}
	}
	readRoots, writeRoots := baseRuntimePolicyRoots(ownerRoot, config)
	gids := []int{}
	if projectValue != nil {
		readRoots = append(readRoots, projectValue.Root)
		if projectAccess == projectAccessWrite {
			writeRoots = append(writeRoots, projectValue.Root)
			gids = append(gids, projectValue.GID)
		}
	}
	for _, root := range requestedReadRoots {
		root, err = canonicalExistingOrPendingPath(root)
		if err != nil {
			return preparedPolicy{}, false, err
		}
		projectValue, _ := projectForPath(current, ownerUserID, root)
		switch {
		case pathWithin(root, ownerRoot), pathWithinAny(root, config.ReadOnlyRoots):
		case pathWithinSharedTemp(root):
		case projectValue != nil:
			if err = verifyOSGroup(projectValue.GroupName, projectValue.GID); err != nil {
				return preparedPolicy{}, false, err
			}
		default:
			return preparedPolicy{}, false, errors.New("runtime read root 授权已失效")
		}
		readRoots = append(readRoots, root)
	}
	slices.Sort(gids)
	gids = slices.Compact(gids)
	environmentNames, err = normalizeEnvironmentNames(environmentNames)
	if err != nil {
		return preparedPolicy{}, false, err
	}
	return preparedPolicy{
		OwnerUserID:       ownerUserID,
		RuntimeKind:       runtimeKind,
		CWD:               cwd,
		ReadRoots:         compactPaths(readRoots),
		WriteRoots:        compactPaths(writeRoots),
		RuntimeReadRoots:  compactPaths(readRoots),
		RuntimeWriteRoots: compactPaths(writeRoots),
		EnvironmentNames:  environmentNames,
		Generation:        max(current.Generation, identityValue.Generation),
		Identity: preparedIdentity{
			Username:          identityValue.Username,
			UID:               identityValue.UID,
			PrivateGID:        identityValue.PrivateGID,
			SupplementaryGIDs: slices.Compact(gids),
			HomeDir:           identityValue.HomeDir,
			TempDir:           identityValue.TempDir,
		},
	}, false, nil
}

func projectForPath(
	current *registry,
	ownerUserID string,
	path string,
) (*project, string) {
	for _, value := range current.Projects {
		if value == nil || !pathWithin(path, value.Root) {
			continue
		}
		access := value.Members[ownerUserID]
		if access == projectAccessRead || access == projectAccessWrite {
			return value, access
		}
	}
	return nil, ""
}

func pathWithinAny(path string, roots []string) bool {
	for _, root := range roots {
		if pathWithin(path, root) {
			return true
		}
	}
	return false
}

// pathWithinSharedTemp 让显式 requestedReadRoots 与默认共享临时根保持同一授权语义。
func pathWithinSharedTemp(path string) bool {
	sharedTempRoot := appfs.RuntimeSharedTempRoot()
	return sharedTempRoot != "" && pathWithin(path, sharedTempRoot)
}

// baseRuntimePolicyRoots 统一 app/web runtime 的基础根；/tmp 是明确选择的
// 共享兼容区，私有临时数据仍应写入 identity.TempDir。
func baseRuntimePolicyRoots(
	ownerRoot string,
	config launcherConfig,
) ([]string, []string) {
	readRoots := append([]string{ownerRoot}, config.ReadOnlyRoots...)
	writeRoots := []string{ownerRoot}
	if sharedTempRoot := appfs.RuntimeSharedTempRoot(); sharedTempRoot != "" {
		readRoots = append(readRoots, sharedTempRoot)
		writeRoots = append(writeRoots, sharedTempRoot)
	}
	return readRoots, writeRoots
}

func runtimeRootForPolicy(config launcherConfig, ownerUserID string) string {
	return filepath.Join(appfs.UserDataRootAt(config.StateRoot, ownerUserID), "runtime")
}
