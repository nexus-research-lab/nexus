package runtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

func TestWithPostToolUseGuidanceHookNormalizesEmptyOutput(t *testing.T) {
	options := WithPostToolUseGuidanceHook(agentclient.Options{}, func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
		return sdkhook.Output{}, nil
	})
	matchers := options.Hooks.Matchers[sdkhook.EventPostToolUse]
	if len(matchers) != 1 || len(matchers[0].Hooks) != 1 {
		t.Fatalf("PostToolUse hook 未注册: %+v", options.Hooks.Matchers)
	}

	output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-1")
	if err != nil {
		t.Fatalf("执行 PostToolUse hook 失败: %v", err)
	}
	if output.Continue == nil || !*output.Continue {
		t.Fatalf("空 hook 输出应规范成 continue=true: %+v", output)
	}
	if len(output.ToMap()) == 0 {
		t.Fatalf("空 hook 输出不应生成空 response")
	}
}

func TestWithPostToolUseGuidanceHookKeepsSpecificOutput(t *testing.T) {
	options := WithPostToolUseGuidanceHook(agentclient.Options{}, func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:     sdkhook.EventPostToolUse,
				AdditionalContext: "下一步继续处理",
			},
		}, nil
	})

	output, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-1",
	)
	if err != nil {
		t.Fatalf("执行 PostToolUse hook 失败: %v", err)
	}
	if output.SpecificOutput == nil || output.SpecificOutput.AdditionalContext != "下一步继续处理" {
		t.Fatalf("非空 hook 输出不应被改写: %+v", output)
	}
	if output.Continue != nil {
		t.Fatalf("非空 hook 输出不应额外注入 continue: %+v", output)
	}
}
