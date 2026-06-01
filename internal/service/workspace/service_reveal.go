package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// RevealFileInFolder 在本机文件管理器中定位 workspace 文件。
func (s *Service) RevealFileInFolder(ctx context.Context, agentID string, relativePath string) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop") {
		return "", ErrLocalFileRevealUnavailable
	}
	filePath, _, err := s.GetFileForDownload(ctx, agentID, relativePath)
	if err != nil {
		return "", err
	}
	if err = revealFileInFolder(ctx, filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func revealFileInFolder(ctx context.Context, filePath string) error {
	switch runtime.GOOS {
	case "darwin":
		return runFileManagerCommand(ctx, "/usr/bin/open", "-R", filePath)
	case "windows":
		return runFileManagerCommand(ctx, "explorer", "/select,"+filePath)
	default:
		// Linux 文件管理器缺少统一的选中文件协议，退回到打开所在目录。
		return runFileManagerCommand(ctx, "xdg-open", filepath.Dir(filePath))
	}
}

func runFileManagerCommand(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return err
	}
	return fmt.Errorf("%s: %w", detail, err)
}
