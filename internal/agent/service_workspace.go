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
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return os.MkdirAll(target, 0o755)
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("目标工作区目录已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Rename(source, target)
}
