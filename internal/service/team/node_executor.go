// INPUT: 用户显式开启的 Node 授权、机器令牌与 Relay 任务提示。
// OUTPUT: 有界并发、持久领取和不重跑未知副作用的本机 Room 执行。
// POS: Node 消费器；不是新的 Agent runtime，不持有浏览器 Session 凭据。
package team

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

type NodeExecutor struct {
	nodes   *NodeService
	relay   *relaysvc.Client
	logger  *slog.Logger
	prepare func(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	start   func(context.Context, roomrealtime.ChatRequest, func(context.Context) error) error
	stop    func(context.Context, roomrealtime.InterruptRequest) error
	ready   atomic.Bool
	mu      sync.Mutex
	active  map[string]string
}

func NewNodeExecutor(nodes *NodeService, relay *relaysvc.Client, rooms *roomsvc.Service, runtime *roomrealtime.Service, logger *slog.Logger) *NodeExecutor {
	e := &NodeExecutor{nodes: nodes, relay: relay, logger: logger, prepare: rooms.EnsureRelayExecutionRoom, start: runtime.HandleAdmittedChat, stop: runtime.HandleInterrupt, active: map[string]string{}}
	nodes.executor = e
	return e
}

func (e *NodeExecutor) running(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, job := range e.active {
		if job == id {
			return true
		}
	}
	return false
}

func (e *NodeExecutor) Run(ctx context.Context) {
	if !e.ready.CompareAndSwap(false, true) {
		return
	}
	defer e.ready.Store(false)
	var workers sync.WaitGroup
	defer workers.Wait()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		e.poll(ctx, &workers)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *NodeExecutor) machineToken(ctx context.Context, grant teamstore.NodeGrant) (nodeToken, error) {
	if grant.RemoteURL != e.nodes.remoteURL || !grant.ExecutionEnabled || grant.State != "authorized" || e.nodes.keys == nil {
		return nodeToken{}, ErrNodeUnavailable
	}
	credential, err := e.nodes.keys.DecryptEnvelope(grant.CredentialEncrypted)
	if err != nil {
		return nodeToken{}, err
	}
	var result nodeToken
	err = e.nodes.controlRequest(ctx, "", string(credential), http.MethodPost, "/nodes/token", nil, &result)
	if err != nil {
		return nodeToken{}, err
	}
	if result.Token == "" || !result.ExpiresAt.After(time.Now().Add(10*time.Second)) {
		return nodeToken{}, ErrNodeUnavailable
	}
	return result, nil
}

func (e *NodeExecutor) poll(ctx context.Context, workers *sync.WaitGroup) {
	grants, err := e.nodes.store.NodeGrants(ctx)
	if err != nil {
		e.logger.Warn("读取本机节点授权失败")
		return
	}
	for _, grant := range grants {
		if !grant.ExecutionEnabled || grant.RemoteURL != e.nodes.remoteURL {
			continue
		}
		ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{UserID: grant.OwnerUserID})
		token, err := e.machineToken(ownerCtx, grant)
		if err != nil {
			continue
		}
		local, err := e.nodes.listAgents(ownerCtx)
		if err != nil {
			continue
		}
		pending, err := e.relay.PendingAgents(ownerCtx, token.Token)
		if err != nil {
			pending = nil
		}
		for _, agent := range token.Agents {
			if !slices.Contains(grant.AgentIDs, agent.AgentID) || !slices.ContainsFunc(local, func(a protocol.Agent) bool {
				return a.AgentID == agent.SourceAgentID && !a.IsMain && a.Status == "active"
			}) {
				continue
			}
			key := grant.NodeID + ":" + agent.AgentID
			e.mu.Lock()
			_, active := e.active[key]
			full := len(e.active) >= 8
			e.mu.Unlock()
			// ponytail: 单宿主最多八个并行 Agent，真实吞吐超过后再接队列调度。
			if active || full {
				continue
			}
			job, err := e.nodes.store.ActiveNodeJob(ownerCtx, grant.OwnerUserID, agent.SourceAgentID)
			if err != nil {
				continue
			}
			if job == nil && slices.Contains(pending, agent.AgentID) {
				job, err = e.nodes.store.PrepareNodeJob(ownerCtx, teamstore.NodeJob{ID: "job_" + rand.Text(), NodeID: grant.NodeID, OwnerUserID: grant.OwnerUserID, LocalAgentID: agent.SourceAgentID, AgentID: agent.AgentID, Scope: grant.Scope})
			}
			if err != nil || job == nil || job.NodeID != grant.NodeID || job.State == "running" || job.State == "review_required" {
				continue
			}
			e.mu.Lock()
			e.active[key] = job.ID
			e.mu.Unlock()
			workers.Add(1)
			go func(grant teamstore.NodeGrant, job teamstore.NodeJob, token nodeToken) {
				defer workers.Done()
				defer func() { e.mu.Lock(); delete(e.active, grant.NodeID+":"+job.AgentID); e.mu.Unlock() }()
				if err := e.consume(ownerCtx, grant, job, token); err != nil && ctx.Err() == nil {
					e.logger.Warn("在线 Agent 执行未完成", "job_id", job.ID, "node_id", grant.NodeID)
				}
			}(grant, *job, token)
		}
	}
}

func (e *NodeExecutor) activeGrant(ctx context.Context, original teamstore.NodeGrant) (teamstore.NodeGrant, error) {
	grant, err := e.nodes.store.NodeGrant(ctx, original.Scope, original.OwnerUserID)
	if err != nil {
		return teamstore.NodeGrant{}, err
	}
	if grant == nil || grant.NodeID != original.NodeID || grant.State != "authorized" || !grant.ExecutionEnabled || grant.RemoteURL != e.nodes.remoteURL {
		return teamstore.NodeGrant{}, ErrNodeLogin
	}
	return *grant, nil
}

func (e *NodeExecutor) consume(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token nodeToken) error {
	if _, err := e.activeGrant(ctx, grant); err != nil {
		return err
	}
	if job.State == "claiming" || job.State == "ready" {
		from := job.State
		item, err := e.relay.ClaimDelivery(ctx, token.Token, job.ID, job.AgentID)
		if err != nil {
			return err
		}
		job.State = "completed"
		if item != nil {
			if item.NodeID != job.NodeID || item.AgentID != job.AgentID || item.ID == "" || item.LeaseID == "" {
				return ErrNodeUnavailable
			}
			job.Delivery = item
			if item.State == "leased" {
				job.State = "ready"
			} else {
				job.State = "failed"
			}
		}
		if err = e.nodes.store.SaveNodeJob(ctx, job, from, nil); err != nil {
			return err
		}
	}
	if job.State == "draining" {
		return e.drain(ctx, grant, job, token.Token)
	}
	if job.State != "ready" {
		return nil
	}
	if job.Delivery == nil || len(job.Delivery.Messages) == 0 || job.Delivery.Messages[len(job.Delivery.Messages)-1].ID != job.Delivery.MessageID {
		return ErrNodeUnavailable
	}
	room, err := e.prepare(ctx, grant.Scope+":"+job.Delivery.RoomID, job.LocalAgentID)
	if err != nil {
		return err
	}
	job.RoomID, job.ConversationID, job.RoundID = room.Room.ID, room.Conversation.ID, "relay_"+job.ID
	// 先用原 claim 的租约重新鉴权，再 CAS 持久 running，最后才触碰 runtime。
	if _, err = e.activeGrant(ctx, grant); err != nil {
		return err
	}
	if _, err = e.relay.SettleDelivery(ctx, token.Token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
		return err
	}
	return e.execute(ctx, grant, job, token)
}

func (e *NodeExecutor) drain(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token string) error {
	for {
		if _, err := e.activeGrant(ctx, grant); err != nil {
			return err
		}
		output, err := e.nodes.store.NextNodeOutput(ctx, job.ID)
		if err != nil {
			return err
		}
		if output == nil {
			break
		}
		// 最终回执可能已提交，不能先 renew completed 租约而挡住原 output 重放。
		if job.State == "draining" && output.Input.Kind != "final" {
			if _, err = e.relay.SettleDelivery(ctx, token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
				return err
			}
		}
		if err = e.relay.DeliveryOutput(ctx, token, job.Delivery.ID, output.ID, output.Input); err != nil {
			if job.State == "draining" && nodeOutputRejected(err) {
				job.State = "failed"
				if saveErr := e.nodes.store.SaveNodeJob(ctx, job, "draining", nil); saveErr != nil {
					return saveErr
				}
			}
			return err
		}
		if err = e.nodes.store.AckNodeOutput(ctx, job.ID, output.Sequence); err != nil {
			return err
		}
		if job.State != "draining" {
			return nil
		}
	}
	if job.State == "draining" {
		job.State = "completed"
		return e.nodes.store.SaveNodeJob(ctx, job, "draining", nil)
	}
	return nil
}

func (e *NodeExecutor) stopJob(ctx context.Context, job teamstore.NodeJob) error {
	err := e.stop(ctx, roomrealtime.InterruptRequest{SessionKey: protocol.BuildRoomSharedSessionKey(job.ConversationID), RoundID: job.RoundID})
	if errors.Is(err, roomrealtime.ErrTargetRoomRoundNotRunning) {
		return nil
	}
	return err
}

func outputText(lease, kind, text string) *relaycontract.DeliveryOutput {
	return &relaycontract.DeliveryOutput{LeaseID: lease, Kind: kind, Content: relaycontract.MessageContent{Version: 1, Blocks: []relaycontract.ContentBlock{{Type: "markdown", Text: text}}}}
}
