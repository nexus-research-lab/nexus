// INPUT: 可信启动意图中的可选 scratch 身份及旧版无身份记录。
// OUTPUT: 完整身份持久化、部分/越界身份拒绝、旧记录不伪造证明。
// POS: 数据库校验；路径和 inode 实际真实性由宿主创建句柄负责。
package sandbox

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProcessScratchIdentityValidation(t *testing.T) {
	proof := protocol.SandboxProcessScratch{BasePath: filepath.Join(t.TempDir(), "sandbox"), LeafName: ".scratch-test", BaseIdentity: "darwin-v1:1:2:0:123:456", LeafIdentity: "darwin-v1:1:3:0:123:456"}
	i := processIntent()
	i.Scratch = proof
	r := newSandboxReceiptRepository(t)
	if err := r.PrepareProcess(t.Context(), i); err != nil {
		t.Fatal(err)
	}
	got, found, err := r.Process(t.Context(), i.Key)
	if err != nil || !found || got.Intent.Scratch != proof {
		t.Fatalf("scratch=%+v %v", got, err)
	}
	for name, mutate := range map[string]func(*protocol.SandboxProcessIntent){
		"missing lease":    func(i *protocol.SandboxProcessIntent) { i.LeaseID = "" },
		"missing parent":   func(i *protocol.SandboxProcessIntent) { i.Scratch.BaseIdentity = "" },
		"missing leaf":     func(i *protocol.SandboxProcessIntent) { i.Scratch.LeafIdentity = "" },
		"traversal":        func(i *protocol.SandboxProcessIntent) { i.Scratch.LeafName = ".scratch-test/../other" },
		"relative parent":  func(i *protocol.SandboxProcessIntent) { i.Scratch.BasePath = "relative/sandbox" },
		"other parent":     func(i *protocol.SandboxProcessIntent) { i.Scratch.BasePath = "/other" },
		"invalid identity": func(i *protocol.SandboxProcessIntent) { i.Scratch.LeafIdentity = "darwin-v1:unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := i
			mutate(&bad)
			if err := validateProcessIntent(bad); err == nil {
				t.Fatal("invalid scratch accepted")
			}
		})
	}
	legacy := processIntent()
	if err := validateProcessIntent(legacy); err != nil {
		t.Fatalf("rejected historical no-proof record: %v", err)
	}
}
