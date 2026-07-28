// INPUT: owner Skill registry、Skill 目录和受控文件名。
// OUTPUT: 固定目录句柄上的读取、列举、写入与私有暂存目录。
// POS: Skill 服务访问 owner workspace 的唯一文件边界。
package skills

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const privateSkillStagingRoot = ".settings/skill-staging"

func openSkillRegistry(registryRoot string, create bool) (*confinedfs.Root, error) {
	boundaryRoot, relativeRoot, err := skillRegistryBoundary(registryRoot)
	if err != nil {
		return nil, err
	}
	if create {
		if err = os.MkdirAll(
			boundaryRoot,
			appfs.RuntimeCollaborativeDirectoryMode(0o700),
		); err != nil {
			return nil, err
		}
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if create {
		return root.OpenOrCreateRootNoSymlink(
			relativeRoot,
			appfs.RuntimeCollaborativeDirectoryMode(0o755),
		)
	}
	return root.OpenRootNoSymlink(relativeRoot)
}

// openSkillRegistryAt 在已经固定的 owner workspace 根中打开 Skill registry。
func openSkillRegistryAt(ownerRoot *confinedfs.Root, create bool) (*confinedfs.Root, error) {
	if ownerRoot == nil {
		return nil, errors.New("owner skill root is nil")
	}
	if create {
		return ownerRoot.OpenOrCreateRootNoSymlink(
			".agents/skills",
			appfs.RuntimeCollaborativeDirectoryMode(0o755),
		)
	}
	return ownerRoot.OpenRootNoSymlink(".agents/skills")
}

func skillRegistryBoundary(registryRoot string) (string, string, error) {
	registryRoot = filepath.Clean(strings.TrimSpace(registryRoot))
	if registryRoot == "" || registryRoot == "." ||
		filepath.Base(registryRoot) != "skills" ||
		filepath.Base(filepath.Dir(registryRoot)) != ".agents" {
		return "", "", errors.New("invalid owner skill registry root")
	}
	boundaryRoot := filepath.Dir(filepath.Dir(registryRoot))
	relativeRoot, err := filepath.Rel(boundaryRoot, registryRoot)
	if err != nil || relativeRoot == "." || relativeRoot == ".." ||
		strings.HasPrefix(relativeRoot, ".."+string(filepath.Separator)) {
		return "", "", errors.New("owner skill registry escapes boundary")
	}
	return boundaryRoot, filepath.ToSlash(relativeRoot), nil
}

func readSkillRegistryFile(registryRoot string, skillName string, fileName string) ([]byte, error) {
	if err := validateSkillName(skillName); err != nil {
		return nil, err
	}
	root, err := openSkillRegistry(registryRoot, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	skillRoot, err := root.OpenRootNoSymlink(skillName)
	if err != nil {
		return nil, err
	}
	defer skillRoot.Close()
	return readConfinedRegularFile(skillRoot, fileName)
}

func readSkillRegistryFileAt(
	ownerRoot *confinedfs.Root,
	skillName string,
	fileName string,
) ([]byte, error) {
	if err := validateSkillName(skillName); err != nil {
		return nil, err
	}
	registryRoot, err := openSkillRegistryAt(ownerRoot, false)
	if err != nil {
		return nil, err
	}
	defer registryRoot.Close()
	skillRoot, err := registryRoot.OpenRootNoSymlink(skillName)
	if err != nil {
		return nil, err
	}
	defer skillRoot.Close()
	return readConfinedRegularFile(skillRoot, fileName)
}

func readSkillDirectoryFile(skillDir string, fileName string) ([]byte, error) {
	registryRoot := filepath.Dir(filepath.Clean(strings.TrimSpace(skillDir)))
	return readSkillRegistryFile(registryRoot, filepath.Base(skillDir), fileName)
}

func readSkillDirectoryFileAt(
	ownerRoot *confinedfs.Root,
	skillName string,
	fileName string,
) ([]byte, error) {
	return readSkillRegistryFileAt(ownerRoot, skillName, fileName)
}

func readSkillFileAtOwnerPath(
	ownerRoot *confinedfs.Root,
	skillPath string,
	fileName string,
) ([]byte, error) {
	relative, err := relativeSkillPath(ownerRoot, skillPath)
	if err != nil {
		return nil, err
	}
	if filepath.ToSlash(filepath.Dir(relative)) != ".agents/skills" {
		return nil, errors.New("skill path is outside owner registry")
	}
	return readSkillRegistryFileAt(ownerRoot, filepath.Base(relative), fileName)
}

func readSkillRegistryDirectories(registryRoot string) (*confinedfs.Root, []fs.DirEntry, error) {
	root, err := openSkillRegistry(registryRoot, true)
	if err != nil {
		return nil, nil, err
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		root.Close()
		return nil, nil, err
	}
	return root, entries, nil
}

func readSkillRegistryDirectoriesAt(
	ownerRoot *confinedfs.Root,
	create bool,
) (*confinedfs.Root, []fs.DirEntry, error) {
	root, err := openSkillRegistryAt(ownerRoot, create)
	if err != nil {
		return nil, nil, err
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		root.Close()
		return nil, nil, err
	}
	return root, entries, nil
}

func readConfinedDirectoryEntries(rootPath string) ([]fs.DirEntry, error) {
	root, err := confinedfs.Open(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return fs.ReadDir(root.FS(), ".")
}

func createPrivateSkillStaging(boundaryRoot string) (string, error) {
	if err := os.MkdirAll(
		boundaryRoot,
		appfs.RuntimeCollaborativeDirectoryMode(0o700),
	); err != nil {
		return "", err
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return "", err
	}
	defer root.Close()
	relativePath, err := root.MkdirTemp(
		privateSkillStagingRoot,
		".external-skill-",
		0o700,
	)
	if err != nil {
		return "", err
	}
	return filepath.Join(boundaryRoot, filepath.FromSlash(relativePath)), nil
}

// relativeSkillPath 将已知 owner 根内的绝对路径转换为受控相对路径。
func relativeSkillPath(root *confinedfs.Root, targetPath string) (string, error) {
	if root == nil {
		return "", errors.New("owner skill root is nil")
	}
	relative, err := filepath.Rel(
		filepath.Clean(root.Name()),
		filepath.Clean(strings.TrimSpace(targetPath)),
	)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("skill path escapes owner workspace")
	}
	return filepath.ToSlash(relative), nil
}

func readConfinedRegularFile(root *confinedfs.Root, fileName string) ([]byte, error) {
	file, err := root.OpenFileNoSymlink(fileName, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func writeSkillDirectoryFile(
	skillDir string,
	fileName string,
	payload []byte,
	mode os.FileMode,
) error {
	root, err := confinedfs.Open(skillDir)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteFileAtomic(fileName, payload, mode)
}
