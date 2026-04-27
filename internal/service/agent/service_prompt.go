package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// BuildRuntimePrompt 构建运行时附加提示词。
func (s *Service) BuildRuntimePrompt(ctx context.Context, agentValue *protocol.Agent) (string, error) {
	if s == nil || s.prompts == nil {
		return "", nil
	}
	return s.prompts.Build(ctx, agentValue)
}
