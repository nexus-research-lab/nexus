// INPUT: owner scope、可选 expected catalog version 与一次受控 catalog 写回调。
// OUTPUT: per-owner 串行、持久 CAS、typed reconcile 与提交后的单调 catalog version。
// POS: HTTP、CLI 与对话控制共用的 Skill 全局写入口；远端下载不得在锁内执行。
package skills

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

var (
	// ErrCatalogVersionConflict 表示 inspect/plan 绑定的 catalog version 已过期。
	ErrCatalogVersionConflict = skillstore.ErrCatalogVersionConflict
	// ErrCatalogVersionUnavailable 表示当前服务没有持久仓储，不能提供对话 CAS。
	ErrCatalogVersionUnavailable = errors.New("skill catalog version store not configured")
	// ErrCatalogSnapshotUnstable 表示连续读取期间 catalog 一直发生变化。
	ErrCatalogSnapshotUnstable = errors.New("skill catalog changed while reading snapshot")
)

// CatalogState 是 owner Skill 全局目录的持久版本快照。
type CatalogState struct {
	Version int64 `json:"version"`
}

// CatalogSkillState 是不暴露本地路径、上传内容或健康噪声的稳定 Skill 目标快照。
type CatalogSkillState struct {
	CatalogVersion int64  `json:"catalog_version"`
	Exists         bool   `json:"exists"`
	Name           string `json:"name,omitempty"`
	Title          string `json:"title,omitempty"`
	Description    string `json:"description,omitempty"`
	Scope          string `json:"scope,omitempty"`
	SourceType     string `json:"source_type,omitempty"`
	SourceKind     string `json:"source_kind,omitempty"`
	SourceTrust    string `json:"source_trust,omitempty"`
	ImportMode     string `json:"import_mode,omitempty"`
	Version        string `json:"version,omitempty"`
	StorageScope   string `json:"storage_scope,omitempty"`
	OriginKind     string `json:"origin_kind,omitempty"`
	SourceIdentity string `json:"source_identity,omitempty"`
	Deletable      bool   `json:"deletable,omitempty"`
}

// CatalogSourceState 是绑定 catalog version 的 marketplace 来源功能配置。
type CatalogSourceState struct {
	CatalogVersion       int64  `json:"catalog_version"`
	Exists               bool   `json:"exists"`
	SourceID             string `json:"source_id,omitempty"`
	Name                 string `json:"name,omitempty"`
	Kind                 string `json:"kind,omitempty"`
	URL                  string `json:"url,omitempty"`
	Trust                string `json:"trust,omitempty"`
	Enabled              bool   `json:"enabled,omitempty"`
	SortOrder            int    `json:"sort_order,omitempty"`
	ManagedBy            string `json:"managed_by,omitempty"`
	AuthType             string `json:"auth_type,omitempty"`
	CredentialConfigured bool   `json:"credential_configured,omitempty"`
	Deletable            bool   `json:"deletable,omitempty"`
}

// CatalogReconcileError 表示 catalog 已经提交或文件发布状态不确定，需要核对修复。
type CatalogReconcileError struct {
	applied bool
	cause   error
}

func (e *CatalogReconcileError) Error() string {
	if e == nil {
		return "Skill catalog 需要 reconcile"
	}
	state := "文件发布状态不确定"
	if e.applied {
		state = "catalog 变更已提交"
	}
	return fmt.Sprintf("Skill %s，但后置步骤需要 reconcile: %v", state, e.cause)
}

func (e *CatalogReconcileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// SkillMutationNeedsReconcile 判断错误是否代表不能按普通失败或成功结案。
func SkillMutationNeedsReconcile(err error) bool {
	var target *CatalogReconcileError
	return errors.As(err, &target)
}

// SkillMutationApplied 判断 catalog 数据库版本是否已经提交。
func SkillMutationApplied(err error) bool {
	var target *CatalogReconcileError
	return errors.As(err, &target) && target.applied
}

// GetCatalogState 返回当前 owner 的持久单调版本。
func (s *Service) GetCatalogState(ctx context.Context) (CatalogState, error) {
	if s.skillStore == nil {
		return CatalogState{}, ErrCatalogVersionUnavailable
	}
	version, err := s.skillStore.CatalogVersion(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return CatalogState{}, err
	}
	return CatalogState{Version: version}, nil
}

// GetCatalogSkillState 以 version-before/read/version-after 返回稳定目标快照。
//
// SourceRef 可能是 human-only local_path，因此本接口只返回 source_identity，
// 不把宿主路径或上传内容交给对话控制面。
func (s *Service) GetCatalogSkillState(
	ctx context.Context,
	skillName string,
) (CatalogSkillState, error) {
	if s.skillStore == nil {
		return CatalogSkillState{}, ErrCatalogVersionUnavailable
	}
	for range 4 {
		before, err := s.GetCatalogState(ctx)
		if err != nil {
			return CatalogSkillState{}, err
		}
		records, err := s.loadCatalogRecords(ctx)
		if err != nil {
			return CatalogSkillState{}, err
		}
		record, exists := findCatalogRecord(records, strings.TrimSpace(skillName))
		after, err := s.GetCatalogState(ctx)
		if err != nil {
			return CatalogSkillState{}, err
		}
		if before.Version != after.Version {
			continue
		}
		state := CatalogSkillState{
			CatalogVersion: after.Version,
			Exists:         exists,
		}
		if !exists {
			return state, nil
		}
		info := selectionInfo(record)
		state.Name = info.Name
		state.Title = info.Title
		state.Description = info.Description
		state.Scope = info.Scope
		state.SourceType = info.SourceType
		state.SourceKind = info.SourceKind
		state.SourceTrust = info.SourceTrust
		state.ImportMode = info.ImportMode
		state.Version = info.Version
		state.StorageScope = info.StorageScope
		state.OriginKind = info.OriginKind
		state.SourceIdentity = info.SourceIdentity
		state.Deletable = info.Deletable
		return state, nil
	}
	return CatalogSkillState{}, ErrCatalogSnapshotUnstable
}

// GetCatalogSourceState 返回不包含 last_checked/last_error 健康噪声的稳定来源快照。
func (s *Service) GetCatalogSourceState(
	ctx context.Context,
	sourceID string,
) (CatalogSourceState, error) {
	if s.skillStore == nil {
		return CatalogSourceState{}, ErrCatalogVersionUnavailable
	}
	sourceID = strings.TrimSpace(sourceID)
	for range 4 {
		before, err := s.GetCatalogState(ctx)
		if err != nil {
			return CatalogSourceState{}, err
		}
		sources, err := s.ListExternalSkillSources(ctx)
		if err != nil {
			return CatalogSourceState{}, err
		}
		var target *ExternalSkillSourceInfo
		for index := range sources {
			if strings.TrimSpace(sources[index].SourceID) == sourceID {
				target = &sources[index]
				break
			}
		}
		after, err := s.GetCatalogState(ctx)
		if err != nil {
			return CatalogSourceState{}, err
		}
		if before.Version != after.Version {
			continue
		}
		state := CatalogSourceState{
			CatalogVersion: after.Version,
			Exists:         target != nil,
		}
		if target == nil {
			return state, nil
		}
		state.SourceID = target.SourceID
		state.Name = target.Name
		state.Kind = target.Kind
		state.URL = target.URL
		state.Trust = target.Trust
		state.Enabled = target.Enabled
		state.SortOrder = target.SortOrder
		state.ManagedBy = target.ManagedBy
		state.AuthType = target.AuthType
		state.CredentialConfigured = target.CredentialConfigured
		state.Deletable = target.Deletable
		return state, nil
	}
	return CatalogSourceState{}, ErrCatalogSnapshotUnstable
}

func (s *Service) lockCatalogMutation(ctx context.Context) func() {
	ownerUserID := authctx.OwnerUserID(ctx)
	value, _ := s.mutationLocks.LoadOrStore(ownerUserID, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (s *Service) withCatalogMutation(
	ctx context.Context,
	expectedVersion *int64,
	bumpVersion bool,
	callback func(*skillstore.CatalogMutation) error,
) (int64, error) {
	unlock := s.lockCatalogMutation(ctx)
	defer unlock()

	if expectedVersion != nil && s.skillStore == nil {
		return 0, ErrCatalogVersionUnavailable
	}
	if s.skillStore == nil {
		return 0, callback(nil)
	}
	mutation, err := s.skillStore.BeginCatalogMutation(
		ctx,
		authctx.OwnerUserID(ctx),
		expectedVersion,
		bumpVersion,
	)
	if err != nil {
		return 0, err
	}
	defer mutation.Rollback()
	if err = callback(mutation); err != nil {
		return 0, err
	}
	if err = mutation.Commit(); err != nil {
		return 0, err
	}
	return mutation.Version(), nil
}
