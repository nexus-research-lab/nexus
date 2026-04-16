// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：mention.go
// @Date   ：2026/04/11 03:28:00
// @Author ：leemysw
// 2026/04/11 03:28:00   Create
// =====================================================

package room

import (
	"regexp"
	"sort"
	"strings"
)

// ResolveMentionAgentIDs 解析消息中的 @mention，并返回对应 agent_id。
func ResolveMentionAgentIDs(content string, agentNameToID map[string]string) []string {
	if strings.TrimSpace(content) == "" || len(agentNameToID) == 0 {
		return nil
	}

	names := make([]string, 0, len(agentNameToID))
	for name := range agentNameToID {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i int, j int) bool {
		return len([]rune(names[i])) > len([]rune(names[j]))
	})

	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		pattern, err := regexp.Compile(`@` + regexp.QuoteMeta(name) + `([\s，。！？、,.!?;\-:：；]|$)`)
		if err != nil {
			continue
		}
		if !pattern.MatchString(content) {
			continue
		}
		agentID := strings.TrimSpace(agentNameToID[name])
		if agentID == "" {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		result = append(result, agentID)
	}
	return result
}
