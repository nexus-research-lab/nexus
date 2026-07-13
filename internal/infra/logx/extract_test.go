package logx

import (
	"reflect"
	"testing"
)

func TestPickAccessPreservesIncompleteFields(t *testing.T) {
	fields := []field{
		{key: "method", value: "GET"},
		{key: "status", value: "200"},
		{key: "request", value: "health"},
	}
	access, rest := pickAccess(fields)
	if access != nil {
		t.Fatalf("缺少 path 时不应识别为 access log: %+v", access)
	}
	if !reflect.DeepEqual(rest, fields) {
		t.Fatalf("普通日志字段应保持原顺序，实际 %+v", rest)
	}
}

func TestPickAccessExtractsCompleteCandidate(t *testing.T) {
	fields := []field{
		{key: "method", value: "POST"},
		{key: "status", value: "201"},
		{key: "path", value: "/v1/tasks"},
		{key: "remote_ip", value: "10.0.0.8"},
		{key: "request_id", value: "request-1"},
	}
	access, rest := pickAccess(fields)
	if access == nil || access.method != "POST" || access.status != 201 || access.path != "/v1/tasks" {
		t.Fatalf("access log 提取错误: %+v", access)
	}
	expected := []field{{key: "request_id", value: "request-1"}, {key: "ip", value: "10.0.0.8"}}
	if !reflect.DeepEqual(rest, expected) {
		t.Fatalf("剩余字段错误: %+v", rest)
	}
}
