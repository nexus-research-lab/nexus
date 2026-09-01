package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFailureCoreV1JSONContract(t *testing.T) {
	failure := FailureCore{
		Version:            FailureCoreVersion,
		Code:               "workgraph.revision_conflict",
		Category:           FailureCategoryConflict,
		Effect:             FailureEffectNotApplied,
		TransportRequestID: "http-request-1",
	}

	payload, err := json.Marshal(failure)
	if err != nil {
		t.Fatalf("编码 FailureCore: %v", err)
	}
	want := `{"version":1,"code":"workgraph.revision_conflict","category":"conflict","effect":"not_applied","transport_request_id":"http-request-1"}`
	if string(payload) != want {
		t.Fatalf("FailureCore JSON 不符合合同:\n got: %s\nwant: %s", payload, want)
	}
}

func TestFailureCoreOmitsOptionalFields(t *testing.T) {
	payload, err := json.Marshal(FailureCore{
		Version:  FailureCoreVersion,
		Code:     "common.request_failed",
		Category: FailureCategoryInternal,
		Effect:   FailureEffectUnknown,
	})
	if err != nil {
		t.Fatalf("编码 FailureCore: %v", err)
	}
	if strings.Contains(string(payload), "transport_request_id") {
		t.Fatalf("空请求身份不应进入 JSON: %s", payload)
	}
}

func TestFailureCoreAcceptsFutureWireValues(t *testing.T) {
	raw := `{
		"version":2,
		"code":"future.new_failure",
		"category":"future_category",
		"effect":"future_effect",
		"future_field":true
	}`
	var failure FailureCore
	if err := json.Unmarshal([]byte(raw), &failure); err != nil {
		t.Fatalf("未来 wire 值不应导致解码失败: %v", err)
	}
	if failure.Version != 2 || failure.Code != "future.new_failure" ||
		failure.Category != "future_category" || failure.Effect != "future_effect" {
		t.Fatalf("未来 wire 值未被保留: %#v", failure)
	}
}
