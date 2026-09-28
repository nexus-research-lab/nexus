// INPUT: 宿主数据库中 exact owner/session 的最新进程启动事实。
// OUTPUT: 未收口执行阻断新 factory；终态只推进同一 StartupGeneration 下界。
// POS: 执行前崩溃窗口的启动栅栏，不读取用户可写 scratch 的进程身份。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func sandboxProcessStartupGeneration(ctx context.Context, store SandboxPolicyReceiptStore, owner, session string) (uint64, error) {
	reader, ok := store.(SandboxProcessReceiptReader)
	// 非数据库 sink 沿用原合同；监督启动接入必须单独要求写入接口。
	if !ok {
		return 0, nil
	}
	snapshot, found, err := reader.LatestProcess(ctx, owner, session)
	if err != nil {
		return 0, fmt.Errorf("read previous sandbox process registration: %w", err)
	}
	if !found {
		return 0, nil
	}
	key := snapshot.Intent.Key
	if snapshot.Intent.Version != 1 || key.OwnerUserID != owner || key.SessionKey != session || key.Generation == 0 || key.Generation >= math.MaxInt64 {
		return 0, errors.New("invalid previous sandbox process binding")
	}
	switch snapshot.Phase {
	case protocol.SandboxProcessPrepared, protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased:
		return 0, ErrSandboxCleanupPending
	case protocol.SandboxProcessAborted:
		if snapshot.Registration != nil || snapshot.Evidence != nil {
			return 0, errors.New("aborted sandbox intent contains execution evidence")
		}
	case protocol.SandboxProcessReaped:
		if snapshot.Registration == nil || snapshot.Evidence == nil || snapshot.Evidence.Registration != *snapshot.Registration {
			return 0, errors.New("sandbox process retirement lacks exact evidence")
		}
		registration, evidence := snapshot.Registration, snapshot.Evidence
		if registration.Version != 1 || registration.CoalitionID == 0 || registration.BootID == "" || registration.BootID != snapshot.Intent.BootID || registration.OwnerUID != snapshot.Intent.OwnerUID || evidence.ObservedBootID == "" {
			return 0, errors.New("sandbox process retirement identity is invalid")
		}
		if !((evidence.Reason == "coalition_reaped" && evidence.ObservedBootID == registration.BootID) || (evidence.Reason == "boot_changed" && evidence.ObservedBootID != registration.BootID)) {
			return 0, errors.New("sandbox process retirement has no kernel scope evidence")
		}
	default:
		return 0, errors.New("unknown sandbox process phase")
	}
	return key.Generation, nil
}
