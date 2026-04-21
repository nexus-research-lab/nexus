// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service_prompt.go
// @Date   ：2026/04/20 16:35:00
// @Author ：leemysw
// 2026/04/20 16:35:00   Create
// =====================================================

package agent

// BuildRuntimePrompt 构建运行时附加提示词。
func (s *Service) BuildRuntimePrompt(agentValue *Agent) (string, error) {
	if s == nil || s.prompts == nil {
		return "", nil
	}
	return s.prompts.Build(agentValue)
}
