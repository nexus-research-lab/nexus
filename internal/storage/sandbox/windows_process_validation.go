// INPUT: 独立 Windows 启动意图和从数据库读取的完整快照。
// OUTPUT: 精确身份、用途与单调阶段验证，未知记录不被当成启动许可。
// POS: Windows repository 持久事实校验，不采用 Darwin process 字段。
package sandbox

import (
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var ErrWindowsSandboxConflict = errors.New("Windows sandbox durable launch conflicts or remains unknown")

func validWindowsSandboxNonce(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && hex.EncodeToString(decoded) == value && strings.Trim(value, "0") != ""
}

func validateWindowsSandboxKey(key protocol.WindowsSandboxKey) error {
	if key.Generation == 0 || key.Generation >= math.MaxInt64 || !validWindowsSandboxNonce(key.LaunchID, 16) {
		return ErrWindowsSandboxConflict
	}
	for _, text := range []string{key.OwnerUserID, key.SessionKey} {
		if strings.TrimSpace(text) == "" || len(text) > 1024 || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
			return ErrWindowsSandboxConflict
		}
	}
	return nil
}

func validateWindowsSandboxIntent(intent protocol.WindowsSandboxIntent) error {
	if err := validateWindowsSandboxKey(intent.Key); err != nil {
		return err
	}
	_, purpose := protocol.WindowsSandboxPurposeOrder(intent.Purpose)
	if intent.ScratchRoot == "" || len(intent.ScratchRoot) > 32768 || !utf8.ValidString(intent.ScratchRoot) || strings.ContainsRune(intent.ScratchRoot, 0) {
		return ErrWindowsSandboxConflict
	}
	if intent.Version != 1 || !purpose || !validWindowsSandboxNonce(intent.HelperSHA256, 32) || intent.PolicyDigest == ([32]byte{}) || intent.OptionsDigest == ([32]byte{}) || intent.CommandDigest == ([32]byte{}) || strings.TrimSpace(intent.LeaseID) == "" || !utf8.ValidString(intent.LeaseID) || len(intent.LeaseID) > 1024 || strings.ContainsRune(intent.LeaseID, 0) {
		return ErrWindowsSandboxConflict
	}
	return nil
}

func validWindowsSandboxPrepared(value protocol.WindowsSandboxPrepared) bool {
	return validWindowsSandboxNonce(value.ExecutionID, 16) && value.PrepareDigest != ([32]byte{}) && value.ManifestDigest != ([32]byte{})
}

func validateWindowsSandboxSnapshot(snapshot protocol.WindowsSandboxSnapshot) error {
	if err := validateWindowsSandboxIntent(snapshot.Intent); err != nil {
		return err
	}
	if snapshot.Prepared != nil && !validWindowsSandboxPrepared(*snapshot.Prepared) {
		return ErrWindowsSandboxConflict
	}
	switch snapshot.Phase {
	case "reserved":
		if snapshot.Prepared != nil || snapshot.Outcome != nil {
			return ErrWindowsSandboxConflict
		}
	case "prepared", "started":
		if snapshot.Prepared == nil || snapshot.Outcome != nil {
			return ErrWindowsSandboxConflict
		}
	case "unknown", "cleaned":
		if snapshot.Outcome == nil {
			return ErrWindowsSandboxConflict
		}
		outcome := *snapshot.Outcome
		if snapshot.Prepared == nil {
			if outcome.Prepared != (protocol.WindowsSandboxPrepared{}) {
				return ErrWindowsSandboxConflict
			}
		} else if outcome.Prepared != *snapshot.Prepared {
			return ErrWindowsSandboxConflict
		}
		if outcome.Cleaned != (snapshot.Phase == "cleaned") {
			return ErrWindowsSandboxConflict
		}
		if outcome.Cleaned {
			if snapshot.Prepared == nil || outcome.Reason != "" {
				return ErrWindowsSandboxConflict
			}
		} else if strings.TrimSpace(outcome.Reason) == "" || len(outcome.Reason) > 2048 || !utf8.ValidString(outcome.Reason) {
			return ErrWindowsSandboxConflict
		}
	default:
		return ErrWindowsSandboxConflict
	}
	return nil
}
