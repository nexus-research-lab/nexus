package workspaceisolation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// OwnerProcessReaper 通过 root-owned launcher 回收 owner cgroup 中的全部
// runtime 进程。它只在 enforce + Linux 下生效，audit/off 不改变既有行为。
type OwnerProcessReaper struct {
	Mode         Mode
	LauncherPath string
}

// ReapOwnerProcesses 实现 runtime.OwnerProcessReaper。
func (r OwnerProcessReaper) ReapOwnerProcesses(ctx context.Context, ownerUserID string) error {
	if r.Mode != ModeEnforce || runtime.GOOS != "linux" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || ownerUserID == "." || ownerUserID == ".." ||
		filepath.Base(ownerUserID) != ownerUserID ||
		strings.ContainsAny(ownerUserID, `/\`) {
		return errors.New("owner user id 不能安全映射为 cgroup")
	}
	launcherPath := filepath.Clean(strings.TrimSpace(r.LauncherPath))
	if launcherPath == "." || !filepath.IsAbs(launcherPath) {
		return errors.New("runtime launcher path is not configured")
	}
	return runLauncherManagementCommand(ctx, launcherPath, "stop-user", "--owner", ownerUserID)
}

func runtimeProcessSignalHandler(
	launcherPath string,
	ownerUserID string,
) agentclient.ProcessSignalHandler {
	launcherPath = filepath.Clean(strings.TrimSpace(launcherPath))
	ownerUserID = strings.TrimSpace(ownerUserID)
	return func(processID int, signal agentclient.ProcessSignal) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return runLauncherManagementCommand(
			ctx,
			launcherPath,
			"signal-process",
			"--owner", ownerUserID,
			"--pid", strconv.Itoa(processID),
			"--signal", string(signal),
		)
	}
}

func runLauncherManagementCommand(
	ctx context.Context,
	launcherPath string,
	arguments ...string,
) error {
	command := exec.CommandContext(ctx, launcherPath, arguments...)
	command.Env = []string{
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C",
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("runtime launcher: %s: %w", detail, err)
		}
		return fmt.Errorf("runtime launcher: %w", err)
	}
	return nil
}
