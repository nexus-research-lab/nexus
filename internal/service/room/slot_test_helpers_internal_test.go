package room

// withRoomSlotStatus 让测试只声明稳定身份，再显式设置 runtime 状态。
func withRoomSlotStatus(slot *activeRoomSlot, status string) *activeRoomSlot {
	if slot != nil {
		slot.setStatus(status)
	}
	return slot
}
