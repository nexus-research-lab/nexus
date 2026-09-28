// INPUT: 跨 owner 的原进程状态、游标及分页期间收口。
// OUTPUT: 未收口 exact key 的稳定有界读取，不因旧页删除跳过下一条。
// POS: 恢复扫描仓储测试，不执行进程回收。
package sandbox

import (
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPendingProcessKeysStableCursorAndPhases(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	var intents []protocol.SandboxProcessIntent
	for n := 1; n <= 5; n++ {
		i := processIntent()
		i.Key.OwnerUserID = fmt.Sprintf("owner-%d", n)
		i.Key.LaunchID = fmt.Sprintf("%032x", n)
		i.JobLabel = "cn.nexus.runtime." + i.Key.LaunchID
		if err := r.PrepareProcess(t.Context(), i); err != nil {
			t.Fatal(err)
		}
		if n == 2 || n == 3 || n == 5 {
			if err := r.RegisterProcess(t.Context(), i.Key, processRegistration(i)); err != nil {
				t.Fatal(err)
			}
		}
		if n == 3 {
			if err := r.ClaimProcessRelease(t.Context(), i.Key); err != nil {
				t.Fatal(err)
			}
		}
		if n == 4 {
			if err := r.AbortPreparedProcess(t.Context(), i.Key); err != nil {
				t.Fatal(err)
			}
		}
		if n == 5 {
			if err := r.ReapProcess(t.Context(), i.Key, protocol.SandboxProcessEvidence{Registration: processRegistration(i), Reason: "coalition_reaped", ObservedBootID: i.BootID}); err != nil {
				t.Fatal(err)
			}
		}
		intents = append(intents, i)
	}
	page, more, err := r.PendingProcessKeys(t.Context(), "", 1)
	if err != nil || !more || len(page) != 1 || page[0] != intents[0].Key {
		t.Fatalf("page=%+v more=%v err=%v", page, more, err)
	}
	if err := r.AbortPreparedProcess(t.Context(), intents[0].Key); err != nil {
		t.Fatal(err)
	}
	page, more, err = r.PendingProcessKeys(t.Context(), page[0].LaunchID, 2)
	if err != nil || more || len(page) != 2 || page[0] != intents[1].Key || page[1] != intents[2].Key {
		t.Fatalf("page=%+v more=%v err=%v", page, more, err)
	}
	// 坏正文不隐藏原 key；逐条恢复负责校验并保留该记录。
	if _, err := r.db.Exec(`UPDATE sandbox_process_launches SET intent_json='broken' WHERE launch_id=?`, intents[1].Key.LaunchID); err != nil {
		t.Fatal(err)
	}
	page, _, err = r.PendingProcessKeys(t.Context(), "", 2)
	if err != nil || len(page) != 2 || page[0] != intents[1].Key {
		t.Fatalf("corrupt payload hid recovery key: %+v %v", page, err)
	}
	if _, _, err := r.Process(t.Context(), intents[1].Key); err == nil {
		t.Fatal("corrupt intent accepted for recovery")
	}
	for _, input := range []struct {
		cursor string
		limit  int
	}{{"bad", 1}, {"", 0}, {"", 257}} {
		if _, _, err := r.PendingProcessKeys(t.Context(), input.cursor, input.limit); err == nil {
			t.Fatal("invalid page accepted", input)
		}
	}
}
