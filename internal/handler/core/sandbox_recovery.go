// INPUT: 已认证的 owner 请求、桌面 runtime sandbox durable marker 与启动恢复失败项。
// OUTPUT: owner-scoped 的资源 inspect、显式 stale-reconcile 与启动恢复失败项放弃 HTTP 响应。
// POS: 只提供诊断/恢复入口；不替代 runtime lease，也不自动删除 cleanup_unknown。
package core

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
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
	OwnerUserID     string                        `json:"owner_user_id"`
	Resources       []sandboxRecoveryRecord       `json:"resources"`
	BlockedSessions []sandboxBlockedSessionRecord `json:"blocked_sessions"`
}

// sandboxBlockedSessionRecord is one record that startup recovery left
// pending. The raw recovery error can contain host paths and stays in the
// host log; the UI only needs the scope and whether the owner may discard it.
type sandboxBlockedSessionRecord struct {
	LaunchID    string `json:"launch_id"`
	SessionKey  string `json:"session_key"`
	Stage       string `json:"stage"`
	Discardable bool   `json:"discardable"`
}

type sandboxRecoveryDiscardRequest struct {
	LaunchID string `json:"launch_id"`
}

const sandboxRecoveryDiscardReason = "owner discarded the unrecoverable record from settings"

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
				OwnerUserID:     owner,
				Resources:       []sandboxRecoveryRecord{},
				BlockedSessions: h.blockedSandboxSessions(owner),
			})
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, "运行时沙箱资源读取失败")
		return
	}
	h.api.WriteSuccess(writer, sandboxRecoveryInspection{
		OwnerUserID:     owner,
		Resources:       projectSandboxRecoveryRecords(records),
		BlockedSessions: h.blockedSandboxSessions(owner),
	})
}

func (h *Handlers) blockedSandboxSessions(owner string) []sandboxBlockedSessionRecord {
	result := []sandboxBlockedSessionRecord{}
	if h.runtime == nil {
		return result
	}
	for _, failure := range h.runtime.SandboxStartupRecoveryFailures(owner) {
		result = append(result, sandboxBlockedSessionRecord{
			LaunchID:    failure.Key.LaunchID,
			SessionKey:  failure.Key.SessionKey,
			Stage:       string(failure.Stage),
			Discardable: failure.Discardable(),
		})
	}
	return result
}

// HandleSandboxRecoveryDiscard lets the owner close one record that startup
// recovery could not finish, so its session can start again. Only terminal
// (lifecycle-stage) records qualify; a record whose process may be alive is
// rejected. The owner and scope come from the authenticated request and the
// host's own failure list, never from a caller-supplied path.
func (h *Handlers) HandleSandboxRecoveryDiscard(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	var payload sandboxRecoveryDiscardRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	owner := currentOwnerUserID(request)
	if owner == "" || h.runtime == nil || strings.TrimSpace(payload.LaunchID) == "" {
		h.api.WriteFailure(writer, http.StatusNotFound, "沙箱恢复记录不存在")
		return
	}
	err := h.runtime.DiscardSandboxStartupRecovery(request.Context(), owner, payload.LaunchID, sandboxRecoveryDiscardReason)
	switch {
	case err == nil:
		h.api.WriteSuccess(writer, map[string]any{"launch_id": strings.TrimSpace(payload.LaunchID), "discarded": true})
	case errors.Is(err, runtimectx.ErrSandboxRecoveryNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "沙箱恢复记录不存在")
	case errors.Is(err, runtimectx.ErrSandboxRecoveryNotDiscardable):
		h.api.WriteFailure(writer, http.StatusConflict, "原进程可能仍在运行，不能放弃该记录；请退出 Nexus 后重新启动再试")
	default:
		slog.Warn("sandbox startup recovery discard failed", "owner_user_id", owner, "launch_id", payload.LaunchID, "err", err)
		h.api.WriteFailure(writer, http.StatusInternalServerError, "放弃沙箱恢复记录失败")
	}
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
