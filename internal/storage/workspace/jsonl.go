package workspace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *SessionFileStore) appendJSONL(path string, row map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".overlay-rewrite-*.jsonl")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	writer := bufio.NewWriter(file)
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			_ = file.Close()
			return err
		}
		if _, err = fmt.Fprintf(writer, "%s\n", payload); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SessionFileStore) readJSONL(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
