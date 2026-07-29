package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

type runtimePermissionRepair struct {
	mu sync.Mutex
}

func newRuntimePermissionRepair() *runtimePermissionRepair {
	return &runtimePermissionRepair{}
}

// withRuntimePermissionRepair 在 owner transcript 被 runtime 重新设为 0600
// 时，通过受信 launcher 修复 ACL 后重试一次。宿主只在收到 EACCES 时触发，
// 不把 root 权限扩展到普通文件读写路径。
func withRuntimePermissionRepair[T any](
	s *AgentHistoryStore,
	operation func() (T, error),
) (T, error) {
	result, err := operation()
	if err == nil || s == nil || strings.TrimSpace(s.ownerUserID) == "" ||
		!errors.Is(err, os.ErrPermission) {
		return result, err
	}
	if repairErr := s.repairOwnerRuntimePermissions(); repairErr != nil {
		return result, errors.Join(err, repairErr)
	}
	return operation()
}

func (s *AgentHistoryStore) repairOwnerRuntimePermissions() error {
	if s == nil || strings.TrimSpace(s.ownerUserID) == "" ||
		!appfs.RuntimeIsolationEnforced() {
		return nil
	}
	ownerUserID := strings.TrimSpace(s.ownerUserID)
	if appfs.UserPathSegment(ownerUserID) != ownerUserID ||
		ownerUserID == "." || ownerUserID == ".." {
		return errors.New("owner user id 无效")
	}
	launcherPath := filepath.Clean(strings.TrimSpace(os.Getenv("NEXUS_RUNTIME_LAUNCHER_PATH")))
	if launcherPath == "." || !filepath.IsAbs(launcherPath) {
		return errors.New("runtime launcher path is not configured")
	}
	if s.runtimeRepair == nil {
		s.runtimeRepair = newRuntimePermissionRepair()
	}

	s.runtimeRepair.mu.Lock()
	defer s.runtimeRepair.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		launcherPath,
		"repair-user",
		"--owner",
		ownerUserID,
	)
	command.Env = runtimePermissionRepairEnvironment()
	command.Stdout = io.Discard
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("repair owner runtime permissions: %s", detail)
	}
	return nil
}

func runtimePermissionRepairEnvironment() []string {
	environment := []string{
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C.UTF-8",
	}
	if value := strings.TrimSpace(os.Getenv("NEXUS_RUNTIME_ISOLATION_CONFIG")); value != "" {
		environment = append(environment, "NEXUS_RUNTIME_ISOLATION_CONFIG="+value)
	}
	return environment
}
