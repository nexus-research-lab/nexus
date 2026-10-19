// INPUT: darwin-v1 持久身份串与待核验身份；不读取文件系统或任务可写状态。
// OUTPUT: 严格解析后的稳定字段判等：inode、generation、birth time 一致即同一目录。
// POS: device 字段仅随身份持久化用于诊断审计，不参与相等；解析失败一律视为不相等。
package runtime

import (
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// scratchIdentityPrefix 与 sandbox_scratch_identity_darwin.go 的生成格式同源。
// 版本前缀不一致或字段残缺都保持 fail-closed，不允许未知格式借稳定字段放行。
const scratchIdentityPrefix = "darwin-v1"

// scratchIdentityStableFields 严格解析 darwin-v1:dev:ino:gen:birthSec:birthNSec，
// 返回除 device 外的四个稳定字段。macOS 系统升级会重编卷设备号，同一目录的
// device 可能漂移；inode、generation 与 birth time 仍唯一钉住目录对象。
func scratchIdentityStableFields(identity string) ([4]uint64, bool) {
	var stable [4]uint64
	rest, ok := strings.CutPrefix(identity, scratchIdentityPrefix+":")
	if !ok {
		return stable, false
	}
	fields := strings.Split(rest, ":")
	if len(fields) != 5 {
		return stable, false
	}
	for i, field := range fields {
		value, err := strconv.ParseUint(field, 16, 64)
		if err != nil {
			return stable, false
		}
		if i > 0 {
			stable[i-1] = value
		}
	}
	return stable, true
}

// sandboxIdentityStableEqual 核对记录身份与当前身份。两侧空串维持零值语义
// （无证明不升级为有证明）；任一侧格式错误都判不相等，绝不因解析失败
// 把替换目录当作原目录放行。
func sandboxIdentityStableEqual(recorded, current string) bool {
	if recorded == "" || current == "" {
		return recorded == current
	}
	left, ok := scratchIdentityStableFields(recorded)
	if !ok {
		return false
	}
	right, ok := scratchIdentityStableFields(current)
	if !ok {
		return false
	}
	return left == right
}

// sandboxScratchBindingEqual 核验持久绑定：固定父路径与 leaf 名仍严格相等，
// 两个身份串按稳定字段比较，仅容忍 device 漂移。
func sandboxScratchBindingEqual(recorded, current protocol.SandboxProcessScratch) bool {
	return recorded.BasePath == current.BasePath &&
		recorded.LeafName == current.LeafName &&
		sandboxIdentityStableEqual(recorded.BaseIdentity, current.BaseIdentity) &&
		sandboxIdentityStableEqual(recorded.LeafIdentity, current.LeafIdentity)
}
