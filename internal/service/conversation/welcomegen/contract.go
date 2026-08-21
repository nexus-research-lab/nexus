// INPUT: 欢迎语生成依赖的 Agent、Provider、Preferences 与 Room 广播能力。
// OUTPUT: 消费侧窄接口。
// POS: welcomegen 依赖边界；不反向依赖 Room service 实现。
package welcomegen

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

type agentResolver interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
	GetAgentsByIDs(context.Context, []string) ([]protocol.Agent, error)
}

type roomResyncBroadcaster interface {
	BroadcastRoomResyncRequired(context.Context, string, string, string)
}
