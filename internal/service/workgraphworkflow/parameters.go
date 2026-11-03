// INPUT: Template parameter declarations, explicit JSON invocation and exact saved version.
// OUTPUT: Validated defaults/values and prompt data that cannot alter runtime authority.
// POS: Shared template editing, slash expansion and Plan provenance validation.
package workgraphworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var parameterNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

func validateParameters(parameters []protocol.WorkGraphParameter) error {
	if len(parameters) > 32 {
		return fmt.Errorf("%w: at most 32 template parameters", ErrInvalidInput)
	}
	seen := map[string]bool{}
	for _, p := range parameters {
		if !parameterNamePattern.MatchString(p.Name) || seen[p.Name] || len(p.Description) > 2048 {
			return fmt.Errorf("%w: invalid or duplicate parameter %q", ErrInvalidInput, p.Name)
		}
		seen[p.Name] = true
		if !slices.Contains([]string{"string", "number", "integer", "boolean", "string_list"}, p.Type) {
			return fmt.Errorf("%w: unknown type for %s", ErrInvalidInput, p.Name)
		}
		if len(p.Enum) > 32 || len(p.Enum) > 0 && p.Type != "string" {
			return fmt.Errorf("%w: enum requires a string parameter", ErrInvalidInput)
		}
		if (p.Minimum != nil || p.Maximum != nil) && p.Type != "integer" && p.Type != "number" {
			return fmt.Errorf("%w: bounds require numeric parameters", ErrInvalidInput)
		}
		for _, bound := range []*float64{p.Minimum, p.Maximum} {
			if bound != nil && (math.IsNaN(*bound) || math.IsInf(*bound, 0)) {
				return fmt.Errorf("%w: parameter bounds must be finite", ErrInvalidInput)
			}
		}
		if p.Minimum != nil && p.Maximum != nil && *p.Minimum > *p.Maximum {
			return fmt.Errorf("%w: parameter bounds reversed", ErrInvalidInput)
		}
		if p.Type == "integer" && p.Minimum != nil && p.Maximum != nil && math.Ceil(*p.Minimum) > math.Floor(*p.Maximum) {
			return fmt.Errorf("%w: parameter bounds contain no integer", ErrInvalidInput)
		}
		enumValues := map[string]bool{}
		for _, v := range p.Enum {
			if len(v) > 4096 || enumValues[v] {
				return fmt.Errorf("%w: invalid enum", ErrInvalidInput)
			}
			enumValues[v] = true
		}
		if p.Default != nil {
			if err := validateParameterValue(p, p.Default); err != nil {
				return err
			}
		}
	}
	return nil
}
func validateParameterValue(p protocol.WorkGraphParameter, value any) error {
	bad := func() error {
		return fmt.Errorf("%w: parameter %s must match %s and its constraints", ErrInvalidInput, p.Name, p.Type)
	}
	if value == nil {
		return bad()
	}
	switch p.Type {
	case "string":
		s, ok := value.(string)
		if !ok || len(s) > 4096 || len(p.Enum) > 0 && !slices.Contains(p.Enum, s) {
			return bad()
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return bad()
		}
	case "number", "integer":
		raw, err := json.Marshal(value)
		if err != nil {
			return bad()
		}
		var n float64
		if json.Unmarshal(raw, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) || p.Type == "integer" && math.Trunc(n) != n || p.Minimum != nil && n < *p.Minimum || p.Maximum != nil && n > *p.Maximum {
			return bad()
		}
	case "string_list":
		raw, err := json.Marshal(value)
		if err != nil {
			return bad()
		}
		var items []string
		if string(raw) == "null" || json.Unmarshal(raw, &items) != nil || len(items) > 32 {
			return bad()
		}
		for _, item := range items {
			if len(item) > 4096 {
				return bad()
			}
		}
	default:
		return bad()
	}
	return nil
}
func resolveParameters(definitions []protocol.WorkGraphParameter, values map[string]any) (map[string]any, error) {
	if err := validateParameters(definitions); err != nil {
		return nil, err
	}
	declared := map[string]bool{}
	result := map[string]any{}
	for _, p := range definitions {
		declared[p.Name] = true
		value, exists := values[p.Name]
		if !exists && p.Default != nil {
			value, exists = p.Default, true
		}
		if !exists {
			if p.Required {
				return nil, fmt.Errorf("%w: missing required parameter %s", ErrInvalidInput, p.Name)
			}
			continue
		}
		if err := validateParameterValue(p, value); err != nil {
			return nil, err
		}
		result[p.Name] = value
	}
	for name := range values {
		if !declared[name] {
			return nil, fmt.Errorf("%w: unknown parameter %s", ErrInvalidInput, name)
		}
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > 16384 {
		return nil, fmt.Errorf("%w: parameter values exceed 16 KiB", ErrInvalidInput)
	}
	var cloned map[string]any
	_ = json.Unmarshal(raw, &cloned)
	return cloned, nil
}
func cloneParameters(parameters []protocol.WorkGraphParameter) []protocol.WorkGraphParameter {
	raw, _ := json.Marshal(parameters)
	var cloned []protocol.WorkGraphParameter
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func (s *Service) ValidateWorkflowSource(ctx context.Context, owner string, source protocol.WorkGraphWorkflowSource) (*protocol.WorkGraphWorkflowSource, error) {
	workflow, err := s.repository.GetByID(ctx, owner, source.ID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		for _, item := range builtinWorkflows("en") {
			if item.ID == source.ID {
				copy := item
				workflow = &copy
				break
			}
		}
	}
	if workflow == nil || source.Version <= 0 || workflow.Version != source.Version {
		return nil, fmt.Errorf("%w: template version is unavailable; inspect the library again", ErrInvalidInput)
	}
	values, err := resolveParameters(workflow.Parameters, source.Parameters)
	if err != nil {
		return nil, err
	}
	source.Parameters = values
	return &source, nil
}

func parameterInvocation(workflow protocol.WorkGraphWorkflow, arguments string) (string, *protocol.WorkGraphWorkflowSource, error) {
	source := &protocol.WorkGraphWorkflowSource{ID: workflow.ID, Version: workflow.Version}
	if len(workflow.Parameters) == 0 {
		return arguments, source, nil
	}
	var input struct {
		Parameters map[string]any `json:"parameters"`
		Request    string         `json:"request"`
	}
	if len(arguments) > 32768 {
		return "", nil, fmt.Errorf("%w: invocation too large", ErrInvalidInput)
	}
	if err := validateInvocationJSON(arguments); err != nil {
		return "", nil, err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return "", nil, fmt.Errorf("%w: parameterized /%s expects JSON {\"parameters\":{...},\"request\":\"...\"}: %v", ErrInvalidInput, workflow.SlashName, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", nil, fmt.Errorf("%w: one invocation object is required", ErrInvalidInput)
	}
	values, err := resolveParameters(workflow.Parameters, input.Parameters)
	if err != nil {
		return "", nil, err
	}
	source.Parameters = values
	return input.Request, source, nil
}

// JSON itself permits duplicate keys; invocation intent must be unambiguous.
func validateInvocationJSON(input string) error {
	d := json.NewDecoder(bytes.NewBufferString(input))
	var read func(int) error
	read = func(depth int) error {
		if depth > 8 {
			return fmt.Errorf("%w: invocation too deeply nested", ErrInvalidInput)
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("%w: duplicate invocation field", ErrInvalidInput)
				}
				seen[name] = true
				if err = read(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err = read(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%w: invalid invocation", ErrInvalidInput)
		}
		_, err = d.Token()
		return err
	}
	if len(input) == 0 || input[0] != '{' {
		return fmt.Errorf("%w: invocation must be a JSON object", ErrInvalidInput)
	}
	return read(0)
}
