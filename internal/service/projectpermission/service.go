package projectpermission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// Project 描述一个由 launcher 管理的共享项目。
type Project struct {
	ProjectID  string            `json:"project_id"`
	GroupName  string            `json:"group_name"`
	GID        int               `json:"gid"`
	Root       string            `json:"root"`
	Members    map[string]string `json:"members"`
	Generation uint64            `json:"generation"`
}

// EnsureResult 表示项目 ensure 是否首次创建。
type EnsureResult struct {
	Project Project `json:"project"`
	Created bool    `json:"created"`
}

// GrantResult 表示成员 ACL 是否发生了实际变化。
type GrantResult struct {
	Changed bool `json:"changed"`
}

var (
	// ErrUnavailable 表示当前部署没有启用 Linux launcher 控制面。
	ErrUnavailable = errors.New("project permission control plane unavailable")
	// ErrInvalidProjectID 表示项目标识不能映射成安全路径段。
	ErrInvalidProjectID = errors.New("invalid project id")
	// ErrInvalidOwnerUserID 表示成员标识不能安全映射到 runtime identity。
	ErrInvalidOwnerUserID = errors.New("invalid owner user id")
	// ErrInvalidAccess 表示项目成员权限不在受控枚举中。
	ErrInvalidAccess = errors.New("project access must be read, write or none")
	// ErrRuntimeCleanup 表示 ACL 已更新，但旧 runtime 未能确认回收。
	ErrRuntimeCleanup = errors.New("owner runtime cleanup failed")
)

const runtimeCleanupTimeout = 30 * time.Second

// Service 是 Nexus 宿主调用 root-owned launcher 的项目权限门面。
type Service struct {
	config        config.Config
	runCommand    func(context.Context, []string) ([]byte, error)
	runtimeCloser OwnerRuntimeCloser
}

// OwnerRuntimeCloser 在项目成员关系变化后回收旧的进程凭据。
type OwnerRuntimeCloser interface {
	CloseOwnerSessions(ctx context.Context, ownerUserID string) (int, error)
}

// NewService 创建项目权限服务。
func NewService(cfg config.Config) *Service {
	service := &Service{config: cfg}
	service.runCommand = service.executeLauncher
	return service
}

// SetRuntimeSessionCloser 注入 owner 级 runtime 回收器。
func (s *Service) SetRuntimeSessionCloser(closer OwnerRuntimeCloser) {
	s.runtimeCloser = closer
}

// List 返回 launcher registry 中的共享项目。
func (s *Service) List(ctx context.Context) ([]Project, error) {
	var result []Project
	if err := s.runJSON(ctx, []string{"project-list"}, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []Project{}
	}
	return result, nil
}

// Ensure 创建或修复一个共享项目根。
func (s *Service) Ensure(ctx context.Context, projectID string) (EnsureResult, error) {
	projectID, err := normalizeProjectID(projectID)
	if err != nil {
		return EnsureResult{}, err
	}
	var result EnsureResult
	err = s.runJSON(ctx, []string{
		"project-ensure",
		"--project", projectID,
		"--path", filepath.Join(appfs.StateRoot(), "shared-workspaces", projectID),
		"--owner", authctx.OwnerUserID(ctx),
	}, &result)
	return result, err
}

// Grant 修改项目成员的 read/write/none 授权。
func (s *Service) Grant(
	ctx context.Context,
	projectID string,
	ownerUserID string,
	access string,
) (GrantResult, error) {
	projectID, err := normalizeProjectID(projectID)
	if err != nil {
		return GrantResult{}, err
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" || ownerUserID == "." || ownerUserID == ".." ||
		appfs.UserPathSegment(ownerUserID) != ownerUserID {
		return GrantResult{}, ErrInvalidOwnerUserID
	}
	access = strings.ToLower(strings.TrimSpace(access))
	switch access {
	case "read", "write", "none":
	default:
		return GrantResult{}, ErrInvalidAccess
	}
	var result GrantResult
	err = s.runJSON(ctx, []string{
		"project-grant",
		"--project", projectID,
		"--owner", ownerUserID,
		"--access", access,
	}, &result)
	if err != nil {
		return GrantResult{}, err
	}
	if !result.Changed || s.runtimeCloser == nil {
		return result, nil
	}

	// ACL 已经落盘后，客户端断开不能取消安全回收，否则旧进程仍可能持有
	// 变更前的 supplementary GID。保留 trace/value，但切断取消链并设总超时。
	cleanupParent := context.WithoutCancel(ctx)
	cleanupCtx, cancel := context.WithTimeout(cleanupParent, runtimeCleanupTimeout)
	defer cancel()
	if _, err = s.runtimeCloser.CloseOwnerSessions(cleanupCtx, ownerUserID); err != nil {
		return result, fmt.Errorf("project permission updated: %w: %v", ErrRuntimeCleanup, err)
	}
	return result, nil
}

func (s *Service) runJSON(ctx context.Context, args []string, target any) error {
	payload, err := s.run(ctx, args)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode launcher response: %w", err)
	}
	return nil
}

func (s *Service) run(ctx context.Context, args []string) ([]byte, error) {
	if s.runCommand == nil {
		return s.executeLauncher(ctx, args)
	}
	return s.runCommand(ctx, args)
}

func (s *Service) executeLauncher(ctx context.Context, args []string) ([]byte, error) {
	if runtime.GOOS != "linux" ||
		!strings.EqualFold(strings.TrimSpace(s.config.RuntimeIsolationMode), "enforce") {
		return nil, ErrUnavailable
	}
	launcherPath := strings.TrimSpace(s.config.RuntimeLauncherPath)
	if launcherPath == "" || !filepath.IsAbs(launcherPath) {
		return nil, errors.New("runtime launcher path is not configured")
	}
	command := exec.CommandContext(ctx, launcherPath, args...)
	command.Env = []string{
		"PATH=/usr/bin:/bin",
		"LANG=C",
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("runtime launcher: %s: %w", detail, err)
		}
		return nil, fmt.Errorf("runtime launcher: %w", err)
	}
	return stdout.Bytes(), nil
}

func normalizeProjectID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || value == "." || value == ".." ||
		appfs.UserPathSegment(value) != value ||
		strings.ContainsAny(value, `/\`) {
		return "", ErrInvalidProjectID
	}
	return value, nil
}
