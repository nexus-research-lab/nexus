package workspace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func (s *SessionFileStore) appendJSONL(path string, row map[string]any) error {
	root, relative, err := s.openStorePath(path, true)
	if err != nil {
		return err
	}
	defer root.Close()
	return appendJSONLAtRoot(root, relative, row)
}

func (s *SessionFileStore) appendJSONLAt(rootPath string, path string, row map[string]any) error {
	if ownerUserID := strings.TrimSpace(s.ownerUserID); ownerUserID != "" {
		return s.appendOwnerWorkspaceJSONL(ownerUserID, rootPath, path, row)
	}
	root, relative, err := relativeStorePath(rootPath, path)
	if err != nil {
		return err
	}
	defer root.Close()
	return appendJSONLAtRoot(root, relative, row)
}

func appendJSONLAtRoot(root *confinedfs.Root, relative string, row map[string]any) error {
	return appendJSONLAtRootWithMode(
		root,
		relative,
		row,
		storageFileMode(0o644),
	)
}

func appendJSONLAtRootWithMode(
	root *confinedfs.Root,
	relative string,
	row map[string]any,
	mode os.FileMode,
) error {
	if err := root.MkdirAll(filepath.Dir(relative), storageDirectoryMode()); err != nil {
		return err
	}

	file, err := root.OpenFileNoSymlink(
		relative,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		mode,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(file, "%s\n", payload); err != nil {
		return err
	}
	return nil
}

func (s *SessionFileStore) replaceJSONL(path string, rows []map[string]any) error {
	root, relative, err := s.openStorePath(path, true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.MkdirAll(filepath.Dir(relative), storageDirectoryMode()); err != nil {
		return err
	}

	var builder strings.Builder
	writer := bufio.NewWriter(&builder)
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(writer, "%s\n", payload); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	return root.WriteFileAtomic(relative, []byte(builder.String()), storageFileMode(0o644))
}

func (s *SessionFileStore) replaceJSONLAt(rootPath string, path string, rows []map[string]any) error {
	if ownerUserID := strings.TrimSpace(s.ownerUserID); ownerUserID != "" {
		return s.replaceOwnerWorkspaceJSONL(ownerUserID, rootPath, path, rows)
	}
	root, relative, err := relativeStorePath(rootPath, path)
	if err != nil {
		return err
	}
	defer root.Close()
	var builder strings.Builder
	writer := bufio.NewWriter(&builder)
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(writer, "%s\n", payload); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	return root.WriteFileAtomic(relative, []byte(builder.String()), storageFileMode(0o644))
}

func (s *SessionFileStore) readJSONL(path string) ([]map[string]any, error) {
	root, relative, err := s.openStorePath(path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readJSONLAtRoot(root, relative)
}

func (s *SessionFileStore) readJSONLAt(rootPath string, path string) ([]map[string]any, error) {
	if ownerUserID := strings.TrimSpace(s.ownerUserID); ownerUserID != "" {
		return s.readOwnerWorkspaceJSONL(ownerUserID, rootPath, path)
	}
	root, relative, err := relativeStorePathWithCreate(rootPath, path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readJSONLAtRoot(root, relative)
}

func readJSONLAtRoot(root *confinedfs.Root, relative string) ([]map[string]any, error) {
	file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readJSONLFile(file)
}

func readJSONLFile(file *os.File) ([]map[string]any, error) {
	reader := bufio.NewScanner(file)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	rows := make([]map[string]any, 0)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		normalized, ok := normalizeDecodedJSONValue(item).(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, normalized)
	}
	return rows, reader.Err()
}

func normalizeDecodedJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeDecodedJSONValue(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeDecodedJSONValue(item))
		}
		return result
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return value
	}
}
