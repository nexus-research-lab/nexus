package conversation

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/conversation/titlegen"
	"github.com/nexus-research-lab/nexus/internal/logx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// TitleScheduler 定义标题调度能力。
type TitleScheduler interface {
	Schedule(context.Context, titlegen.Request)
}

// RoundCoordinator 负责收口 DM / Room 共有的 round 骨架。
type RoundCoordinator struct {
	runtime    *runtimectx.Manager
	permission *permissionctx.Context
	logger     *slog.Logger
	titles     TitleScheduler
}

// NewRoundCoordinator 创建 round 协调器。
func NewRoundCoordinator(
	runtimeManager *runtimectx.Manager,
	permission *permissionctx.Context,
) *RoundCoordinator {
	return &RoundCoordinator{
		runtime:    runtimeManager,
		permission: permission,
		logger:     logx.NewDiscardLogger(),
	}
}

// SetLogger 设置日志实例。
func (c *RoundCoordinator) SetLogger(logger *slog.Logger) {
	if logger == nil {
		c.logger = logx.NewDiscardLogger()
		return
	}
	c.logger = logger
}

// SetTitleScheduler 设置标题调度器。
func (c *RoundCoordinator) SetTitleScheduler(scheduler TitleScheduler) {
	c.titles = scheduler
}

// StartRound 注册新的运行轮次。
func (c *RoundCoordinator) StartRound(
	sessionKey string,
	roundID string,
	cancel context.CancelFunc,
) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.StartRound(sessionKey, roundID, cancel)
}

// FinishRound 结束轮次并广播 session 状态。
func (c *RoundCoordinator) FinishRound(
	ctx context.Context,
	sessionKey string,
	roundID string,
) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.MarkRoundFinished(sessionKey, roundID)
	c.BroadcastSessionStatus(ctx, sessionKey)
}

// FailRound 结束异常轮次并广播 session 状态。
func (c *RoundCoordinator) FailRound(
	ctx context.Context,
	sessionKey string,
	roundID string,
) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.MarkRoundFinished(sessionKey, roundID)
	c.BroadcastSessionStatus(ctx, sessionKey)
}

// BroadcastSessionStatus 广播当前 session 的运行态快照。
func (c *RoundCoordinator) BroadcastSessionStatus(
	ctx context.Context,
	sessionKey string,
) {
	if c == nil || c.runtime == nil || c.permission == nil {
		return
	}
	if errs := c.permission.BroadcastSessionStatus(
		ctx,
		sessionKey,
		c.runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		c.loggerFor(ctx).Warn("广播 session 状态失败",
			"session_key", sessionKey,
			"error_count", len(errs),
		)
	}
}

// ScheduleTitle 统一调度会话标题生成。
func (c *RoundCoordinator) ScheduleTitle(
	ctx context.Context,
	request titlegen.Request,
) {
	if c == nil || c.titles == nil {
		return
	}
	c.titles.Schedule(ctx, request)
}

// AcquireRuntimeClient 获取或重建运行时 client。
func (c *RoundCoordinator) AcquireRuntimeClient(
	ctx context.Context,
	sessionKey string,
	options agentclient.Options,
	permissionMode sdkprotocol.PermissionMode,
) (runtimectx.Client, error) {
	if c == nil || c.runtime == nil {
		return nil, errors.New("round coordinator is not initialized")
	}

	client, err := c.runtime.GetOrCreate(ctx, sessionKey, options)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if permissionMode != "" {
		if err := client.SetPermissionMode(ctx, permissionMode); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func (c *RoundCoordinator) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, c.logger)
}

func isBrokenRuntimeClientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, agentclient.ErrNotConnected) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "file already closed") ||
		strings.Contains(message, "send control request failed") ||
		strings.Contains(message, "write payload failed") ||
		strings.Contains(message, "broken pipe")
}
