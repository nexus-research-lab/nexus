package agent

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunRuntimeWritesJSONUsageError(t *testing.T) {
	var stderr bytes.Buffer
	code := RunRuntime([]string{"--json", "--pretty", "goal", "inspect"}, &stderr)
	if code != exitCodeUsage {
		t.Fatalf("exit code = %d, want %d", code, exitCodeUsage)
	}
	var payload struct {
		Success bool `json:"success"`
		Error   struct {
			Kind string `json:"kind"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("解析 stderr JSON: %v", err)
	}
	if payload.Success || payload.Error.Kind != cliErrorKindUsage {
		t.Fatalf("usage envelope = %+v", payload)
	}
}
