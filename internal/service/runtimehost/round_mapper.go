package runtimehost

import (
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

// RoundMapper 把 DM 与 Room 共用的 message.EventMapper 适配为 exec.RoundMapper。
type RoundMapper struct {
	*message.EventMapper
}

// Map 映射单条 SDK 消息并保留终态 subtype。
func (m RoundMapper) Map(incoming sdkprotocol.ReceivedMessage, interruptReason ...string) (exec.RoundMapResult, error) {
	result, err := m.EventMapper.Map(incoming, interruptReason...)
	if err != nil {
		return exec.RoundMapResult{}, err
	}
	return exec.RoundMapResult(result), nil
}
