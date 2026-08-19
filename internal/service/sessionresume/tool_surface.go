// INPUT: 旧、新模型工具面脱敏指纹。
// OUTPUT: 是否必须从旧 transcript fork 新物理 SDK session。
// POS: Nexus 产品层对所有 runtime 工具面换代的单一判定入口。
package sessionresume

import (
	"strings"
)

// RequiresToolSurfaceFork 判断旧 SDK session 是否不能安全复用。
//
// 旧会话没有基线或工具面已变化时，宿主不能把“runtime 已重配置”等同于
// “模型已采用新 schema”；必须让分支 Session 从首轮采用当前工具面。
func RequiresToolSurfaceFork(
	storedFingerprint string,
	currentFingerprint string,
	forkLegacy bool,
) bool {
	currentFingerprint = strings.TrimSpace(currentFingerprint)
	if currentFingerprint == "" {
		return false
	}
	storedFingerprint = strings.TrimSpace(storedFingerprint)
	if storedFingerprint == "" {
		return forkLegacy
	}
	return storedFingerprint != currentFingerprint
}
