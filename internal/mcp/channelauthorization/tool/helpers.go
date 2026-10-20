// INPUT: strict MCP args and safe service values.
// OUTPUT: trimmed strings and JSON/error results.
// POS: transport-only helpers with no authorization or Channel logic.
package tool

import (
	"strings"
)

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}
