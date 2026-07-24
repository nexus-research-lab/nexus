//go:build linux

package runtimeidentity

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func ensureProject(
	config launcherConfig,
	current *registry,
	projectID string,
	root string,
) (*project, bool, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || len(projectID) > 128 {
		return nil, false, errors.New("project id 无效")
	}
	root, err := canonicalExistingOrPendingPath(root)
	if err != nil {
		return nil, false, err
	}
	sharedRoot, err := canonicalExistingOrPendingPath(filepath.Join(config.StateRoot, "shared-workspaces"))
	if err != nil {
		return nil, false, err
	}
	sharedFD, err := ensureDirectoryNoSymlink(sharedRoot, 0o751)
	if err != nil {
		return nil, false, err
	}
	if err = unix.Fchown(sharedFD, 0, config.HostGID); err != nil {
		_ = unix.Close(sharedFD)
		return nil, false, err
	}
	if err = clearPOSIXACLFD(sharedFD, true); err != nil {
		_ = unix.Close(sharedFD)
		return nil, false, err
	}
	if err = unix.Fchmod(sharedFD, 0o751); err != nil {
		_ = unix.Close(sharedFD)
		return nil, false, err
	}
	_ = unix.Close(sharedFD)
	if filepath.Dir(root) != sharedRoot {
		return nil, false, errors.New("共享 project 必须是 state_root/shared-workspaces 的直接子目录")
	}
	for existingID, existing := range current.Projects {
		if existing == nil || existingID == projectID {
			continue
		}
		if pathWithin(root, existing.Root) || pathWithin(existing.Root, root) {
			return nil, false, errors.New("共享 project 不能嵌套，避免 ACL policy 歧义")
		}
	}
	if existing := current.Projects[projectID]; existing != nil {
		if existing.Root != root {
			return nil, false, errors.New("project id 已绑定到其他路径")
		}
		if err = ensureOSGroup(existing.GroupName, existing.GID); err != nil {
			return nil, false, err
		}
		if err = ensureProjectRootDirectory(existing.Root, existing.GID); err != nil {
			return nil, false, err
		}
		if err = applyProjectTreeACL(config, existing, current); err != nil {
			return nil, false, fmt.Errorf("修复 project ACL: %w", err)
		}
		return existing, false, nil
	}
	groupName := stableAccountName("nxp", projectID)
	gid, recovered, err := recoverStableGroupID(config, current, groupName)
	if err != nil {
		return nil, false, err
	}
	if !recovered {
		gid, err = allocateNumericID(config, current)
		if err != nil {
			return nil, false, err
		}
	}
	nextGeneration := current.Generation + 1
	created := &project{
		ProjectID:  projectID,
		GroupName:  groupName,
		GID:        gid,
		Root:       root,
		Members:    map[string]string{},
		Generation: nextGeneration,
	}
	if err = ensureOSGroup(created.GroupName, created.GID); err != nil {
		return nil, false, err
	}
	if err = ensureProjectRootDirectory(created.Root, created.GID); err != nil {
		return nil, false, err
	}
	current.Generation = nextGeneration
	current.Projects[projectID] = created
	if err = applyProjectTreeACL(config, created, current); err != nil {
		delete(current.Projects, projectID)
		return nil, false, fmt.Errorf("应用 project ACL: %w", err)
	}
	return created, true, nil
}

func grantProject(
	config launcherConfig,
	current *registry,
	projectID string,
	ownerUserID string,
	access string,
) (bool, error) {
	value := current.Projects[strings.TrimSpace(projectID)]
	if value == nil {
		return false, errors.New("project 不存在")
	}
	access = strings.ToLower(strings.TrimSpace(access))
	switch access {
	case projectAccessNone, projectAccessRead, projectAccessWrite:
	default:
		return false, errors.New("project access 只能是 read、write 或 none")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return false, errors.New("owner_user_id 无效")
	}
	if access == projectAccessNone {
		if _, exists := value.Members[ownerUserID]; !exists {
			return false, nil
		}
	}
	if err := ensureOSGroup(value.GroupName, value.GID); err != nil {
		return false, err
	}
	if err := ensureProjectRootDirectory(value.Root, value.GID); err != nil {
		return false, err
	}
	identityChanged := false
	if access != projectAccessNone {
		_, changed, ensureErr := ensureIdentity(config, current, ownerUserID)
		if ensureErr != nil {
			return false, ensureErr
		}
		identityChanged = changed
	}
	switch access {
	case projectAccessNone:
		delete(value.Members, ownerUserID)
	case projectAccessRead, projectAccessWrite:
		if value.Members[ownerUserID] == access {
			return identityChanged, nil
		}
		value.Members[ownerUserID] = access
	}
	current.Generation++
	value.Generation = current.Generation
	if err := applyProjectTreeACL(config, value, current); err != nil {
		return false, err
	}
	return true, nil
}

func ensureProjectRootDirectory(root string, gid int) error {
	fd, err := ensureDirectoryNoSymlink(root, 0o770)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	if err = unix.Fchown(fd, 0, gid); err != nil {
		return err
	}
	return unix.Fchmod(fd, unix.S_ISGID|0o770)
}
