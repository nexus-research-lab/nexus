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

func TestSessionAdditionalDirectoriesOptions(t *testing.T) {
	options := WithSessionAdditionalDirectories(map[string]any{
		"keep": "value",
	}, []string{" /tmp/project ", "/tmp/project", ""})
	if got := SessionAdditionalDirectoriesFromOptions(options); !reflect.DeepEqual(
		got,
		[]string{"/tmp/project"},
	) {
		t.Fatalf("Session 附加目录 = %v", got)
	}
	if options["keep"] != "value" {
		t.Fatalf("无关 option 被覆盖: %v", options)
	}
	cleared := WithSessionAdditionalDirectories(options, nil)
	if _, exists := cleared[OptionSessionAdditionalDirectories]; exists {
		t.Fatalf("清空后仍保留附加目录: %v", cleared)
	}
}
