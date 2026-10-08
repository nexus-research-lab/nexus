// INPUT: nexuscfg 真实序列化请求与宿主严格解码的配置 broker。
// OUTPUT: 只读和写入请求均能进入业务校验，人工 reconcile 不进入 runtime 通道。
// POS: CLI 与宿主请求契约回归；不连接真实账号或写入配置。
package cli

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	appruntime "github.com/nexus-research-lab/nexus/internal/app/runtime"
	"github.com/nexus-research-lab/nexus/internal/config"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestRuntimeConfigurationRequestsReachHostValidation(t *testing.T) {
	manager := runtimectx.NewManager()
	const session, round = "broker-test-session", "broker-test-round"
	if err := manager.StartRound(t.Context(), session, round, nil); err != nil {
		t.Fatal(err)
	}
	defer manager.MarkRoundFinished(session, round)
	service := configurationsvc.NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil, manager)
	token, err := service.IssueRuntimeCapability(configurationsvc.Actor{
		OwnerUserID: "owner", AgentID: "main", LeaseSessionKey: session, LeaseRoundID: round, RoundLeaseRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(appruntime.NewConfigurationHandler(service, nil))
	defer server.Close()
	controller := &runtimeConfigurationController{endpoint: server.URL, token: token, client: server.Client()}
	request := configurationsvc.ChangeRequest{Domain: "agents", Operation: "update_self_profile"}
	calls := map[string]func(context.Context) error{
		"inspect all": func(ctx context.Context) error { _, err := controller.Inspect(ctx, nil, false); return err },
		"inspect members": func(ctx context.Context) error {
			_, err := controller.Inspect(ctx, []string{"members"}, false)
			return err
		},
		"inspect agents": func(ctx context.Context) error {
			_, err := controller.Inspect(ctx, []string{"agents"}, false)
			return err
		},
		"history": func(ctx context.Context) error { _, err := controller.History(ctx, "members", 10); return err },
		"plan":    func(ctx context.Context) error { _, err := controller.Plan(ctx, request); return err },
		"apply": func(ctx context.Context) error {
			_, err := controller.Apply(ctx, request, configurationsvc.CLIApplyOptions{})
			return err
		},
		"review": func(ctx context.Context) error { _, err := controller.Review(ctx, "cfg-broker-test"); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// 不装配 Agent 服务，让请求在通过真实解码后安全停在身份校验。
			if err := call(t.Context()); err == nil || !strings.Contains(err.Error(), configurationsvc.ErrMainAgentRequired.Error()) {
				t.Fatalf("应进入宿主业务校验，实际错误: %v", err)
			}
		})
	}
	if _, err := controller.Reconcile(t.Context(), configurationsvc.ReconcileRequest{}); err == nil || !strings.Contains(err.Error(), "不能提交人工配置 reconcile") {
		t.Fatalf("runtime 必须拒绝人工核对命令: %v", err)
	}
}
