// INPUT: Control 持久失效序列、认证投影与宿主连接/runtime 关闭能力。
// OUTPUT: 顺序消费与游标提交、按原因失效、断联安全窗口和可重试的关闭。
// POS: 认证领域的身份失效协调；不依赖 HTTP 或应用装配。
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
)

const controlInvalidationApplyAttempts = 3

// ControlIdentityInvalidationSource 提供持久事件序列与本地身份投影。
type ControlIdentityInvalidationSource interface {
	ControlIdentityInvalidationCursor(context.Context) (int64, error)
	CommitControlIdentityInvalidationCursor(context.Context, int64) error
	ControlIdentityInvalidations(context.Context, int64) ([]ControlIdentityInvalidation, error)
	ApplyControlIdentityInvalidation(context.Context, ControlIdentityInvalidation) (string, error)
	FailClosedControlIdentities(context.Context) ([]string, error)
}

// ControlIdentityConnections 表示宿主可撤销的在线连接。
type ControlIdentityConnections interface {
	CloseControlConnections() int
	CloseControlSessionConnections(string) int
	CloseOwnerConnections(string) int
}

// ControlIdentityRuntimes 表示失效 owner 的运行时撤销能力。
type ControlIdentityRuntimes interface {
	CloseOwnerSessions(context.Context, string) (int, error)
}

// ControlIdentityInvalidationCoordinator 统一身份失效策略与持久游标推进。
type ControlIdentityInvalidationCoordinator struct {
	source       ControlIdentityInvalidationSource
	connections  ControlIdentityConnections
	runtimes     ControlIdentityRuntimes
	logger       *slog.Logger
	pollInterval time.Duration
	grace        time.Duration
}

// NewControlIdentityInvalidationCoordinator 绑定宿主能力，构造时不启动任务。
func NewControlIdentityInvalidationCoordinator(source ControlIdentityInvalidationSource, connections ControlIdentityConnections, runtimes ControlIdentityRuntimes, logger *slog.Logger) *ControlIdentityInvalidationCoordinator {
	if logger == nil {
		logger = logx.NewDiscardLogger()
	}
	return &ControlIdentityInvalidationCoordinator{source: source, connections: connections, runtimes: runtimes, logger: logger, pollInterval: time.Second, grace: time.Minute}
}

// Start 从已提交游标恢复；返回的停止函数等待消费退出。
func (c *ControlIdentityInvalidationCoordinator) Start(ctx context.Context) (func(), error) {
	if c.source == nil || c.connections == nil || c.runtimes == nil {
		return nil, errors.New("Control identity invalidation dependencies are unavailable")
	}
	cursor, err := c.source.ControlIdentityInvalidationCursor(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Control identity invalidation cursor: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); c.run(runCtx, c.source, cursor) }()
	c.logger.Info("启动 Control identity invalidation coordinator", "cursor", cursor)
	return func() { cancel(); <-done }, nil
}

func (c *ControlIdentityInvalidationCoordinator) run(
	ctx context.Context,
	source ControlIdentityInvalidationSource,
	cursor int64,
) {
	var unavailableSince time.Time
	failClosed := false
	for ctx.Err() == nil {
		events, err := source.ControlIdentityInvalidations(ctx, cursor)
		if err != nil {
			if unavailableSince.IsZero() {
				unavailableSince = time.Now().UTC()
			}
			c.logger.Warn("读取 Control identity invalidation 失败", "err", err)
			if !failClosed && time.Since(unavailableSince) >= c.grace {
				owners, closeErr := source.FailClosedControlIdentities(ctx)
				connections := c.connections.CloseControlConnections()
				for _, ownerUserID := range owners {
					_, runtimeErr := c.runtimes.CloseOwnerSessions(ctx, ownerUserID)
					closeErr = errors.Join(closeErr, runtimeErr)
				}
				c.logger.Error(
					"Control identity invalidation 超过安全窗口，已关闭认证会话",
					"owners", len(owners),
					"connections", connections,
					"err", closeErr,
				)
				failClosed = closeErr == nil
			}
			if !c.wait(ctx) {
				return
			}
			continue
		}
		unavailableSince = time.Time{}
		failClosed = false
		processedAll := true
		for _, event := range events {
			ownerUserID, connections, applyErr := c.applyControlIdentityInvalidationEvent(ctx, source, event)
			if applyErr != nil {
				attempts := 1
				for attempts < controlInvalidationApplyAttempts && ctx.Err() == nil {
					if !c.wait(ctx) {
						return
					}
					attempts++
					ownerUserID, connections, applyErr = c.applyControlIdentityInvalidationEvent(ctx, source, event)
					if applyErr == nil {
						break
					}
				}
				if ctx.Err() != nil {
					return
				}
				if applyErr != nil {
					owners, closedConnections, failClosedErr := c.failClosedControlIdentities(ctx, source)
					if failClosedErr != nil {
						c.logger.Error(
							"Control identity invalidation 持续失败，且 fail-closed 处理失败",
							"event_id", event.EventID,
							"attempts", attempts,
							"owners", owners,
							"connections", closedConnections,
							"err", errors.Join(applyErr, failClosedErr),
						)
						processedAll = false
						break
					}
					c.logger.Error(
						"Control identity invalidation 持续失败，已隔离并跳过事件",
						"event_id", event.EventID,
						"attempts", attempts,
						"owner_user_id", ownerUserID,
						"owners", owners,
						"connections", closedConnections,
						"err", applyErr,
					)
					applyErr = nil
				}
			}
			if applyErr = source.CommitControlIdentityInvalidationCursor(ctx, event.EventID); applyErr != nil {
				c.logger.Warn(
					"持久化 Control identity invalidation 游标失败",
					"event_id", event.EventID,
					"err", applyErr,
				)
				processedAll = false
				break
			}
			cursor = event.EventID
			c.logger.Info(
				"应用 Control identity invalidation",
				"event_id", event.EventID,
				"owner_user_id", ownerUserID,
				"connections", connections,
			)
		}
		if processedAll && len(events) == ControlIdentityInvalidationBatchSize {
			continue
		}
		if !c.wait(ctx) {
			return
		}
	}
}

func (c *ControlIdentityInvalidationCoordinator) applyControlIdentityInvalidationEvent(
	ctx context.Context,
	source ControlIdentityInvalidationSource,
	event ControlIdentityInvalidation,
) (string, int, error) {
	ownerUserID, applyErr := source.ApplyControlIdentityInvalidation(ctx, event)
	connections := 0
	if ownerUserID != "" {
		switch event.Reason {
		case "session_revoked":
			connections = c.connections.CloseControlSessionConnections(event.SessionID)
		case "entitlement_changed":
			// 本地额度投影对下一个请求生效，不中断当前 Agent。
		case "profile_changed":
			connections = c.connections.CloseOwnerConnections(ownerUserID)
		default:
			connections = c.connections.CloseOwnerConnections(ownerUserID)
			_, runtimeErr := c.runtimes.CloseOwnerSessions(ctx, ownerUserID)
			applyErr = errors.Join(applyErr, runtimeErr)
		}
	}
	return ownerUserID, connections, applyErr
}

func (c *ControlIdentityInvalidationCoordinator) failClosedControlIdentities(
	ctx context.Context,
	source ControlIdentityInvalidationSource,
) (int, int, error) {
	owners, closeErr := source.FailClosedControlIdentities(ctx)
	connections := c.connections.CloseControlConnections()
	for _, ownerUserID := range owners {
		_, runtimeErr := c.runtimes.CloseOwnerSessions(ctx, ownerUserID)
		closeErr = errors.Join(closeErr, runtimeErr)
	}
	return len(owners), connections, closeErr
}

func (c *ControlIdentityInvalidationCoordinator) wait(ctx context.Context) bool {
	timer := time.NewTimer(c.pollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
