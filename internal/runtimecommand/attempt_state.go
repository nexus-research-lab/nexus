// INPUT: physical round 内需要跨多次 CLI 进程保留的有界传输失败键。
// OUTPUT: 并发安全的递增与复位计数，不携带业务状态或授权。
// POS: 每次 broker 请求重建 operation registry 时仍保持 round-local 防循环语义。
package runtimecommand

import (
	"strings"
	"sync"
)

type AttemptState struct {
	mu     sync.Mutex
	counts map[string]uint32
}

func NewAttemptState() *AttemptState {
	return &AttemptState{counts: make(map[string]uint32)}
}

func (s *AttemptState) Increment(key string) uint32 {
	if s == nil {
		return 1
	}
	key = strings.TrimSpace(key)
	s.mu.Lock()
	s.counts[key]++
	value := s.counts[key]
	s.mu.Unlock()
	return value
}

func (s *AttemptState) Reset(key string) {
	if s == nil {
		return
	}
	key = strings.TrimSpace(key)
	s.mu.Lock()
	delete(s.counts, key)
	s.mu.Unlock()
}
