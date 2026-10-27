package runtime

import (
	"context"
	"errors"
	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"strings"
	"testing"
)

func TestSandboxStartupExplanationPreservesCause(t *testing.T) {
	cause := errors.New("this Windows build does not provide the process security environment API")
	client := &agentClient{newSession: func(context.Context, bridge.Options) (*bridge.Session, error) { return nil, cause }}
	_, err := client.openSession(context.Background(), bridge.Options{})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "当前 Windows 版本不支持桌面沙箱") {
		t.Fatalf("lost explanation or cause: %v", err)
	}
}
