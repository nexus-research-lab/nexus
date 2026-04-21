package agent

// BuildRuntimePrompt 构建运行时附加提示词。
func (s *Service) BuildRuntimePrompt(agentValue *Agent) (string, error) {
	if s == nil || s.prompts == nil {
		return "", nil
	}
	return s.prompts.Build(agentValue)
}
