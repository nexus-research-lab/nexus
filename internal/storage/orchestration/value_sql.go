// INPUT: protocol JSON 字段、nullable scalar 与调用方时间。
// OUTPUT: SQLite/PostgreSQL 可写入、可扫描的 SQL value。
// POS: Orchestration Repository 的值归一化边界。
package orchestration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func timeOr(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback.UTC()
	}
	return value.UTC()
}

func versionOrOne(value int64) int64 {
	if value <= 0 {
		return 1
	}
	return value
}

func marshalJSON(value any, empty any) (string, error) {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Map && reflect.ValueOf(value).IsNil()) {
		value = empty
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func marshalMap(value map[string]any) (string, error) {
	return marshalJSON(value, map[string]any{})
}

func marshalSlice[T any](value []T) (string, error) {
	return marshalJSON(value, []T{})
}

func parseMap(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("decode JSON object: %w", err)
	}
	return value, nil
}

func parseSlice[T any](raw string) ([]T, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var value []T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("decode JSON array: %w", err)
	}
	return value, nil
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
