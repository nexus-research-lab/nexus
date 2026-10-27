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
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "当前 Windows 未提供桌面沙箱所需的系统 API") {
		t.Fatalf("lost explanation or cause: %v", err)
	}
}

func TestSandboxUnsupportedCreateIsNotMissingDependency(t *testing.T) {
	got := DesktopSandboxUnavailableMessage("the Windows sandbox cannot create a process security environment (stage=availability, build=26300): CreateProcessSecurityEnvironment failed: HRESULT 0x80070032")
	if !strings.Contains(got, "不支持当前安全环境创建请求") || !strings.Contains(got, "设置 → 沙箱") {
		t.Fatalf("unexpected message: %s", got)
	}
	if got := DesktopSandboxUnavailableMessage("CreateProcessSecurityEnvironment failed: HRESULT 0x80070005"); got != "" {
		t.Fatalf("access denied misclassified: %s", got)
	}
}
