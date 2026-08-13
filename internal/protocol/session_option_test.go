package protocol

import (
	"reflect"
	"testing"
)

func TestEffectiveSessionConnectorIDs(t *testing.T) {
	defaults := []string{"amap", "yuque"}
	if got := EffectiveSessionConnectorIDs(defaults, nil); !reflect.DeepEqual(got, defaults) {
		t.Fatalf("继承 Agent 默认 Connector = %v, want %v", got, defaults)
	}
	if got := EffectiveSessionConnectorIDs(defaults, map[string]any{
		OptionSessionConnectorIDs: []any{},
	}); len(got) != 0 {
		t.Fatalf("Session 显式关闭 Connector = %v, want []", got)
	}
	if got := EffectiveSessionConnectorIDs(defaults, map[string]any{
		OptionSessionConnectorIDs: []any{" feishu-docx ", "feishu-docx", ""},
	}); !reflect.DeepEqual(got, []string{"feishu-docx"}) {
		t.Fatalf("Session Connector 覆盖 = %v", got)
	}
}
