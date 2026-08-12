// INPUT: 模型或持久化命令声明的 Work Item output scope 与 hard dependency 图。
// OUTPUT: typed canonical scope、跨平台 NFC/case-fold 比较键、稳定校验错误与唯一 claim conflict 判定。
// POS: Execution Orchestration 的 output scope 语法和顺序所有权真相源，供 service 与 storage 共同使用。
package protocol

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// ErrInvalidWorkOutputScope 表示 output scope 不符合 typed canonical grammar。
var ErrInvalidWorkOutputScope = errors.New("invalid Work Item output scope")

// WorkOutputScopeKind 表示 output scope 所声明资源的类型。
type WorkOutputScopeKind string

const (
	WorkOutputScopeKindFile     WorkOutputScopeKind = "file"
	WorkOutputScopeKindDir      WorkOutputScopeKind = "dir"
	WorkOutputScopeKindSemantic WorkOutputScopeKind = "semantic"
)

// CanonicalWorkOutputScope 是解析后的 typed canonical scope。
//
// file 与 dir 的 Value 始终是 workspace-relative POSIX clean path；
// semantic 的 Value 是去除首尾空白后的非空 key。
type CanonicalWorkOutputScope struct {
	Kind  WorkOutputScopeKind
	Value string
}

// String 返回可持久化、可注入模型上下文的 canonical scope。
func (scope CanonicalWorkOutputScope) String() string {
	return string(scope.Kind) + ":" + scope.Value
}

// ParseWorkOutputScope 解析 file:<path>、dir:<path> 或 semantic:<key>。
func ParseWorkOutputScope(raw string) (CanonicalWorkOutputScope, error) {
	raw = strings.TrimSpace(raw)
	kindText, value, found := strings.Cut(raw, ":")
	if !found {
		return CanonicalWorkOutputScope{}, fmt.Errorf(
			"%w: expected file:<path>, dir:<path>, or semantic:<key>",
			ErrInvalidWorkOutputScope,
		)
	}
	kind := WorkOutputScopeKind(strings.TrimSpace(kindText))
	value = strings.TrimSpace(value)
	switch kind {
	case WorkOutputScopeKindFile, WorkOutputScopeKindDir:
		canonical, err := canonicalWorkspacePath(value)
		if err != nil {
			return CanonicalWorkOutputScope{}, err
		}
		return CanonicalWorkOutputScope{Kind: kind, Value: canonical}, nil
	case WorkOutputScopeKindSemantic:
		if value == "" {
			return CanonicalWorkOutputScope{}, fmt.Errorf(
				"%w: semantic key must not be empty",
				ErrInvalidWorkOutputScope,
			)
		}
		if containsScopeControl(value) {
			return CanonicalWorkOutputScope{}, fmt.Errorf(
				"%w: semantic key contains a control character",
				ErrInvalidWorkOutputScope,
			)
		}
		return CanonicalWorkOutputScope{Kind: kind, Value: value}, nil
	default:
		return CanonicalWorkOutputScope{}, fmt.Errorf(
			"%w: unknown scope kind %q",
			ErrInvalidWorkOutputScope,
			kindText,
		)
	}
}

// NormalizeWorkOutputScope 返回带 canonical scope 与显式 mode 的副本。
func NormalizeWorkOutputScope(scope WorkOutputScope) (WorkOutputScope, error) {
	parsed, err := ParseWorkOutputScope(scope.Scope)
	if err != nil {
		return WorkOutputScope{}, err
	}
	if scope.Mode == "" {
		scope.Mode = WorkOutputScopeExclusive
	}
	if scope.Mode != WorkOutputScopeExclusive && scope.Mode != WorkOutputScopeShared {
		return WorkOutputScope{}, fmt.Errorf(
			"%w: mode must be exclusive or shared",
			ErrInvalidWorkOutputScope,
		)
	}
	scope.Scope = parsed.String()
	return scope, nil
}

// WorkOutputScopesConflict 判断两个声明是否存在不可并存的产出重叠。
//
// file 只与同一 NFC/case-fold 路径的 file 重叠；dir 与 file/dir 按同一
// comparison key 的 ancestor-or-equal containment 重叠；semantic 只按
// 大小写敏感 key 精确重叠。重叠仅在双方均为 shared 时允许，任一方为
// exclusive 即冲突。
func WorkOutputScopesConflict(left, right WorkOutputScope) (bool, error) {
	normalizedLeft, err := NormalizeWorkOutputScope(left)
	if err != nil {
		return false, err
	}
	normalizedRight, err := NormalizeWorkOutputScope(right)
	if err != nil {
		return false, err
	}
	leftScope, err := ParseWorkOutputScope(normalizedLeft.Scope)
	if err != nil {
		return false, err
	}
	rightScope, err := ParseWorkOutputScope(normalizedRight.Scope)
	if err != nil {
		return false, err
	}
	if !workOutputScopesOverlap(leftScope, rightScope) {
		return false, nil
	}
	return normalizedLeft.Mode == WorkOutputScopeExclusive ||
		normalizedRight.Mode == WorkOutputScopeExclusive, nil
}

// WorkOutputClaimsConflict 判断两个 Work Item 的 output claim 能否在同一 Plan
// 中安全共存。hardDependencies 使用 downstream -> direct upstream；只允许包含
// hard edge。重叠的 exclusive scope 通常冲突，但若一方经全 hard dependency
// 路径依赖另一方，则 Acceptance gate 已保证两个责任不会同时 Ready，可把该
// scope 视为按序移交。无依赖、兄弟分支与 soft-only 路径仍然冲突。
func WorkOutputClaimsConflict(
	leftOwnerID string,
	leftScope WorkOutputScope,
	rightOwnerID string,
	rightScope WorkOutputScope,
	hardDependencies map[string][]string,
) (bool, error) {
	conflict, err := WorkOutputScopesConflict(leftScope, rightScope)
	if err != nil || !conflict {
		return conflict, err
	}
	if leftOwnerID == "" || rightOwnerID == "" || leftOwnerID == rightOwnerID {
		return true, nil
	}
	if transitivelyHardDependsOn(leftOwnerID, rightOwnerID, hardDependencies) ||
		transitivelyHardDependsOn(rightOwnerID, leftOwnerID, hardDependencies) {
		return false, nil
	}
	return true, nil
}

func transitivelyHardDependsOn(
	downstream string,
	upstream string,
	hardDependencies map[string][]string,
) bool {
	seen := map[string]struct{}{downstream: {}}
	pending := append([]string(nil), hardDependencies[downstream]...)
	for len(pending) > 0 {
		index := len(pending) - 1
		candidate := pending[index]
		pending = pending[:index]
		if candidate == upstream {
			return true
		}
		if _, visited := seen[candidate]; visited {
			continue
		}
		seen[candidate] = struct{}{}
		pending = append(pending, hardDependencies[candidate]...)
	}
	return false
}

// WorkOutputScopeComparisonKey 返回冲突与同 Work Item 去重共用的比较键。
//
// file/dir 使用 NFC 后的 Unicode case-fold 路径，保守模拟跨平台大小写不敏感
// workspace；semantic 保持大小写敏感的 canonical exact key。该键只用于比较，
// 不替代 NormalizeWorkOutputScope 返回的用户可见/持久化 canonical scope。
func WorkOutputScopeComparisonKey(scope WorkOutputScope) (string, error) {
	normalized, err := NormalizeWorkOutputScope(scope)
	if err != nil {
		return "", err
	}
	parsed, err := ParseWorkOutputScope(normalized.Scope)
	if err != nil {
		return "", err
	}
	return string(parsed.Kind) + ":" + workOutputScopeComparisonValue(parsed), nil
}

func canonicalWorkspacePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%w: workspace-relative path must not be empty", ErrInvalidWorkOutputScope)
	}
	if containsScopeControl(value) {
		return "", fmt.Errorf("%w: path contains a control character", ErrInvalidWorkOutputScope)
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("%w: path must use POSIX '/' separators", ErrInvalidWorkOutputScope)
	}
	if path.IsAbs(value) || windowsDriveAbsolute(value) {
		return "", fmt.Errorf("%w: path must be workspace-relative", ErrInvalidWorkOutputScope)
	}
	canonical := path.Clean(value)
	if canonical == "." || canonical == ".." || strings.HasPrefix(canonical, "../") {
		return "", fmt.Errorf("%w: path escapes the workspace or resolves to '.'", ErrInvalidWorkOutputScope)
	}
	return canonical, nil
}

func containsScopeControl(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return r == 0 || r == '\r' || r == '\n'
	}) >= 0
}

func windowsDriveAbsolute(value string) bool {
	if len(value) < 3 || value[1] != ':' || value[2] != '/' {
		return false
	}
	first := value[0]
	return first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
}

func workOutputScopesOverlap(left, right CanonicalWorkOutputScope) bool {
	leftValue := workOutputScopeComparisonValue(left)
	rightValue := workOutputScopeComparisonValue(right)
	switch {
	case left.Kind == WorkOutputScopeKindSemantic || right.Kind == WorkOutputScopeKindSemantic:
		return left.Kind == WorkOutputScopeKindSemantic &&
			right.Kind == WorkOutputScopeKindSemantic &&
			leftValue == rightValue
	case left.Kind == WorkOutputScopeKindFile && right.Kind == WorkOutputScopeKindFile:
		return leftValue == rightValue
	case left.Kind == WorkOutputScopeKindDir && right.Kind == WorkOutputScopeKindDir:
		return pathContains(leftValue, rightValue) || pathContains(rightValue, leftValue)
	case left.Kind == WorkOutputScopeKindDir:
		return pathContains(leftValue, rightValue)
	case right.Kind == WorkOutputScopeKindDir:
		return pathContains(rightValue, leftValue)
	default:
		return false
	}
}

func workOutputScopeComparisonValue(scope CanonicalWorkOutputScope) string {
	if scope.Kind == WorkOutputScopeKindSemantic {
		return scope.Value
	}
	return cases.Fold().String(norm.NFC.String(scope.Value))
}

func pathContains(parent, child string) bool {
	return parent == child || strings.HasPrefix(child, parent+"/")
}
