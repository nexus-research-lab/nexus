// INPUT: 宿主启动意图、集合登记与回收事实。
// OUTPUT: exact 身份和单调阶段的校验；存储不猜测原生内核状态。
// POS: 生命周期持久边界，禁止用用户 scratch 标记充当可信登记。
package sandbox

import (
	"encoding/hex"
	"errors"
	"math"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var ErrProcessConflict = errors.New("sandbox process lifecycle conflict")
var ErrInvalidProcess = errors.New("invalid sandbox process registration")

func validProcessKey(k protocol.SandboxProcessKey) bool {
	return strings.TrimSpace(k.OwnerUserID) != "" && len(k.OwnerUserID) <= 512 && strings.TrimSpace(k.SessionKey) != "" && len(k.SessionKey) <= 1024 && k.Generation > 0 && k.Generation < math.MaxInt64 && validLowerHex(k.LaunchID, 32)
}
func validLowerHex(value string, length int) bool {
	if len(value) != length || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func validProcessBoot(value string) bool {
	return len(value) == 36 && value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-' && validLowerHex(strings.ReplaceAll(value, "-", ""), 32)
}
func validateProcessIntent(i protocol.SandboxProcessIntent) error {
	if _, ok := i.Purpose.Order(); !ok {
		return ErrInvalidProcess
	}
	if !validProcessKey(i.Key) || i.Version != 1 || (i.RuntimeKind != "nxs" && i.RuntimeKind != "claude") || !validProcessBoot(i.BootID) || i.JobLabel != "cn.nexus.runtime."+i.Key.LaunchID || !validLowerHex(i.HelperSHA256, 64) || len(i.LeaseID) > 512 {
		return ErrInvalidProcess
	}
	scratch := i.Scratch
	if scratch != (protocol.SandboxProcessScratch{}) {
		if i.LeaseID == "" || !filepath.IsAbs(scratch.BasePath) || filepath.Clean(scratch.BasePath) != scratch.BasePath || len(scratch.BasePath) > 4096 || filepath.Base(scratch.BasePath) != "sandbox" || !strings.HasPrefix(scratch.LeafName, ".scratch-") || filepath.Base(scratch.LeafName) != scratch.LeafName || strings.ContainsAny(scratch.LeafName, `/\\`+"\x00") || len(scratch.LeafName) > 255 || !validScratchIdentity(scratch.BaseIdentity) || !validScratchIdentity(scratch.LeafIdentity) {
			return ErrInvalidProcess
		}
	}
	return nil
}
func validScratchIdentity(value string) bool {
	fields := strings.Split(value, ":")
	if len(fields) != 6 || fields[0] != "darwin-v1" {
		return false
	}
	for _, field := range fields[1:] {
		if len(field) == 0 || len(field) > 16 {
			return false
		}
		for _, c := range field {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
func validateProcessRegistration(i protocol.SandboxProcessIntent, r protocol.SandboxProcessRegistration) error {
	if r.Version != 1 || r.CoalitionID == 0 || r.BootID != i.BootID || r.OwnerUID != i.OwnerUID {
		return ErrInvalidProcess
	}
	return nil
}
func validateProcessEvidence(r protocol.SandboxProcessRegistration, e protocol.SandboxProcessEvidence) error {
	if e.Registration != r || !validProcessBoot(e.ObservedBootID) {
		return ErrInvalidProcess
	}
	switch e.Reason {
	case "coalition_reaped":
		if e.ObservedBootID != r.BootID {
			return ErrInvalidProcess
		}
	case "boot_changed":
		if e.ObservedBootID == r.BootID {
			return ErrInvalidProcess
		}
	default:
		return ErrInvalidProcess
	}
	return nil
}
func validateProcessSnapshot(s protocol.SandboxProcessSnapshot) error {
	if err := validateProcessIntent(s.Intent); err != nil {
		return err
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		return ErrInvalidProcess
	}
	switch s.Phase {
	case protocol.SandboxProcessPrepared, protocol.SandboxProcessAborted:
		if s.Registration != nil || s.Evidence != nil {
			return ErrInvalidProcess
		}
	case protocol.SandboxProcessRegistered, protocol.SandboxProcessReleased, protocol.SandboxProcessReaped:
		if s.Registration == nil {
			return ErrInvalidProcess
		}
		if err := validateProcessRegistration(s.Intent, *s.Registration); err != nil {
			return err
		}
		if s.Phase == protocol.SandboxProcessReaped {
			if s.Evidence == nil {
				return ErrInvalidProcess
			}
			return validateProcessEvidence(*s.Registration, *s.Evidence)
		}
		if s.Evidence != nil {
			return ErrInvalidProcess
		}
	default:
		return ErrInvalidProcess
	}
	return nil
}
