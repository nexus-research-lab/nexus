// INPUT: Launcher 配置、Agent/Room 服务与只读 Session 目录投影。
// OUTPUT: 侧栏与通知共用的首屏目录服务（Bootstrap）。
// POS: 目录 bootstrap 装配边界（路由沿用 /launcher/bootstrap）；首屏不得依赖 transcript/history 读取能力。
package launcher

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

// directorySessionReader 首屏只消费目录与持久摘要，不具备历史页读取能力。
type directorySessionReader interface {
	ListDirectorySessions(context.Context) ([]protocol.Session, error)
	ListRoomReplyPreviews(context.Context) (map[string]string, error)
}

// Service 提供首屏目录快照。
type Service struct {
	config       config.Config
	agentService *agentsvc.Service
	roomService  *roomsvc.Service
	session      directorySessionReader
}

// NewService 创建 Launcher 服务。
func NewService(
	cfg config.Config,
	agentService *agentsvc.Service,
	roomService *roomsvc.Service,
	sessionService *sessionsvc.Service,
) *Service {
	return &Service{
		config:       cfg,
		agentService: agentService,
		roomService:  roomService,
		session:      sessionService,
	}
}

func isoString(value interface {
	IsZero() bool
	Format(string) string
}) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}
