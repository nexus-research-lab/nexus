// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package realtime

func newRoomRoundRegistryFromRounds(rounds map[string]*activeRoomRound) roomRoundRegistry {
	registry := &roomRoundRegistry{
		conversations: make(map[string]*roomConversationState),
	}
	for _, roundValue := range rounds {
		registry.register(roundValue)
	}
	return roomRoundRegistry{conversations: registry.conversations}
}
