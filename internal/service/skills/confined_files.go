package skills

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const privateSkillStagingRoot = ".settings/skill-staging"

func openSkillRegistry(registryRoot string, create bool) (*confinedfs.Root, string, error) {
	boundaryRoot, relativeRoot, err := skillRegistryBoundary(registryRoot)
	if err != nil {
		return nil, "", err
	}
	if create {
		if err = os.MkdirAll(
			boundaryRoot,
			appfs.RuntimeCollaborativeDirectoryMode(0o700),
		); err != nil {
			return nil, "", err
		}
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return nil, "", err
	}
	if create {
		if err = root.MkdirAll(relativeRoot, appfs.RuntimeCollaborativeDirectoryMode(0o755)); err != nil {
			root.Close()
			return nil, "", err
		}
	}
	return root, relativeRoot, nil
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
	root, relativeRoot, err := openSkillRegistry(registryRoot, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	relativePath := filepath.ToSlash(filepath.Join(relativeRoot, skillName, fileName))
	info, err := root.Lstat(relativePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("skill file is not a regular confined file")
	}
	return root.ReadFile(relativePath)
}

func readSkillDirectoryFile(skillDir string, fileName string) ([]byte, error) {
	registryRoot := filepath.Dir(filepath.Clean(strings.TrimSpace(skillDir)))
	return readSkillRegistryFile(registryRoot, filepath.Base(skillDir), fileName)
}

func readSkillRegistryDirectories(registryRoot string) (*confinedfs.Root, string, []fs.DirEntry, error) {
	root, relativeRoot, err := openSkillRegistry(registryRoot, true)
	if err != nil {
		return nil, "", nil, err
	}
	entries, err := fs.ReadDir(root.FS(), relativeRoot)
	if err != nil {
		root.Close()
		return nil, "", nil, err
	}
	return root, relativeRoot, entries, nil
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
	if err = root.MkdirAll(privateSkillStagingRoot, 0o700); err != nil {
		return "", err
	}
	for attempt := 0; attempt < 16; attempt++ {
		var random [12]byte
		if _, err = rand.Read(random[:]); err != nil {
			return "", err
		}
		relativePath := filepath.ToSlash(filepath.Join(
			privateSkillStagingRoot,
			".external-skill-"+hex.EncodeToString(random[:]),
		))
		if err = root.Mkdir(relativePath, 0o700); errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return filepath.Join(boundaryRoot, filepath.FromSlash(relativePath)), nil
	}
	return "", errors.New("unable to allocate private skill staging directory")
}
