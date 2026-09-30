// INPUT: 宿主进程回执与连接后策略回执的不同代次和阶段。
// OUTPUT: 后端切换或无策略回执都不能越过未收口启动；终态继续原有代次。
// POS: 产品创建进程之前的回归，不代表 launchd 或持久放行已经接线。
package runtime

import (
	"context"
	"errors"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type processStartupStore struct {
	recordingSandboxReceiptStore
	process    protocol.SandboxProcessSnapshot
	processErr error
}

func (s *processStartupStore) LatestProcess(context.Context, string, string) (protocol.SandboxProcessSnapshot, bool, error) {
	return s.process, true, s.processErr
}
func processStartupSnapshot() protocol.SandboxProcessSnapshot {
	return protocol.SandboxProcessSnapshot{Intent: protocol.SandboxProcessIntent{Version: 1, Key: protocol.SandboxProcessKey{OwnerUserID: "owner", SessionKey: "session", Generation: 41, LaunchID: "launch"}}, Phase: protocol.SandboxProcessPrepared}
}

func TestSandboxProcessIntentBlocksNewFactoryAcrossBackends(t *testing.T) {
	for _, phase := range []protocol.SandboxProcessPhase{protocol.SandboxProcessPrepared, protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased} {
		for _, kind := range []bridge.RuntimeKind{bridge.RuntimeNXS, bridge.RuntimeClaude} {
			t.Run(string(phase)+"/"+string(kind), func(t *testing.T) {
				snapshot := processStartupSnapshot()
				snapshot.Phase = phase
				store := &processStartupStore{process: snapshot}
				calls := 0
				manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls++; return &fakeRuntimeClient{} }))
				manager.SetSandboxPolicyReceiptStore(store)
				_, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{Runtime: bridge.RuntimeOptions{Kind: kind}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}})
				if !errors.Is(err, ErrSandboxCleanupPending) || calls != 0 {
					t.Fatalf("unresolved launch bypassed: %v, factory=%d", err, calls)
				}
			})
		}
	}
}
func TestSandboxProcessTerminalContinuesSharedGeneration(t *testing.T) {
	snapshot := processStartupSnapshot()
	snapshot.Phase = protocol.SandboxProcessAborted
	store := &processStartupStore{process: snapshot}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return &fakeRuntimeClient{} }))
	manager.SetSandboxPolicyReceiptStore(store)
	if _, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}); err != nil {
		t.Fatal(err)
	}
	if got := manager.sessions["session"].StartupGeneration; got != 42 {
		t.Fatalf("generation=%d", got)
	}
	policy := testSandboxReceiptSnapshotForManager()
	policy.OwnerUserID = "owner"
	policy.SessionKey = "session"
	policy.Generation = 51
	policy.Phase = protocol.SandboxPolicyReceiptRetired
	store.latest = policy
	store.found = true
	if floor, err := manager.sandboxStartupGeneration(t.Context(), "owner", "session"); err != nil || floor != 51 {
		t.Fatalf("combined floor=%d %v", floor, err)
	}
}
func TestSandboxProcessUnverifiableReadRejectsStartup(t *testing.T) {
	for _, mode := range []string{"read_error", "wrong_owner", "wrong_session", "missing_evidence", "phase"} {
		t.Run(mode, func(t *testing.T) {
			snapshot := processStartupSnapshot()
			store := &processStartupStore{process: snapshot}
			switch mode {
			case "read_error":
				store.processErr = errors.New("unreadable database")
			case "wrong_owner":
				store.process.Intent.Key.OwnerUserID = "other"
			case "wrong_session":
				store.process.Intent.Key.SessionKey = "other"
			case "missing_evidence":
				store.process.Phase = protocol.SandboxProcessReaped
			case "phase":
				store.process.Phase = "unknown"
			}
			if _, err := sandboxProcessStartupGeneration(t.Context(), store, "owner", "session"); err == nil {
				t.Fatal("unverifiable process state admitted")
			}
		})
	}
}

func TestSandboxProcessRootExitCannotRetireScope(t *testing.T) {
	snapshot := processStartupSnapshot()
	snapshot.Intent.BootID = "12345678-1234-1234-1234-123456789abc"
	snapshot.Intent.OwnerUID = 501
	snapshot.Phase = protocol.SandboxProcessReaped
	registration := protocol.SandboxProcessRegistration{Version: 1, BootID: snapshot.Intent.BootID, OwnerUID: 501, CoalitionID: 91}
	snapshot.Registration = &registration
	snapshot.Evidence = &protocol.SandboxProcessEvidence{Registration: registration, Reason: "root_exit", ObservedBootID: snapshot.Intent.BootID}
	store := &processStartupStore{process: snapshot}
	if _, err := sandboxProcessStartupGeneration(t.Context(), store, "owner", "session"); err == nil {
		t.Fatal("root exit admitted fresh generation")
	}
	snapshot.Evidence.Reason = "coalition_reaped"
	if floor, err := sandboxProcessStartupGeneration(t.Context(), store, "owner", "session"); err != nil || floor != 41 {
		t.Fatalf("exact retirement=%d %v", floor, err)
	}
}
