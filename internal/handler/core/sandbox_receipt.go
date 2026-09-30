// INPUT: 已认证 owner、精确 runtime session key 与可选 durable receipt store。
// OUTPUT: 当前或最近一次 generation 的桌面沙箱生效回执；无回执时返回 not found。
// POS: 只读诊断投影，不把 Bridge 能力协商或 host lease 当作 OS 隔离证明。
package core

import (
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type sandboxPolicyReceiptResponse struct {
	SessionKey string                                    `json:"session_key"`
	Receipt    *runtimectx.SandboxEffectivePolicyReceipt `json:"receipt"`
}

func redactSandboxPolicyReceipt(receipt *runtimectx.SandboxEffectivePolicyReceipt) *runtimectx.SandboxEffectivePolicyReceipt {
	if receipt == nil {
		return nil
	}
	copyReceipt := *receipt
	if receipt.ResourcePolicy != nil {
		policy := *receipt.ResourcePolicy
		// ScratchRoot is an owner-local absolute path. It is useful to the
		// host runtime, but must not become a browser-visible filesystem probe.
		policy.ScratchRoot = ""
		copyReceipt.ResourcePolicy = &policy
	}
	// UnknownReason is persisted for host-side reconciliation and may contain
	// paths, command lines or SDK error detail. The browser only needs a stable
	// lifecycle category, so never project the raw diagnostic string.
	if receipt.Phase == protocol.SandboxPolicyReceiptUnknown && strings.TrimSpace(receipt.UnknownReason) != "" {
		copyReceipt.UnknownReason = "cleanup_unknown"
	} else {
		copyReceipt.UnknownReason = ""
	}
	return &copyReceipt
}

// HandleSandboxPolicyReceipt returns a diagnostic receipt for one exact,
// owner-scoped runtime session.  The session key is a lookup hint only; the
// runtime manager performs the owner check before exposing any receipt.
func (h *Handlers) HandleSandboxPolicyReceipt(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	sessionKey := strings.TrimSpace(request.URL.Query().Get("session_key"))
	if sessionKey == "" || h.runtime == nil {
		h.api.WriteFailure(writer, http.StatusNotFound, "运行时沙箱回执不存在")
		return
	}
	ownerUserID := currentOwnerUserID(request)
	if ownerUserID == "" {
		h.api.WriteFailure(writer, http.StatusNotFound, "运行时沙箱回执不存在")
		return
	}
	receipt := h.runtime.SandboxPolicyReceipt(ownerUserID, sessionKey)
	if receipt == nil {
		var found bool
		var err error
		receipt, found, err = h.runtime.PersistentSandboxPolicyReceipt(request.Context(), ownerUserID, sessionKey)
		if err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, "运行时沙箱回执读取失败")
			return
		}
		if !found || receipt == nil {
			h.api.WriteFailure(writer, http.StatusNotFound, "运行时沙箱回执不存在")
			return
		}
	}
	h.api.WriteSuccess(writer, sandboxPolicyReceiptResponse{SessionKey: sessionKey, Receipt: redactSandboxPolicyReceipt(receipt)})
}
