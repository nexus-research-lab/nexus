// INPUT: owner、channel 等持久化作用域标识。
// OUTPUT: 同一作用域内复用的进程级互斥锁释放函数。
// POS: Channels 所有配置和 pairing 写路径共享的进程内并发边界。
package channels

import "sync"

var (
	channelMutationLocks sync.Map
	pairingMutationLocks sync.Map
	controlMutationLocks sync.Map
)

func (s *ControlService) lockControlMutation(ownerUserID string) func() {
	key := normalizeChannelOwnerUserID(ownerUserID)
	value, _ := controlMutationLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (s *ControlService) lockChannelMutation(ownerUserID string, channelType string) func() {
	key := channelRouteKey(
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
	)
	value, _ := channelMutationLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (s *ControlService) lockPairingMutation(ownerUserID string) func() {
	key := normalizeChannelOwnerUserID(ownerUserID)
	value, _ := pairingMutationLocks.LoadOrStore(key, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}
