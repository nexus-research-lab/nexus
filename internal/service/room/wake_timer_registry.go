// INPUT: delayed wake 和短窗口 dispatch 的唯一键计时任务。
// OUTPUT: 去重调度、回调前释放与统一停机。
// POS: Room 唤醒计时器的封装边界，避免 RealtimeService 持有多组锁和 map。
package room

import (
	"sync"
	"time"
)

type roomWakeTimerRegistry struct {
	mu       sync.Mutex
	delayed  map[string]*time.Timer
	dispatch map[string]*time.Timer
	stopped  bool
}

func newRoomWakeTimerRegistry() *roomWakeTimerRegistry {
	return &roomWakeTimerRegistry{
		delayed:  make(map[string]*time.Timer),
		dispatch: make(map[string]*time.Timer),
	}
}

func (r *roomWakeTimerRegistry) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = false
}

func (r *roomWakeTimerRegistry) ScheduleDelayed(key string, delay time.Duration, callback func()) {
	r.schedule(r.delayed, key, delay, callback)
}

func (r *roomWakeTimerRegistry) ScheduleDispatch(key string, delay time.Duration, callback func()) {
	r.schedule(r.dispatch, key, delay, callback)
}

func (r *roomWakeTimerRegistry) schedule(
	timers map[string]*time.Timer,
	key string,
	delay time.Duration,
	callback func(),
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || key == "" {
		return
	}
	if _, exists := timers[key]; exists {
		return
	}
	timers[key] = time.AfterFunc(delay, func() {
		r.mu.Lock()
		delete(timers, key)
		r.mu.Unlock()
		if callback != nil {
			callback()
		}
	})
}

func (r *roomWakeTimerRegistry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	stopRoomWakeTimers(r.delayed)
	stopRoomWakeTimers(r.dispatch)
}

func stopRoomWakeTimers(timers map[string]*time.Timer) {
	for key, timer := range timers {
		timer.Stop()
		delete(timers, key)
	}
}
