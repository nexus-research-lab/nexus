package room

import (
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

type roomRoundMapperAdapter struct {
	mapper *roomdomain.SlotMessageMapper
}

func (a roomRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (exec.RoundMapResult, error) {
	events, messages, terminalStatus, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return exec.RoundMapResult{}, err
	}
	return exec.RoundMapResult{
		Events:          events,
		DurableMessages: messages,
		TerminalStatus:  terminalStatus,
	}, nil
}

func (a roomRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}
