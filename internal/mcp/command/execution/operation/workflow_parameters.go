// INPUT: Complete template editor revisions.
// OUTPUT: Bounded typed parameter definitions accepted by the shared validator.
// POS: Model transport schema; values are data, never executable interpolation.
package operation

func workflowParametersSchema() map[string]any {
	return map[string]any{"type": "array", "maxItems": 32, "items": objectSchema(map[string]any{
		"name":        map[string]any{"type": "string", "pattern": "^[a-zA-Z][a-zA-Z0-9_]{0,63}$"},
		"type":        enumProperty("Parameter value type.", "string", "number", "integer", "boolean", "string_list"),
		"description": stringProperty("Purpose of this input."), "required": booleanProperty("Require an explicit value when no default exists."),
		"default": map[string]any{"description": "Optional typed default; null is not a value."},
		"enum":    stringArrayProperty("Allowed values for a string parameter."),
		"minimum": map[string]any{"type": "number"}, "maximum": map[string]any{"type": "number"},
	}, "name", "type")}
}
