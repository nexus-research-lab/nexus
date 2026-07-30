package workspace

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type uploadFileOptions struct {
	dedupeRoots []string
}

func md5Hex(content []byte) string {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:])
}

func fileMatchesMD5(root *confinedfs.Root, path string, expectedMD5 string, expectedSize int64) (bool, error) {
	info, err := root.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() || info.Size() != expectedSize {
		return false, nil
	}
	actualMD5, err := fileMD5(root, path)
	if err != nil {
		return false, err
	}
	return actualMD5 == expectedMD5, nil
}

func fileMD5(root *confinedfs.Root, path string) (string, error) {
	file, err := root.OpenFileNoSymlink(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func findDuplicateUploadedFile(
	root *confinedfs.Root,
	normalizedPath string,
	expectedMD5 string,
	expectedSize int64,
	dedupeRoots []string,
) (string, bool, error) {
	dedupeRoot, ok := matchedUploadDedupeRoot(normalizedPath, dedupeRoots)
	if !ok {
		return "", false, nil
	}
	if _, err := root.Stat(dedupeRoot); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	var matchedPath string
	err := root.Walk(dedupeRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry == nil || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() || info.Size() != expectedSize {
			return nil
		}
		matched, err := fileMatchesMD5(root, path, expectedMD5, expectedSize)
		if err != nil {
			return err
		}
		if !matched {
			return nil
		}
		matchedPath = filepath.ToSlash(path)
		return filepath.SkipAll
	})
	if err != nil {
		return "", false, err
	}
	return matchedPath, matchedPath != "", nil
}

func matchedUploadDedupeRoot(normalizedPath string, dedupeRoots []string) (string, bool) {
	path := filepath.ToSlash(strings.Trim(strings.TrimSpace(normalizedPath), "/"))
	for _, root := range dedupeRoots {
		normalizedRoot := filepath.ToSlash(strings.Trim(strings.TrimSpace(root), "/"))
		if normalizedRoot == "" {
			continue
		}
		if path == normalizedRoot || strings.HasPrefix(path, normalizedRoot+"/") {
			return normalizedRoot, true
		}
	}
	return "", false
}
