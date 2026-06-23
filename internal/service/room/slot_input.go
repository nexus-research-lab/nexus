package room

import (
	"slices"
	"strings"
)

func (slot *activeRoomSlot) enqueueQueuedInput(roundID string, content string) {
	if slot == nil || strings.TrimSpace(content) == "" {
		return
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	slot.QueuedInputs = append(slot.QueuedInputs, roomQueuedInput{
		RoundID: strings.TrimSpace(roundID),
		Content: strings.TrimSpace(content),
	})
}

func (slot *activeRoomSlot) drainQueuedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	if len(slot.QueuedInputs) == 0 {
		return nil
	}
	inputs := slices.Clone(slot.QueuedInputs)
	slot.QueuedInputs = nil
	return inputs
}

func (slot *activeRoomSlot) enqueueGuidedInput(roundID string, content string) {
	if slot == nil || strings.TrimSpace(content) == "" {
		return
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	slot.GuidedInputs = append(slot.GuidedInputs, roomQueuedInput{
		RoundID: strings.TrimSpace(roundID),
		Content: strings.TrimSpace(content),
	})
}

func (slot *activeRoomSlot) drainGuidedInputs() []roomQueuedInput {
	if slot == nil {
		return nil
	}
	slot.inputMu.Lock()
	defer slot.inputMu.Unlock()
	if len(slot.GuidedInputs) == 0 {
		return nil
	}
	inputs := slices.Clone(slot.GuidedInputs)
	slot.GuidedInputs = nil
	return inputs
}
