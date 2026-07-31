// INPUT: Repository 的 CAS、幂等、图与派生状态失败。
// OUTPUT: service 可分类处理的稳定错误。
// POS: Orchestration SQL 仓储的错误语义边界。
package orchestration

import (
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	// ErrVersionConflict 表示 Execution 或子 aggregate 的 expected version 已过期。
	ErrVersionConflict = errors.New("execution orchestration version conflict")
	// ErrCommandConflict 表示同一 command ID 被用于不同 mutation。
	ErrCommandConflict = errors.New("execution orchestration command conflict")
	// ErrInvariant 表示 command 违反 WorkGraph 或同链约束。
	ErrInvariant = errors.New("execution orchestration invariant violation")
	// ErrWorkNotReady 表示 Work Item 尚未满足 Assignment 前置条件。
	ErrWorkNotReady = errors.New("execution work item is not ready")
	// ErrCompletionBlocked 表示 Execution 仍存在完成阻塞项。
	ErrCompletionBlocked = errors.New("execution completion is blocked")
	// ErrDispatchLease 表示 Dispatch 已由其他 consumer claim，或 ACK 使用了过期 lease/version。
	ErrDispatchLease = errors.New("execution dispatch lease conflict")
	// ErrProjectionLimitExceeded 表示命令会写入无法无损投影给模型的集合。
	ErrProjectionLimitExceeded = protocol.ErrExecutionProjectionLimitExceeded
)
