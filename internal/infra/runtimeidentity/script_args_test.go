package runtimeidentity

import (
	"strings"
	"testing"
)

func TestValidateScriptArguments(t *testing.T) {
	if script, err := validateScriptArguments([]string{"printf 'ok\\n'"}); err != nil || script == "" {
		t.Fatalf("合法脚本被拒绝: script=%q err=%v", script, err)
	}
	for _, args := range [][]string{
		nil,
		{""},
		{"echo one", "echo two"},
		{"echo \x00"},
		{strings.Repeat("x", maxScriptBytes+1)},
	} {
		if _, err := validateScriptArguments(args); err == nil {
			t.Fatalf("非法脚本参数未被拒绝: %#v", args)
		}
	}
}
