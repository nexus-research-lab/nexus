// INPUT: database/sql timestamp values whose aggregate expression may erase the declared column type.
// OUTPUT: normalized optional UTC time across SQLite and PostgreSQL drivers.
// POS: shared SQL value boundary for deadline index queries; ordinary typed row scanners stay domain-local.
package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var databaseTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
}

// NullableTime converts the raw value produced by database/sql aggregate and
// scalar expressions. SQLite commonly returns a string because MIN/CASE loses
// the source column declaration; pgx normally returns time.Time.
func NullableTime(value any) (*time.Time, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case time.Time:
		result := typed.UTC()
		return &result, nil
	case string:
		return parseDatabaseTime(typed)
	case []byte:
		return parseDatabaseTime(string(typed))
	case int64:
		result := time.Unix(typed, 0).UTC()
		return &result, nil
	default:
		return nil, fmt.Errorf("unsupported database time value %T", value)
	}
}

func parseDatabaseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range databaseTimeLayouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			result := parsed.UTC()
			return &result, nil
		}
	}
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		result := time.Unix(unixSeconds, 0).UTC()
		return &result, nil
	}
	return nil, fmt.Errorf("unsupported database time value %q", value)
}
