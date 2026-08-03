package jsoncodec

import "testing"

func TestMarshalMapReturnsEncodingErrors(t *testing.T) {
	if _, err := MarshalMap(map[string]any{"invalid": make(chan struct{})}); err == nil {
		t.Fatal("MarshalMap() 应返回不可编码值的错误")
	}

	payload, err := MarshalMap(nil)
	if err != nil {
		t.Fatalf("MarshalMap(nil) error = %v", err)
	}
	if payload != "{}" {
		t.Fatalf("MarshalMap(nil) = %q, want {}", payload)
	}
}
