// INPUT: closed Nexus Computer Use operation objects validated by the portable schema subset.
// OUTPUT: official SDK action/predicate values with cross-field and numeric bounds enforced.
// POS: Agent-controlled semantic input boundary; secrets are wrapped before reaching the SDK.
package computeruse

import (
	"errors"
	"fmt"
	"math"
	"strings"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

const maxSensitiveTextBytes = 64 << 10

func parseAction(input map[string]any) (nexuscua.Action, error) {
	kind := stringInput(input, "kind")
	switch kind {
	case string(nexuscua.ActionFocusWindow):
		return nexuscua.FocusWindow{}, requireActionKeys(input, "kind")
	case string(nexuscua.ActionFocusElement):
		if err := requireActionKeys(input, "kind", "element_ref"); err != nil {
			return nil, err
		}
		return nexuscua.FocusElement{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref"))}, requireNonEmpty(input, "element_ref")
	case string(nexuscua.ActionInvokeElement):
		if err := requireActionKeys(input, "kind", "element_ref"); err != nil {
			return nil, err
		}
		return nexuscua.InvokeElement{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref"))}, requireNonEmpty(input, "element_ref")
	case string(nexuscua.ActionClickPoint):
		if err := requireActionKeys(input, "kind", "x", "y", "button", "count"); err != nil {
			return nil, err
		}
		x, err := uint32Input(input, "x", true)
		if err != nil {
			return nil, err
		}
		y, err := uint32Input(input, "y", true)
		if err != nil {
			return nil, err
		}
		count, err := uint32Input(input, "count", false)
		if err != nil || count < 1 || count > 3 {
			return nil, errors.New("click_point count must be between 1 and 3")
		}
		button := nexuscua.PointerButton(stringInputDefault(input, "button", string(nexuscua.PointerLeft)))
		if button != nexuscua.PointerLeft && button != nexuscua.PointerMiddle && button != nexuscua.PointerRight {
			return nil, errors.New("click_point button is invalid")
		}
		return nexuscua.ClickPoint{Point: nexuscua.ScreenshotPoint{X: x, Y: y}, Button: button, Count: uint8(count)}, nil
	case string(nexuscua.ActionSetValue):
		if err := requireActionKeys(input, "kind", "element_ref", "value"); err != nil {
			return nil, err
		}
		if err := requireNonEmpty(input, "element_ref"); err != nil {
			return nil, err
		}
		value, err := sensitiveStringInput(input, "value")
		if err != nil {
			return nil, err
		}
		return nexuscua.SetValue{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref")), Value: nexuscua.NewSensitiveText(value)}, nil
	case string(nexuscua.ActionToggleElement):
		if err := requireActionKeys(input, "kind", "element_ref"); err != nil {
			return nil, err
		}
		return nexuscua.ToggleElement{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref"))}, requireNonEmpty(input, "element_ref")
	case string(nexuscua.ActionSelectElement):
		if err := requireActionKeys(input, "kind", "element_ref"); err != nil {
			return nil, err
		}
		return nexuscua.SelectElement{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref"))}, requireNonEmpty(input, "element_ref")
	case string(nexuscua.ActionSetExpanded):
		if err := requireActionKeys(input, "kind", "element_ref", "expanded"); err != nil {
			return nil, err
		}
		expanded, ok := input["expanded"].(bool)
		if !ok || strings.TrimSpace(stringInput(input, "element_ref")) == "" {
			return nil, errors.New("set_expanded requires element_ref and expanded")
		}
		return nexuscua.SetExpanded{ElementRef: nexuscua.ElementRef(stringInput(input, "element_ref")), Expanded: expanded}, nil
	case string(nexuscua.ActionMovePointer):
		if err := requireActionKeys(input, "kind", "x", "y", "duration_ms"); err != nil {
			return nil, err
		}
		x, err := uint32Input(input, "x", true)
		if err != nil {
			return nil, err
		}
		y, err := uint32Input(input, "y", true)
		if err != nil {
			return nil, err
		}
		duration, err := boundedDuration(input)
		if err != nil {
			return nil, err
		}
		return nexuscua.MovePointer{Point: nexuscua.ScreenshotPoint{X: x, Y: y}, DurationMS: duration}, nil
	case string(nexuscua.ActionTypeText):
		if err := requireActionKeys(input, "kind", "text"); err != nil {
			return nil, err
		}
		value, err := sensitiveStringInput(input, "text")
		if err != nil {
			return nil, err
		}
		return nexuscua.TypeText{Text: nexuscua.NewSensitiveText(value)}, nil
	case string(nexuscua.ActionPressKeys):
		if err := requireActionKeys(input, "kind", "keys"); err != nil {
			return nil, err
		}
		keys, err := stringArrayInput(input, "keys", 16)
		if err != nil || len(keys) == 0 {
			return nil, errors.New("press_keys requires 1-16 non-empty keys")
		}
		return nexuscua.PressKeys{Keys: keys}, nil
	case string(nexuscua.ActionScroll):
		if err := requireActionKeys(input, "kind", "element_ref", "delta_x", "delta_y"); err != nil {
			return nil, err
		}
		deltaX, err := finiteNumberInput(input, "delta_x", false)
		if err != nil {
			return nil, err
		}
		deltaY, err := finiteNumberInput(input, "delta_y", false)
		if err != nil {
			return nil, err
		}
		var element *nexuscua.ElementRef
		if value := strings.TrimSpace(stringInput(input, "element_ref")); value != "" {
			ref := nexuscua.ElementRef(value)
			element = &ref
		}
		return nexuscua.Scroll{ElementRef: element, DeltaX: deltaX, DeltaY: deltaY}, nil
	case string(nexuscua.ActionDrag):
		if err := requireActionKeys(input, "kind", "from_x", "from_y", "to_x", "to_y", "duration_ms"); err != nil {
			return nil, err
		}
		fromX, err := uint32Input(input, "from_x", true)
		if err != nil {
			return nil, err
		}
		fromY, err := uint32Input(input, "from_y", true)
		if err != nil {
			return nil, err
		}
		toX, err := uint32Input(input, "to_x", true)
		if err != nil {
			return nil, err
		}
		toY, err := uint32Input(input, "to_y", true)
		if err != nil {
			return nil, err
		}
		duration, err := boundedDuration(input)
		if err != nil {
			return nil, err
		}
		return nexuscua.Drag{
			From: nexuscua.ScreenshotPoint{X: fromX, Y: fromY}, To: nexuscua.ScreenshotPoint{X: toX, Y: toY}, DurationMS: duration,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported Computer Use action kind %q", kind)
	}
}

func parsePredicate(input map[string]any) (nexuscua.StatePredicate, error) {
	kind := stringInput(input, "kind")
	switch kind {
	case "window_title_contains":
		if err := requireActionKeys(input, "kind", "text"); err != nil {
			return nil, err
		}
		if err := requireNonEmpty(input, "text"); err != nil {
			return nil, err
		}
		return nexuscua.WindowTitleContains{Text: stringInput(input, "text")}, nil
	case "element_exists":
		if err := requireActionKeys(input, "kind", "role", "name"); err != nil {
			return nil, err
		}
		var role, name *string
		if value := strings.TrimSpace(stringInput(input, "role")); value != "" {
			role = &value
		}
		if value := strings.TrimSpace(stringInput(input, "name")); value != "" {
			name = &value
		}
		if role == nil && name == nil {
			return nil, errors.New("element_exists requires role or name")
		}
		return nexuscua.ElementExists{Role: role, Name: name}, nil
	case "bounds_contained":
		if err := requireActionKeys(input, "kind", "inner", "outer"); err != nil {
			return nil, err
		}
		inner, err := screenRectInput(input["inner"], "inner")
		if err != nil {
			return nil, err
		}
		outer, err := screenRectInput(input["outer"], "outer")
		if err != nil {
			return nil, err
		}
		return nexuscua.BoundsContained{Inner: inner, Outer: outer}, nil
	default:
		return nil, fmt.Errorf("unsupported Computer Use predicate kind %q", kind)
	}
}

func requireActionKeys(input map[string]any, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range input {
		if _, ok := set[key]; !ok {
			return fmt.Errorf("field %q is not valid for this Computer Use kind", key)
		}
	}
	return nil
}

func requireNonEmpty(input map[string]any, key string) error {
	if strings.TrimSpace(stringInput(input, key)) == "" {
		return fmt.Errorf("%s is required", key)
	}
	return nil
}

func stringInput(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}

func stringInputDefault(input map[string]any, key, fallback string) string {
	if value := stringInput(input, key); value != "" {
		return value
	}
	return fallback
}

func sensitiveStringInput(input map[string]any, key string) (string, error) {
	value, ok := input[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	if len(value) > maxSensitiveTextBytes {
		return "", fmt.Errorf("%s exceeds the Computer Use input size limit", key)
	}
	return value, nil
}

func uint32Input(input map[string]any, key string, required bool) (uint32, error) {
	value, exists := input[key]
	if !exists && !required {
		return 1, nil
	}
	number, err := finiteNumber(value)
	if err != nil || math.Trunc(number) != number || number < 0 || number > math.MaxUint32 {
		return 0, fmt.Errorf("%s must be an unsigned 32-bit integer", key)
	}
	return uint32(number), nil
}

func finiteNumberInput(input map[string]any, key string, required bool) (float64, error) {
	value, exists := input[key]
	if !exists && !required {
		return 0, nil
	}
	number, err := finiteNumber(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a finite number", key)
	}
	return number, nil
}

func finiteNumber(value any) (float64, error) {
	var result float64
	switch typed := value.(type) {
	case float64:
		result = typed
	case float32:
		result = float64(typed)
	case int:
		result = float64(typed)
	case int64:
		result = float64(typed)
	case uint32:
		result = float64(typed)
	default:
		return 0, errors.New("not a number")
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, errors.New("not finite")
	}
	return result, nil
}

func boundedDuration(input map[string]any) (uint32, error) {
	duration, err := uint32Input(input, "duration_ms", false)
	if err != nil {
		return 0, err
	}
	if duration > 30_000 {
		return 0, errors.New("duration_ms must not exceed 30000")
	}
	return duration, nil
}

func stringArrayInput(input map[string]any, key string, limit int) ([]string, error) {
	values, ok := input[key].([]any)
	if !ok {
		if typed, typedOK := input[key].([]string); typedOK {
			values = make([]any, len(typed))
			for index := range typed {
				values[index] = typed[index]
			}
		} else {
			return nil, fmt.Errorf("%s must be an array", key)
		}
	}
	if len(values) > limit {
		return nil, fmt.Errorf("%s contains too many values", key)
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		text, ok := item.(string)
		text = strings.TrimSpace(text)
		if !ok || text == "" || len(text) > 64 {
			return nil, fmt.Errorf("%s contains an invalid value", key)
		}
		result = append(result, text)
	}
	return result, nil
}

func screenRectInput(value any, label string) (nexuscua.ScreenRect, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nexuscua.ScreenRect{}, fmt.Errorf("%s must be an object", label)
	}
	if err := requireActionKeys(object, "x", "y", "width", "height"); err != nil {
		return nexuscua.ScreenRect{}, err
	}
	x, err := finiteNumberInput(object, "x", true)
	if err != nil {
		return nexuscua.ScreenRect{}, err
	}
	y, err := finiteNumberInput(object, "y", true)
	if err != nil {
		return nexuscua.ScreenRect{}, err
	}
	width, err := finiteNumberInput(object, "width", true)
	if err != nil || width < 0 {
		return nexuscua.ScreenRect{}, fmt.Errorf("%s width must be non-negative", label)
	}
	height, err := finiteNumberInput(object, "height", true)
	if err != nil || height < 0 {
		return nexuscua.ScreenRect{}, fmt.Errorf("%s height must be non-negative", label)
	}
	return nexuscua.ScreenRect{X: x, Y: y, Width: width, Height: height}, nil
}
