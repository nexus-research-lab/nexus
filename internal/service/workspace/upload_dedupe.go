package workspace

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type uploadFileOptions struct {
	dedupeRoots []string
}

func md5Hex(content []byte) string {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:])
}

func fileMatchesMD5(path string, expectedMD5 string, expectedSize int64) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() || info.Size() != expectedSize {
		return false, nil
	}
	actualMD5, err := fileMD5(path)
	if err != nil {
		return false, err
	}
	return actualMD5 == expectedMD5, nil
}

func fileMD5(path string) (string, error) {
	file, err := os.Open(path)
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
	root string,
	normalizedPath string,
	expectedMD5 string,
	expectedSize int64,
	dedupeRoots []string,
) (string, bool, error) {
	dedupeRoot, ok := matchedUploadDedupeRoot(normalizedPath, dedupeRoots)
	if !ok {
		return "", false, nil
	}
	dedupeRootPath, _, err := resolveWorkspacePath(root, dedupeRoot)
	if err != nil {
		return "", false, err
	}
	if _, err = os.Stat(dedupeRootPath); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	var matchedPath string
	err = filepath.Walk(dedupeRootPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || info.IsDir() || info.Size() != expectedSize {
			return nil
		}
		matched, err := fileMatchesMD5(path, expectedMD5, expectedSize)
		if err != nil {
			return err
		}
		if !matched {
			return nil
		}
		relativePath, err := filepath.Rel(filepath.Clean(root), path)
		if err != nil {
			return err
		}
		matchedPath = filepath.ToSlash(relativePath)
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
