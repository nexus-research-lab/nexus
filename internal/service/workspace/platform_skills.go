// INPUT: 产品随附的 skills 目录与全局配置根。
// OUTPUT: nxs/Claude 共用的全局平台 Skill 兼容库。
// POS: workspace 与 runtime 装配之间的平台 Skill 同步边界。
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

var platformSkillLibraryState struct {
	sync.Mutex
	root        string
	fingerprint string
}

const platformSkillManifestName = ".platform-skill-manifest"

// EnsurePlatformSkillLibrary 将产品随附 Skill 同步到全局兼容根。
//
// 同步只发生一次内容变化时，Agent workspace 不再持有 Skill 副本；nxs 和
// Claude 通过两个兼容入口读取同一份平台文件，更新平台文件后下一次同步即可生效。
func EnsurePlatformSkillLibrary() error {
	sourceRoot := filepath.Join(appfs.Root(), "skills")
	if _, err := os.Stat(sourceRoot); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	fingerprint, err := platformSkillFingerprint(sourceRoot)
	if err != nil {
		return err
	}
	targetRoot := appfs.PlatformSkillRoot()

	platformSkillLibraryState.Lock()
	defer platformSkillLibraryState.Unlock()
	if platformSkillLibraryState.root == targetRoot && platformSkillLibraryState.fingerprint == fingerprint && platformSkillLibraryReady(targetRoot, fingerprint) {
		return nil
	}
	if err = replacePlatformSkillLibrary(sourceRoot, targetRoot, fingerprint); err != nil {
		return err
	}
	platformSkillLibraryState.root = targetRoot
	platformSkillLibraryState.fingerprint = fingerprint
	return nil
}

func platformSkillFingerprint(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(relative)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(fmt.Sprintf("%o", info.Mode().Perm())))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(content)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func platformSkillLibraryReady(root string, fingerprint string) bool {
	manifest, err := os.ReadFile(filepath.Join(root, platformSkillManifestName))
	if err != nil || string(manifest) != fingerprint+"\n" {
		return false
	}
	for _, path := range []string{
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, ".claude", "skills"),
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func replacePlatformSkillLibrary(sourceRoot string, targetRoot string, fingerprint string) error {
	if err := os.MkdirAll(filepath.Dir(targetRoot), 0o755); err != nil {
		return err
	}
	temporaryRoot, err := os.MkdirTemp(filepath.Dir(targetRoot), ".platform-skills-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	if err := os.MkdirAll(filepath.Join(temporaryRoot, ".agents", "skills"), 0o755); err != nil {
		return err
	}
	if err := copyPlatformDirectory(sourceRoot, filepath.Join(temporaryRoot, ".agents", "skills")); err != nil {
		return err
	}
	claudeSkillsRoot := filepath.Join(temporaryRoot, ".claude", "skills")
	if err := ensureRelativeSymlink(claudeSkillsRoot, filepath.Join("..", ".agents", "skills")); err != nil {
		if copyErr := copyPlatformDirectory(filepath.Join(temporaryRoot, ".agents", "skills"), claudeSkillsRoot); copyErr != nil {
			return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, copyErr)
		}
	}
	if err := os.WriteFile(filepath.Join(temporaryRoot, platformSkillManifestName), []byte(fingerprint+"\n"), 0o644); err != nil {
		return err
	}
	if err := replaceDirectory(temporaryRoot, targetRoot); err != nil {
		return err
	}
	return nil
}

func copyPlatformDirectory(sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return err
	}
	return filepath.Walk(sourceRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(targetRoot, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			_ = source.Close()
			return err
		}
		if _, err = io.Copy(destination, source); err != nil {
			_ = source.Close()
			_ = destination.Close()
			return err
		}
		if err = source.Close(); err != nil {
			_ = destination.Close()
			return err
		}
		if err = destination.Close(); err != nil {
			return err
		}
		return os.Chmod(target, info.Mode())
	})
}

func replaceDirectory(sourceRoot string, targetRoot string) error {
	if err := os.MkdirAll(filepath.Dir(targetRoot), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(targetRoot); os.IsNotExist(err) {
		return os.Rename(sourceRoot, targetRoot)
	} else if err != nil {
		return err
	}
	backupRoot := targetRoot + ".old"
	if err := os.RemoveAll(backupRoot); err != nil {
		return err
	}
	if err := os.Rename(targetRoot, backupRoot); err != nil {
		return err
	}
	if err := os.Rename(sourceRoot, targetRoot); err != nil {
		_ = os.Rename(backupRoot, targetRoot)
		return err
	}
	return os.RemoveAll(backupRoot)
}
