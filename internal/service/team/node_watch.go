// INPUT: 当前机器授权、Relay 节点提示和进程生命周期。
// OUTPUT: 无固定任务轮询的 WS 唤醒与断线恢复。
// POS: 复用 Relay transport 与 duework；不复制持久领取或 Room 执行逻辑。
package team

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

const (
	inactiveWatchRecheck  = 30 * time.Second
	stableWatchConnection = time.Minute
)

type nodeWatch struct {
	credential string
	ctx        context.Context
	cancel     context.CancelFunc
}

func (e *NodeExecutor) syncWatches(ctx context.Context, grants []teamstore.NodeGrant, watchers map[string]nodeWatch, workers *sync.WaitGroup) {
	wanted := make(map[string]teamstore.NodeGrant)
	for _, grant := range grants {
		if grant.State == "authorized" && grant.ExecutionEnabled && grant.RemoteURL == e.nodes.remoteURL {
			wanted[grant.NodeID] = grant
		}
	}
	for id, watcher := range watchers {
		if grant, ok := wanted[id]; !ok || grant.CredentialEncrypted != watcher.credential {
			watcher.cancel()
			delete(watchers, id)
		}
	}
	for id, grant := range wanted {
		if _, ok := watchers[id]; ok {
			continue
		}
		watchCtx, cancel := context.WithCancel(ctx)
		watchers[id] = nodeWatch{credential: grant.CredentialEncrypted, ctx: watchCtx, cancel: cancel}
		workers.Add(1)
		go func() {
			defer workers.Done()
			e.watchNode(watchCtx, grant)
		}()
	}
}

func (e *NodeExecutor) watchNode(ctx context.Context, grant teamstore.NodeGrant) {
	retry := duework.New(duework.Options{OnError: func(err error) {
		e.logFailure(ctx, "watch_deliveries", grant, teamstore.NodeJob{}, err)
	}})
	_ = retry.Run(ctx, func(ctx context.Context, _ time.Time) (duework.Result, error) {
		current, err := e.activeGrant(ctx, grant)
		if err != nil {
			if errors.Is(err, ErrNodeInactive) {
				return e.inactiveWatch(), nil
			}
			return duework.Result{}, err
		}
		token, err := e.machineToken(ctx, current)
		if err != nil {
			return duework.Result{}, err
		}
		// Relay 提前请求换票，保持原 WS 和任务不变；每次仍重验本机授权。
		connectedAt := time.Now()
		err = e.relay.WatchDeliveries(ctx, token.Token, func(ctx context.Context) (string, error) {
			current, err := e.activeGrant(ctx, grant)
			if err != nil {
				return "", err
			}
			fresh, err := e.machineToken(ctx, current)
			return fresh.Token, err
		}, func() error {
			e.loop.Notify()
			return nil
		})
		if errors.Is(err, ErrNodeInactive) {
			return e.inactiveWatch(), nil
		}
		// 稳定运行过的长连接断开是新故障，从最小退避重连，不继承历史失败的长等待。
		return duework.Result{ResetBackoff: time.Since(connectedAt) >= stableWatchConnection}, err
	})
}

// inactiveWatch 让主循环按最新授权移除或替换订阅；授权同凭据恢复时，本 watcher 自行定期复查，不永久空闲。
func (e *NodeExecutor) inactiveWatch() duework.Result {
	e.loop.Notify()
	next := time.Now().Add(inactiveWatchRecheck)
	return duework.Result{NextDueAt: &next}
}
