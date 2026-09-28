//go:build darwin

// INPUT: 已监督的 live lease、真实仓储和正常退出提交故障。
// OUTPUT: 最后一次 Release 留下持久完成事实，响应丢失可重试且保留引用。
// POS: 正常热退出测试；进程意图在执行前撤销，不伪造原生执行。
package runtime

import (
	"os"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSupervisedLastLeaseReleaseUsesDurableCleanup(t *testing.T) {
	h, repo, intent := newProcessHostFixture(t)
	fault := &scratchFaultStore{Repository: repo, phase: protocol.SandboxScratchRecoveryComplete, after: true}
	m := NewManager()
	m.SetSandboxPolicyReceiptStore(fault)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	path, leaseID := lease.Path(), lease.Marker().LeaseID
	options, err := m.supervisedProcessOptions(bridge.Options{Sandbox: &bridge.SandboxSettings{Resources: lease.Resources()}}, "owner", "session", 0, lease)
	if err != nil {
		t.Fatal(err)
	}
	config, err := options.ProcessSupervision(t.Context(), supervision.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Host.Reserve(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err == nil {
		t.Fatal("released scratch while launch remained prepared")
	}
	if err := config.Host.Finish(t.Context(), intent, nil); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err == nil || !fault.fired {
		t.Fatalf("expected lost completion response: %v", err)
	}
	if !lease.active() {
		t.Fatal("cleanup failure released owning handle")
	}
	record, found, err := repo.ScratchRecovery(t.Context(), "owner", "session", leaseID)
	if err != nil || !found || record.Phase != protocol.SandboxScratchRecoveryComplete {
		t.Fatalf("missing durable completion: %+v %v", record, err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if lease.active() {
		t.Fatal("successful retry retained owning handle")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("scratch remains", err)
	}
}
