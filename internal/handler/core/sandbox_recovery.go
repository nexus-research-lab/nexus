// INPUT: 已认证的 owner 请求、桌面 runtime sandbox durable marker。
// OUTPUT: owner-scoped 的资源 inspect 与显式 stale-reconcile HTTP 响应。
// POS: 只提供诊断/恢复入口；不替代 runtime lease，也不自动删除 cleanup_unknown。
package core

import (
	"errors"
	"net/http"
	"os"
	"time"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const maxSandboxRecoveryAgeSeconds int64 = 100 * 365 * 24 * 60 * 60

// sandboxRecoveryMarker is the owner-safe projection of a runtime marker.
// Filesystem identity and raw cleanup errors stay host-private: error strings
// can contain paths, while the UI only needs lifecycle state and stable scope
// labels to explain a recovery result.
type sandboxRecoveryMarker struct {
	Version          int       `json:"version"`
	LeaseID          string    `json:"lease_id"`
	OwnerUserID      string    `json:"owner_user_id"`
	SessionKey       string    `json:"session_key"`
	RoundID          string    `json:"round_id"`
	ProcessID        int       `json:"process_id"`
	CreatedAt        time.Time `json:"created_at"`
	CleanupState     string    `json:"cleanup_state,omitempty"`
	CleanupUpdatedAt time.Time `json:"cleanup_updated_at,omitempty"`
}

// sandboxRecoveryRecord is the stable, JSON-safe projection of a runtime
// marker. The runtime path, canonical root, and raw cleanup error remain
// private to the host.
type sandboxRecoveryRecord struct {
	Marker        sandboxRecoveryMarker `json:"marker"`
	AgeSeconds    int64                 `json:"age_seconds"`
	ProcessActive bool                  `json:"process_active"`
}

type sandboxRecoveryRequest struct {
	OlderThanSeconds int64 `json:"older_than_seconds"`
	Apply            bool  `json:"apply"`
}

type sandboxRecoveryInspection struct {
	OwnerUserID string                  `json:"owner_user_id"`
	Resources   []sandboxRecoveryRecord `json:"resources"`
}

type sandboxRecoveryResult struct {
	OwnerUserID      string                  `json:"owner_user_id"`
	OlderThanSeconds int64                   `json:"older_than_seconds"`
	Apply            bool                    `json:"apply"`
	Candidates       []sandboxRecoveryRecord `json:"candidates"`
	Removed          []sandboxRecoveryRecord `json:"removed"`
	Skipped          []sandboxRecoveryRecord `json:"skipped"`
}

// HandleSandboxResourceInspection lists only the current authenticated
// owner's valid durable markers. It never accepts a root or owner from the
// request, so a browser cannot inspect another owner's runtime directory.
func (h *Handlers) HandleSandboxResourceInspection(writer http.ResponseWriter, request *http.Request) {
	owner := currentOwnerUserID(request)
	writer.Header().Set("Cache-Control", "no-store")
	if owner == "" {
		h.api.WriteFailure(writer, http.StatusNotFound, "运行时沙箱资源不存在")
		return
	}
	records, err := runtimectx.DiscoverSandboxResources(request.Context(), runtimectx.SandboxResourceSweepInput{
		OwnerUserID: owner,
	})
	if err != nil {
		// A first-run owner may not have a runtime root yet. This is an empty
		// inventory, not evidence that another root is safe to inspect.
		if errors.Is(err, os.ErrNotExist) {
			h.api.WriteSuccess(writer, sandboxRecoveryInspection{
				OwnerUserID: owner,
				Resources:   []sandboxRecoveryRecord{},
			})
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, "运行时沙箱资源读取失败")
		return
	}
	h.api.WriteSuccess(writer, sandboxRecoveryInspection{
		OwnerUserID: owner,
		Resources:   projectSandboxRecoveryRecords(records),
	})
}

// HandleSandboxResourceReconcile performs an explicit owner-scoped stale
// sweep. The default is a dry run; deletion requires a JSON body with a
// positive age and apply=true. cleanup_unknown and active/unknown processes
// remain protected by the runtime sweep regardless of that flag.
func (h *Handlers) HandleSandboxResourceReconcile(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	var payload sandboxRecoveryRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if payload.OlderThanSeconds <= 0 || payload.OlderThanSeconds > maxSandboxRecoveryAgeSeconds {
		h.api.WriteFailure(writer, http.StatusBadRequest, "回收年龄必须为正数且不超过 100 年")
		return
	}
	owner := currentOwnerUserID(request)
	if owner == "" {
		h.api.WriteFailure(writer, http.StatusNotFound, "运行时沙箱资源不存在")
		return
	}
	result, err := runtimectx.SweepStaleSandboxResources(request.Context(), runtimectx.SandboxResourceSweepInput{
		OwnerUserID: owner,
		OlderThan:   time.Duration(payload.OlderThanSeconds) * time.Second,
		Apply:       payload.Apply,
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.api.WriteSuccess(writer, sandboxRecoveryResult{
				OwnerUserID:      owner,
				OlderThanSeconds: payload.OlderThanSeconds,
				Apply:            payload.Apply,
				Candidates:       []sandboxRecoveryRecord{},
				Removed:          []sandboxRecoveryRecord{},
				Skipped:          []sandboxRecoveryRecord{},
			})
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, "运行时沙箱资源回收失败")
		return
	}
	h.api.WriteSuccess(writer, sandboxRecoveryResult{
		OwnerUserID:      owner,
		OlderThanSeconds: payload.OlderThanSeconds,
		Apply:            payload.Apply,
		Candidates:       projectSandboxRecoveryRecords(result.Candidates),
		Removed:          projectSandboxRecoveryRecords(result.Removed),
		Skipped:          projectSandboxRecoveryRecords(result.Skipped),
	})
}

func projectSandboxRecoveryRecords(records []runtimectx.SandboxResourceRecord) []sandboxRecoveryRecord {
	if len(records) == 0 {
		return []sandboxRecoveryRecord{}
	}
	result := make([]sandboxRecoveryRecord, 0, len(records))
	for _, record := range records {
		result = append(result, sandboxRecoveryRecord{
			Marker: sandboxRecoveryMarker{
				Version:          record.Marker.Version,
				LeaseID:          record.Marker.LeaseID,
				OwnerUserID:      record.Marker.OwnerUserID,
				SessionKey:       record.Marker.SessionKey,
				RoundID:          record.Marker.RoundID,
				ProcessID:        record.Marker.ProcessID,
				CreatedAt:        record.Marker.CreatedAt,
				CleanupState:     record.Marker.CleanupState,
				CleanupUpdatedAt: record.Marker.CleanupUpdatedAt,
			},
			AgeSeconds:    int64(record.Age / time.Second),
			ProcessActive: record.ProcessActive,
		})
	}
	return result
}
