// INPUT: 已验证 owner、round lease 与宿主签发的 Automation capability。
// OUTPUT: owner 私有 runtime/tmp 下的单 round、0600 JSON 输入槽及安全清理函数。
// POS: Automation CLI 文件输入的宿主生命周期边界；模型不能选择文件名或跨 round 复用路径。
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const automationRuntimeInputFileName = "input.json"
const automationRuntimeInputRetention = 24 * time.Hour

func prepareAutomationRuntimeInput(
	ownerUserID string,
	roundID string,
	capability string,
) (string, func(), error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	roundID = strings.TrimSpace(roundID)
	capability = strings.TrimSpace(capability)
	if ownerUserID == "" || roundID == "" || capability == "" {
		return "", nil, errors.New("Automation input staging 缺少 owner、round 或 capability")
	}
	if err := appfs.EnsureUserRuntimeLayout(ownerUserID); err != nil {
		return "", nil, err
	}
	runtimeRootPath := appfs.UserRuntimeRoot(ownerUserID)
	runtimeRoot, err := confinedfs.Open(runtimeRootPath)
	if err != nil {
		return "", nil, err
	}
	defer runtimeRoot.Close()

	sum := sha256.Sum256([]byte(capability + "\x00" + roundID))
	directoryName := hex.EncodeToString(sum[:16])
	baseDirectory := filepath.ToSlash(filepath.Join("tmp", "automation-inputs"))
	baseRoot, err := runtimeRoot.OpenOrCreateRootNoSymlink(baseDirectory, 0o700)
	if err != nil {
		return "", nil, err
	}
	defer baseRoot.Close()
	cleanupStaleAutomationRuntimeInputs(baseRoot, directoryName, time.Now())
	stagingRoot, err := baseRoot.OpenOrCreateRootNoSymlink(directoryName, 0o700)
	if err != nil {
		return "", nil, err
	}
	defer stagingRoot.Close()
	if _, err = stagingRoot.Stat(automationRuntimeInputFileName); errors.Is(err, os.ErrNotExist) {
		if err = stagingRoot.WriteFileAtomic(automationRuntimeInputFileName, []byte("{}\n"), 0o600); err != nil {
			return "", nil, err
		}
	} else if err != nil {
		return "", nil, err
	} else {
		file, openErr := stagingRoot.OpenFileNoSymlink(automationRuntimeInputFileName, os.O_RDONLY, 0)
		if openErr != nil {
			return "", nil, openErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return "", nil, closeErr
		}
	}
	directory := filepath.ToSlash(filepath.Join(baseDirectory, directoryName))
	inputPath := filepath.Join(runtimeRootPath, filepath.FromSlash(directory), automationRuntimeInputFileName)
	cleanup := func() {
		root, openErr := confinedfs.Open(runtimeRootPath)
		if openErr != nil {
			return
		}
		defer root.Close()
		_ = root.RemoveAll(directory)
	}
	return inputPath, cleanup, nil
}

func cleanupStaleAutomationRuntimeInputs(root *confinedfs.Root, keep string, now time.Time) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return
	}
	cutoff := now.Add(-automationRuntimeInputRetention)
	for _, entry := range entries {
		if entry.Name() == keep {
			continue
		}
		info, statErr := root.Lstat(entry.Name())
		if statErr != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = root.RemoveAll(entry.Name())
	}
}
