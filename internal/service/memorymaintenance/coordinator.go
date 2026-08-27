package memorymaintenance

// 本文件负责 AutoDream 宿主时钟、Agent 扫描和运行占用。

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	autoDreamProviderUnavailableReason        = "provider_unavailable"
	autoDreamProviderUnavailableRetryInterval = time.Hour
)

type agentCatalog interface {
	ListAllAgentRecordsForMaintenance(context.Context) ([]protocol.Agent, error)
	EnsureRuntimeSettingsProjection(protocol.Agent) error
	LoadRuntimeSettingsProjection(protocol.Agent) (map[string]any, error)
}

type dreamRunner interface {
	tryAutoDream(context.Context, protocol.Agent) (agentclient.AutoDreamResult, error)
}

// autoDreamEnabled 只做宿主侧廉价预检，nxs 仍会再次执行完整 gate 判断。
func autoDreamEnabled(catalog agentCatalog, agentValue protocol.Agent) (bool, error) {
	if err := catalog.EnsureRuntimeSettingsProjection(agentValue); err != nil {
		return false, err
	}
	settings, err := catalog.LoadRuntimeSettingsProjection(agentValue)
	if err != nil {
		return false, err
	}
	if memorySettings, ok := settings["memory"].(map[string]any); ok {
		if dreamSettings, ok := memorySettings["dream"].(map[string]any); ok {
			if enabled, ok := dreamSettings["enabled"].(bool); ok {
				return enabled, nil
			}
		}
		if enabled, ok := memorySettings["dreamEnabled"].(bool); ok {
			return enabled, nil
		}
	}
	enabled, _ := settings["autoDreamEnabled"].(bool)
	return enabled, nil
}

// Coordinator 是 Nexus 托管模式下唯一的 AutoDream 唤醒者。
type Coordinator struct {
	agents agentCatalog
	config config.MemoryMaintenanceConfig
	logger *slog.Logger
	runner dreamRunner
	now    func() time.Time

	mu         sync.Mutex
	cancel     context.CancelFunc
	running    map[string]struct{}
	nextChecks map[string]time.Time
	started    bool
	semaphore  chan struct{}
	wg         sync.WaitGroup
}

func newCoordinator(cfg config.MemoryMaintenanceConfig, agents agentCatalog, runner dreamRunner) *Coordinator {
	concurrency := cfg.MaxConcurrent
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Coordinator{
		agents:     agents,
		config:     cfg,
		logger:     logx.NewDiscardLogger(),
		runner:     runner,
		now:        time.Now,
		running:    map[string]struct{}{},
		nextChecks: map[string]time.Time{},
		semaphore:  make(chan struct{}, concurrency),
	}
}

// SetLogger 注入业务日志实例。
func (c *Coordinator) SetLogger(logger *slog.Logger) {
	if logger == nil {
		c.logger = logx.NewDiscardLogger()
		return
	}
	c.logger = logger
}

// Start 启动后台扫描；首次检查不等待完整 interval。
func (c *Coordinator) Start(ctx context.Context) error {
	if c == nil || c.agents == nil || c.runner == nil || c.config.SweepInterval <= 0 {
		return nil
	}
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	loopContext, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.started = true
	c.mu.Unlock()

	c.wg.Add(1)
	go c.runLoop(loopContext)
	return nil
}

// Stop 停止扫描并等待已经唤醒的 nxs maintenance 退出。
func (c *Coordinator) Stop() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.started = false
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
}

// runOnce 扫描一次当前到期 Agent；具体 Dream 在受控后台 goroutine 中完成。
func (c *Coordinator) runOnce(ctx context.Context) error {
	if c == nil || c.agents == nil || c.runner == nil {
		return nil
	}
	agents, err := c.agents.ListAllAgentRecordsForMaintenance(ctx)
	if err != nil {
		return err
	}
	now := c.nowTime()
	for _, item := range agents {
		agentValue := item
		enabled, settingsErr := autoDreamEnabled(c.agents, agentValue)
		if settingsErr != nil {
			c.logger.Warn("读取 Agent AutoDream 设置失败", "agent_id", agentValue.AgentID, "err", settingsErr)
			continue
		}
		if !enabled || !c.claim(agentValue, now) {
			continue
		}
		c.wg.Add(1)
		go c.runAgent(ctx, agentValue)
	}
	return nil
}

func (c *Coordinator) runLoop(ctx context.Context) {
	defer c.wg.Done()
	if err := c.runOnce(ctx); err != nil {
		c.logger.Error("首次 AutoDream 扫描失败", "err", err)
	}
	ticker := time.NewTicker(c.config.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.runOnce(ctx); err != nil {
				c.logger.Error("AutoDream 扫描失败", "err", err)
			}
		}
	}
}

func (c *Coordinator) runAgent(parent context.Context, agentValue protocol.Agent) {
	defer c.wg.Done()
	key := agentKey(agentValue)
	defer c.release(key)

	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-parent.Done():
		return
	}

	ctx := parent
	cancel := func() {}
	if c.config.RunTimeout > 0 {
		ctx, cancel = context.WithTimeout(parent, c.config.RunTimeout)
	}
	defer cancel()

	result, err := c.runner.tryAutoDream(ctx, agentValue)
	if err != nil {
		c.scheduleNext(key, c.nowTime().Add(c.config.SweepInterval))
		c.logger.Error("唤醒 Agent AutoDream 失败", "agent_id", agentValue.AgentID, "err", err)
		return
	}
	now := c.nowTime()
	nextCheck := time.UnixMilli(result.NextCheckAtMS)
	providerUnavailable := result.Status == agentclient.AutoDreamStatusSkipped &&
		strings.TrimSpace(result.Reason) == autoDreamProviderUnavailableReason
	if providerUnavailable {
		nextCheck = now.Add(max(c.config.SweepInterval, autoDreamProviderUnavailableRetryInterval))
	} else if result.NextCheckAtMS <= 0 || !nextCheck.After(now) {
		nextCheck = now.Add(c.config.SweepInterval)
	}
	c.scheduleNext(key, nextCheck)
	level := slog.LevelInfo
	if providerUnavailable {
		level = slog.LevelDebug
	}
	c.logger.Log(ctx, level, "Agent AutoDream 检查完成",
		"agent_id", agentValue.AgentID,
		"status", result.Status,
		"reason", result.Reason,
		"sessions_reviewed", result.SessionsReviewed,
		"written_paths", result.WrittenPaths,
		"next_check_at", nextCheck.UTC(),
	)
}

func (c *Coordinator) claim(agentValue protocol.Agent, now time.Time) bool {
	key := agentKey(agentValue)
	if key == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.running[key]; exists {
		return false
	}
	if next := c.nextChecks[key]; !next.IsZero() && next.After(now) {
		return false
	}
	c.running[key] = struct{}{}
	return true
}

func (c *Coordinator) release(key string) {
	c.mu.Lock()
	delete(c.running, key)
	c.mu.Unlock()
}

func (c *Coordinator) scheduleNext(key string, next time.Time) {
	c.mu.Lock()
	c.nextChecks[key] = next.UTC()
	c.mu.Unlock()
}

func (c *Coordinator) nowTime() time.Time {
	if c.now == nil {
		return time.Now().UTC()
	}
	return c.now().UTC()
}

func agentKey(agentValue protocol.Agent) string {
	agentID := strings.TrimSpace(agentValue.AgentID)
	if agentID == "" {
		return ""
	}
	return strings.TrimSpace(agentValue.OwnerUserID) + ":" + agentID
}
