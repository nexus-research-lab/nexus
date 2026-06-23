package room

import (
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type roomRoundMapperAdapter struct {
	mapper *roomdomain.SlotMessageMapper
}

func (a roomRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (runtimectx.RoundMapResult, error) {
	events, messages, terminalStatus, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return runtimectx.RoundMapResult{}, err
	}
	return runtimectx.RoundMapResult{
		Events:          events,
		DurableMessages: messages,
		TerminalStatus:  terminalStatus,
	}, nil
}

func (a roomRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}
