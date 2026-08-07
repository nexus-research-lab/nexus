package workspaceisolation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var errPathOutsidePolicy = errors.New("路径超出 runtime workspace policy")

// Identity 是 launcher 持久化的产品用户到 OS 身份映射。
type Identity struct {
	Username          string `json:"username"`
	UID               int    `json:"uid"`
	PrivateGID        int    `json:"private_gid"`
	SupplementaryGIDs []int  `json:"supplementary_gids"`
	HomeDir           string `json:"home_dir"`
	TempDir           string `json:"temp_dir"`
}

// Policy 是 launcher 与 PreToolUse hook 共用的不可变路径授权快照。
type Policy struct {
	OwnerUserID string   `json:"owner_user_id"`
	RuntimeKind string   `json:"runtime_kind"`
	CWD         string   `json:"cwd"`
	ReadRoots   []string `json:"read_roots"`
	WriteRoots  []string `json:"write_roots"`
	Generation  uint64   `json:"generation"`
	Ticket      string   `json:"ticket"`
	Identity    Identity `json:"identity"`
	// IsMainAgent 只由宿主在主智能体的 control-plane policy 中设置。
	// launcher 返回的普通 runtime policy 必须保持 false。
	IsMainAgent bool `json:"is_main_agent,omitempty"`
}

func (p *Policy) validate(expected Input, requireTicket bool) error {
	if p == nil {
		return errors.New("workspace policy 为空")
	}
	if strings.TrimSpace(p.OwnerUserID) != strings.TrimSpace(expected.OwnerUserID) {
		return errors.New("launcher 返回了不匹配的 owner")
	}
	if !strings.EqualFold(strings.TrimSpace(p.RuntimeKind), strings.TrimSpace(expected.RuntimeKind)) {
		return errors.New("launcher 返回了不匹配的 runtime")
	}
	expectedCWD, err := canonicalPolicyPath(expected.CWD)
	if err != nil {
		return fmt.Errorf("解析预期 workspace: %w", err)
	}
	actualCWD, err := canonicalPolicyPath(p.CWD)
	if err != nil {
		return fmt.Errorf("解析 launcher workspace: %w", err)
	}
	if !samePolicyPath(actualCWD, expectedCWD) {
		return errors.New("launcher 返回了不匹配的 workspace")
	}
	if requireTicket && strings.TrimSpace(p.Ticket) == "" {
		return errors.New("launcher 未返回策略票据")
	}
	if p.Generation == 0 && requireTicket {
		return errors.New("launcher 未返回有效的 policy generation")
	}
	if requireTicket {
		if p.Identity.UID <= 0 || p.Identity.PrivateGID <= 0 ||
			strings.TrimSpace(p.Identity.Username) == "" ||
			strings.TrimSpace(p.Identity.HomeDir) == "" ||
			strings.TrimSpace(p.Identity.TempDir) == "" {
			return errors.New("launcher 返回了无效 runtime identity")
		}
	}
	p.ReadRoots, err = normalizePolicyRoots(p.ReadRoots)
	if err != nil {
		return err
	}
	p.WriteRoots, err = normalizePolicyRoots(p.WriteRoots)
	if err != nil {
		return err
	}
	if len(p.ReadRoots) == 0 || len(p.WriteRoots) == 0 {
		return errors.New("launcher 返回了空路径策略")
	}
	if _, err = p.authorize(actualCWD, false); err != nil {
		return errors.New("launcher workspace 不在可读根内")
	}
	return nil
}

func normalizePolicyRoots(roots []string) ([]string, error) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		canonical, err := canonicalPolicyPath(root)
		if err != nil {
			return nil, fmt.Errorf("解析 policy root %q: %w", root, err)
		}
		normalized = append(normalized, canonical)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func (p Policy) authorize(path string, write bool) (string, error) {
	canonical, err := canonicalPolicyPath(path)
	if err != nil {
		return "", err
	}
	roots := p.ReadRoots
	if write {
		roots = p.WriteRoots
	}
	for _, root := range roots {
		if pathWithinPolicyRoot(canonical, root) {
			return canonical, nil
		}
	}
	return canonical, errPathOutsidePolicy
}

// canonicalPolicyPath 对存在路径解析完整 symlink；对待创建路径解析最近的
// 已存在祖先，防止父目录符号链接把最终系统调用重定向到授权根之外。
func canonicalPolicyPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("路径为空")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	current := absolute
	missing := make([]string, 0)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", resolveErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolute, nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func pathWithinPolicyRoot(path string, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if samePolicyPath(path, root) {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == "" || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) &&
		!filepath.IsAbs(relative)
}

func samePolicyPath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
