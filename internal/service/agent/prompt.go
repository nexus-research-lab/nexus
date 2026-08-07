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

// BuildRuntimeUserMessageSuffixForContext 按用户偏好构建指定情绪上下文的动态上下文。
func (s *Service) BuildRuntimeUserMessageSuffixForContext(
	ctx context.Context,
	agentValue *protocol.Agent,
	emotionContextID string,
	emotionEnabled bool,
) string {
	if s == nil || s.prompts == nil || !emotionEnabled {
		return ""
	}
	return s.prompts.BuildUserMessageSuffix(ctx, agentValue, emotionContextID)
}
