package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) syncWorkspacePath(currentPath string, targetPath string) error {
	source := strings.TrimSpace(currentPath)
	target := strings.TrimSpace(targetPath)
	if source == "" || target == "" || source == target {
		if target == "" {
			return nil
		}
		return os.MkdirAll(target, 0o755)
	}
	sourceInfo, err := os.Stat(source)
	if os.IsNotExist(err) {
		return os.MkdirAll(target, 0o755)
	} else if err != nil {
		return err
	}
	if targetInfo, err := os.Stat(target); err == nil {
		if os.SameFile(sourceInfo, targetInfo) {
			return renameWorkspacePath(source, target)
		}
		return errors.New("目标工作区目录已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return renameWorkspacePath(source, target)
}

func sameWorkspacePath(left string, right string) (bool, error) {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false, nil
	}
	if filepath.Clean(left) == filepath.Clean(right) {
		return true, nil
	}
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, nil
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, nil
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

func renameWorkspacePath(source string, target string) error {
	if filepath.Clean(source) == filepath.Clean(target) {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.Rename(source, target); err == nil {
		return nil
	}

	parent := filepath.Dir(target)
	temporaryPath := filepath.Join(parent, "."+filepath.Base(target)+".rename-"+NewAgentID())
	if err := os.Rename(source, temporaryPath); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		_ = os.Rename(temporaryPath, source)
		return err
	}
	return nil
}
