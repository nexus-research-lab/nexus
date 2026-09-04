// INPUT: Operation.InputSchema 的 portable JSON Schema 子集，以及 MCP/CLI 解码后的 JSON 输入。
// OUTPUT: 在领域 handler 前完成 required/type/enum/pattern/closed-object/array-boundary 校验的稳定错误。
// POS: Goal 与 Execution command 共用的模型输入边界；复刻原生 MCP provider 的 schema 前置约束。
package command

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
)

// ValidateInput validates the portable schema dialect emitted by runtime
// command operations. Domain handlers remain responsible for stateful and
// cross-field authorization rules.
func ValidateInput(schema map[string]any, input map[string]any) error {
	if input == nil {
		input = map[string]any{}
	}
	return validateSchemaValue("$", input, schema)
}

func validateSchemaValue(path string, value any, schema map[string]any) error {
	if schema == nil {
		return nil
	}
	switch schemaType, _ := schema["type"].(string); schemaType {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("at %s must be an object", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, required := range schemaStrings(schema["required"]) {
			if _, exists := object[required]; !exists {
				return fmt.Errorf("at %s.%s is required", path, required)
			}
		}
		if closed, declared := schema["additionalProperties"].(bool); declared && !closed {
			for name := range object {
				if _, known := properties[name]; !known {
					return fmt.Errorf("at %s.%s is not allowed", path, name)
				}
			}
		}
		for name, childValue := range object {
			childSchema, ok := properties[name].(map[string]any)
			if !ok {
				continue
			}
			if err := validateSchemaValue(path+"."+name, childValue, childSchema); err != nil {
				return err
			}
		}
	case "array":
		array, ok := schemaArray(value)
		if !ok {
			return fmt.Errorf("at %s must be an array", path)
		}
		if maximum, ok := schemaInteger(schema["maxItems"]); ok && len(array) > maximum {
			return fmt.Errorf("at %s must contain at most %d items", path, maximum)
		}
		itemSchema, _ := schema["items"].(map[string]any)
		for index, item := range array {
			if err := validateSchemaValue(fmt.Sprintf("%s[%d]", path, index), item, itemSchema); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("at %s must be a string", path)
		}
		if allowed := schemaStrings(schema["enum"]); len(allowed) > 0 && !containsString(allowed, text) {
			return fmt.Errorf("at %s must be one of %s", path, strings.Join(allowed, ", "))
		}
		if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("at %s uses invalid schema pattern: %v", path, err)
			}
			if !compiled.MatchString(text) {
				return fmt.Errorf("at %s does not match required pattern %q", path, pattern)
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("at %s must be a boolean", path)
		}
	case "integer":
		if !isJSONInteger(value) {
			return fmt.Errorf("at %s must be an integer", path)
		}
	}
	return nil
}

func schemaStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}

func schemaArray(value any) ([]any, bool) {
	if typed, ok := value.([]any); ok {
		return typed, true
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Slice {
		return nil, false
	}
	result := make([]any, reflected.Len())
	for index := range result {
		result[index] = reflected.Index(index).Interface()
	}
	return result, true
}

func schemaInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if math.Trunc(typed) == typed {
			return int(typed), true
		}
	}
	return 0, false
}

func isJSONInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return !math.IsNaN(typed) && !math.IsInf(typed, 0) && math.Trunc(typed) == typed
	case float32:
		return !float32IsSpecial(typed) && float32(math.Trunc(float64(typed))) == typed
	case json.Number:
		converted, err := typed.Float64()
		return err == nil && !math.IsNaN(converted) && !math.IsInf(converted, 0) && math.Trunc(converted) == converted
	default:
		return false
	}
}

func float32IsSpecial(value float32) bool {
	converted := float64(value)
	return math.IsNaN(converted) || math.IsInf(converted, 0)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
