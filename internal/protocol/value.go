package protocol

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Int64FromAny 统一解码 JSON payload 与进程内协议对象中的整数值。
// 非数字、溢出和非有限浮点数返回 0，调用方无需复制不完整的类型分支。
func Int64FromAny(value any) int64 {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case json.Number:
		return int64FromJSONNumber(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
		return 0
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		unsigned := reflected.Uint()
		if unsigned <= math.MaxInt64 {
			return int64(unsigned)
		}
	case reflect.Float32, reflect.Float64:
		return int64FromFloat(reflected.Float())
	}
	return 0
}

func int64FromJSONNumber(value json.Number) int64 {
	if parsed, err := value.Int64(); err == nil {
		return parsed
	}
	parsed, err := strconv.ParseFloat(value.String(), 64)
	if err != nil {
		return 0
	}
	return int64FromFloat(parsed)
}

func int64FromFloat(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value >= math.MaxInt64 || value < math.MinInt64 {
		return 0
	}
	return int64(value)
}
