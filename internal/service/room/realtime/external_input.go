// INPUT: 宿主注入的外部输入回复与权限适配器。
// OUTPUT: 仅按持久 root 和成员身份关联的私域完成回复及权限处理。
// POS: Room 保持调度权，外部 transport 不参与公区事件或成员选择。
package realtime

import (
	"context"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SetExternalInputHooks 在服务启动前连接外部 transport，不改变 Room 调度。
func (s *Service) SetExternalInputHooks(
	reply func(context.Context, string, string, string, protocol.Message) error,
	permission func(context.Context, string, string, string) (sdkpermission.Handler, error),
) {
	s.externalReply = reply
	s.externalPermission = permission
}
