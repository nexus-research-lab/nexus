package runtimecommand

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildContractReturnsDirectoryBeforeExactOperationSchema(t *testing.T) {
	operations := []Operation{
		{Name: "read", Description: "Read state", ReadOnly: true, InputSchema: map[string]any{"type": "object"}},
		{Name: "write", Description: "Write state", Idempotent: true, InputSchema: map[string]any{"type": "object", "required": []string{"value"}}},
	}
	directory, err := BuildContract("domain", "read", "", operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(directory.Operations) != 2 {
		t.Fatalf("directory operations = %d, want 2", len(directory.Operations))
	}
	for _, operation := range directory.Operations {
		if operation.InputSchema != nil {
			t.Fatalf("directory operation %q exposed its schema", operation.Name)
		}
	}
	encoded, err := json.Marshal(directory)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "input_schema") {
		t.Fatalf("directory JSON contains input_schema: %s", encoded)
	}

	exact, err := BuildContract("domain", "read", "write", operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(exact.Operations) != 1 || exact.Operations[0].Name != "write" ||
		exact.Operations[0].InputSchema == nil {
		t.Fatalf("exact contract = %+v", exact)
	}
}
