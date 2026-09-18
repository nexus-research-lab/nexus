// INPUT: 本人入群自动登记的 Node 授权、机器令牌与 Relay 任务提示。
// OUTPUT: 有界并发、持久领取和不重跑未知副作用的本机 Room 执行。
// POS: Node 消费器；不是新的 Agent runtime，不持有浏览器 Session 凭据。
package team

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	relaysvc "github.com/nexus-research-lab/nexus/internal/service/relay"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

type NodeExecutor struct {
	nodes    *NodeService
	relay    *relaysvc.Client
	logger   *slog.Logger
	prepare  func(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	start    func(context.Context, roomrealtime.ChatRequest, func(context.Context) error) error
	upload   func(context.Context, string, string, string, string, io.Reader) (*workspacesvc.UploadResult, error)
	stop     func(context.Context, roomrealtime.InterruptRequest) error
	ready    atomic.Bool
	mu       sync.Mutex
	active   map[string]string
	logTimes map[string]time.Time
	tokens   map[string]cachedNodeToken
	loop     *duework.Loop
	workErr  error
}

type cachedNodeToken struct {
	credential string
	value      nodeToken
}

func NewNodeExecutor(nodes *NodeService, relay *relaysvc.Client, rooms *roomsvc.Service, runtime *roomrealtime.Service, logger *slog.Logger) *NodeExecutor {
	e := &NodeExecutor{nodes: nodes, relay: relay, logger: logger, prepare: rooms.EnsureRelayExecutionRoom, start: runtime.HandleAdmittedChat, stop: runtime.HandleInterrupt, active: map[string]string{}}
	e.loop = duework.New(duework.Options{})
	nodes.executor = e
	e.upload = rooms.UploadConversationAttachment
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
	e.logger.Info("在线 Agent 执行器启动")
	defer e.logger.Info("在线 Agent 执行器停止")
	var workers sync.WaitGroup
	defer workers.Wait()
	watchers := make(map[string]nodeWatch)
	defer func() {
		for _, watcher := range watchers {
			watcher.cancel()
		}
	}()
	_ = e.loop.Run(ctx, func(ctx context.Context, _ time.Time) (duework.Result, error) {
		e.mu.Lock()
		err := e.workErr
		e.workErr = nil
		e.mu.Unlock()
		if err != nil {
			return duework.Result{}, err
		}
		// 与自动登记同序，防止领取旧范围任务后又被新的设备登记撤销。
		e.nodes.provisionMu.Lock()
		defer e.nodes.provisionMu.Unlock()
		grants, err := e.nodes.store.NodeGrants(ctx)
		if err != nil {
			e.logFailure(ctx, "list_grants", teamstore.NodeGrant{}, teamstore.NodeJob{}, err)
			return duework.Result{}, err
		}
		e.syncWatches(ctx, grants, watchers, &workers)
		return e.reconcileJobs(ctx, grants, watchers, &workers)
	})
}

func (e *NodeExecutor) cachedToken(ctx context.Context, grant teamstore.NodeGrant) (nodeToken, error) {
	if grant.RemoteURL != e.nodes.remoteURL || !grant.ExecutionEnabled || grant.State != "authorized" || e.nodes.keys == nil {
		return nodeToken{}, ErrNodeUnavailable
	}
	// 令牌有效期内复用；本地授权每轮仍重读，Relay 仍检查 Control 的撤权栅栏。
	e.mu.Lock()
	cached := e.tokens[grant.NodeID]
	e.mu.Unlock()
	if cached.credential == grant.CredentialEncrypted && cached.value.ExpiresAt.After(time.Now().Add(15*time.Second)) {
		return cached.value, nil
	}
	return e.machineToken(ctx, grant)
}

func (e *NodeExecutor) machineToken(ctx context.Context, grant teamstore.NodeGrant) (nodeToken, error) {
	if grant.RemoteURL != e.nodes.remoteURL || !grant.ExecutionEnabled || grant.State != "authorized" || e.nodes.keys == nil {
		return nodeToken{}, ErrNodeUnavailable
	}
	credential, err := e.nodes.keys.DecryptEnvelope(grant.CredentialEncrypted)
	if err != nil {
		e.logFailure(ctx, "decrypt_node_credential", grant, teamstore.NodeJob{}, err)
		return nodeToken{}, err
	}
	var result nodeToken
	err = e.nodes.controlRequest(ctx, "", string(credential), http.MethodPost, "/nodes/token", nil, &result)
	if err != nil {
		return nodeToken{}, err
	}
	if result.Token == "" || !result.ExpiresAt.After(time.Now().Add(10*time.Second)) {
		e.logFailure(ctx, "validate_machine_token", grant, teamstore.NodeJob{}, ErrNodeUnavailable)
		return nodeToken{}, ErrNodeUnavailable
	}
	e.mu.Lock()
	if e.tokens == nil {
		e.tokens = make(map[string]cachedNodeToken)
	}
	for id, item := range e.tokens {
		if !item.value.ExpiresAt.After(time.Now()) {
			delete(e.tokens, id)
		}
	}
	e.tokens[grant.NodeID] = cachedNodeToken{credential: grant.CredentialEncrypted, value: result}
	e.mu.Unlock()
	return result, nil
}

func (e *NodeExecutor) reconcileJobs(ctx context.Context, grants []teamstore.NodeGrant, watchers map[string]nodeWatch, workers *sync.WaitGroup) (result duework.Result, resultErr error) {
	for _, grant := range grants {
		if grant.State != "authorized" || !grant.ExecutionEnabled || grant.RemoteURL != e.nodes.remoteURL {
			e.logger.Debug("在线 Agent 跳过节点", "node_id", grant.NodeID, "execution_enabled", grant.ExecutionEnabled, "remote_matches", grant.RemoteURL == e.nodes.remoteURL)
			continue
		}
		// 本机撤销同时取消订阅和执行，不等下一次租约维护才停止原生 round。
		ownerCtx := authctx.WithPrincipal(watchers[grant.NodeID].ctx, &authctx.Principal{UserID: grant.OwnerUserID})
		token, err := e.cachedToken(ownerCtx, grant)
		if err != nil {
			e.logFailure(ctx, "machine_token", grant, teamstore.NodeJob{}, err)
			resultErr = err
			continue
		}
		local, err := e.nodes.listAgents(ownerCtx)
		if err != nil {
			e.logFailure(ctx, "list_local_agents", grant, teamstore.NodeJob{}, err)
			resultErr = err
			continue
		}
		work, err := e.relay.PendingAgents(ownerCtx, token.Token)
		if err != nil {
			e.logFailure(ctx, "pending_agents", grant, teamstore.NodeJob{}, err)
			resultErr = err
		}
		pending := work.AgentIDs
		if work.NextDueAt != nil && (result.NextDueAt == nil || work.NextDueAt.Before(*result.NextDueAt)) {
			result.NextDueAt = work.NextDueAt
		}
		if len(pending) > 0 {
			e.logger.Debug("在线 Agent 发现待执行任务", "node_id", grant.NodeID, "token_agents", len(token.Agents), "local_agents", len(local), "pending_agents", len(pending))
		}
		for _, agent := range token.Agents {
			if !slices.Contains(grant.AgentIDs, agent.AgentID) || !slices.ContainsFunc(local, func(a protocol.Agent) bool {
				return a.AgentID == agent.SourceAgentID && !a.IsMain && a.Status == "active"
			}) {
				e.logger.Debug("在线 Agent 不满足执行条件", "node_id", grant.NodeID, "agent_id", agent.AgentID, "local_agent_id", agent.SourceAgentID)
				continue
			}
			key := grant.NodeID + ":" + agent.AgentID
			e.mu.Lock()
			_, active := e.active[key]
			full := len(e.active) >= 8
			e.mu.Unlock()
			// ponytail: 单宿主最多八个并行 Agent，真实吞吐超过后再接队列调度。
			if active || full {
				if slices.Contains(pending, agent.AgentID) {
					e.logger.Debug("在线 Agent 等待执行槽位", "node_id", grant.NodeID, "agent_id", agent.AgentID, "active", active, "capacity_full", full)
				}
				continue
			}
			job, err := e.nodes.store.ActiveNodeJob(ownerCtx, grant.OwnerUserID, agent.SourceAgentID)
			if err != nil {
				e.logFailure(ctx, "read_active_job", grant, teamstore.NodeJob{AgentID: agent.AgentID, LocalAgentID: agent.SourceAgentID}, err)
				resultErr = err
				continue
			}
			if job == nil && slices.Contains(pending, agent.AgentID) {
				job, err = e.nodes.store.PrepareNodeJob(ownerCtx, teamstore.NodeJob{ID: "job_" + rand.Text(), NodeID: grant.NodeID, OwnerUserID: grant.OwnerUserID, LocalAgentID: agent.SourceAgentID, AgentID: agent.AgentID, Scope: grant.Scope})
			}
			if err != nil {
				e.logFailure(ctx, "prepare_job", grant, teamstore.NodeJob{AgentID: agent.AgentID, LocalAgentID: agent.SourceAgentID}, err)
				resultErr = err
				continue
			}
			if job == nil {
				continue
			}
			if job.NodeID != grant.NodeID || job.State == "running" || job.State == "review_required" {
				e.logFailure(ctx, "job_requires_review", grant, *job, ErrNodeUnavailable)
				continue
			}
			e.mu.Lock()
			e.active[key] = job.ID
			e.mu.Unlock()
			workers.Add(1)
			go func(grant teamstore.NodeGrant, job teamstore.NodeJob, token nodeToken) {
				defer workers.Done()
				err := e.consume(ownerCtx, grant, job, token)
				e.mu.Lock()
				delete(e.active, grant.NodeID+":"+job.AgentID)
				if err != nil {
					e.workErr = err
				}
				e.mu.Unlock()
				// 释放槽位立即接续积压；未知结果由 duework 退避后重放原持久意图。
				e.loop.Notify()
			}(grant, *job, token)
		}
	}
	return result, resultErr
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

func (e *NodeExecutor) consume(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token nodeToken) (resultErr error) {
	stage := "validate_grant"
	defer func() { e.logFailure(ctx, stage, grant, job, resultErr) }()
	if _, err := e.activeGrant(ctx, grant); err != nil {
		return err
	}
	if job.State == "claiming" || job.State == "ready" {
		stage = "claim_delivery"
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
		stage = "persist_claim"
		if err = e.nodes.store.SaveNodeJob(ctx, job, from, nil); err != nil {
			return err
		}
		e.logger.Info("在线 Agent 领取完成", nodeJobLogAttrs(grant, job)...)
	}
	if job.State == "draining" {
		stage = "drain_outputs"
		return e.drain(ctx, grant, job, token.Token)
	}
	if job.State != "ready" {
		return nil
	}
	stage = "validate_delivery_context"
	if job.Delivery == nil || len(job.Delivery.Messages) == 0 || job.Delivery.Messages[len(job.Delivery.Messages)-1].ID != job.Delivery.MessageID {
		return ErrNodeUnavailable
	}
	stage = "prepare_room"
	room, err := e.prepare(ctx, grant.Scope+":"+job.Delivery.RoomID, job.LocalAgentID)
	if err != nil {
		return err
	}
	job.RoomID, job.ConversationID, job.RoundID = room.Room.ID, room.Conversation.ID, "relay_"+job.ID
	// 先用原 claim 的租约重新鉴权，再 CAS 持久 running，最后才触碰 runtime。
	stage = "renew_before_execution"
	if _, err = e.activeGrant(ctx, grant); err != nil {
		return err
	}
	if _, err = e.relay.SettleDelivery(ctx, token.Token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
		return err
	}
	stage = "execute"
	return e.execute(ctx, grant, job, token)
}

func (e *NodeExecutor) drain(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token string) (resultErr error) {
	stage := "read_output"
	defer func() { e.logFailure(ctx, stage, grant, job, resultErr) }()
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
			stage = "renew_output_lease"
			if _, err = e.relay.SettleDelivery(ctx, token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
				return err
			}
		}
		stage = "publish_output"
		if err = e.relay.DeliveryOutput(ctx, token, job.Delivery.ID, output.ID, output.Input); err != nil {
			if job.State == "draining" && nodeOutputRejected(err) {
				job.State = "failed"
				if saveErr := e.nodes.store.SaveNodeJob(ctx, job, "draining", nil); saveErr != nil {
					return saveErr
				}
			}
			return err
		}
		stage = "ack_output"
		if err = e.nodes.store.AckNodeOutput(ctx, job.ID, output.Sequence); err != nil {
			return err
		}
		e.logger.Info("在线 Agent 输出已确认", append(nodeJobLogAttrs(grant, job), "output_id", output.ID, "output_kind", output.Input.Kind, "sequence", output.Sequence)...)
		stage = "read_output"
	}
	if job.State == "draining" {
		job.State = "completed"
		stage = "complete_job"
		if err := e.nodes.store.SaveNodeJob(ctx, job, "draining", nil); err != nil {
			return err
		}
		e.logger.Info("在线 Agent 任务完成", nodeJobLogAttrs(grant, job)...)
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

func outputText(lease, kind, text string, execution *relaycontract.ExecutionMetadata) *relaycontract.DeliveryOutput {
	return &relaycontract.DeliveryOutput{LeaseID: lease, Kind: kind, Content: relaycontract.MessageContent{Version: 1, Blocks: []relaycontract.ContentBlock{{Type: "markdown", Text: text}}, Execution: execution}}
}

func nodeJobLogAttrs(grant teamstore.NodeGrant, job teamstore.NodeJob) []any {
	attrs := []any{"node_id", grant.NodeID, "owner_user_id", grant.OwnerUserID, "job_id", job.ID,
		"agent_id", job.AgentID, "local_agent_id", job.LocalAgentID, "state", job.State, "round_id", job.RoundID}
	if job.Delivery != nil {
		attrs = append(attrs, "delivery_id", job.Delivery.ID, "source_room_id", job.Delivery.RoomID, "source_message_id", job.Delivery.MessageID)
	}
	return attrs
}

// 同一节点/Agent/阶段一分钟最多告警一次；不记录远端正文、URL、令牌或任务内容。
func (e *NodeExecutor) logFailure(ctx context.Context, stage string, grant teamstore.NodeGrant, job teamstore.NodeJob, err error) {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) || e.logger == nil {
		return
	}
	now := time.Now()
	key := grant.NodeID + ":" + job.AgentID + ":" + stage
	e.mu.Lock()
	if now.Sub(e.logTimes[key]) < time.Minute {
		e.mu.Unlock()
		return
	}
	if e.logTimes == nil {
		e.logTimes = make(map[string]time.Time)
	}
	// 过期项随写入清理，不随任务历史无限增长。
	for key, at := range e.logTimes {
		if now.Sub(at) >= time.Minute {
			delete(e.logTimes, key)
		}
	}
	e.logTimes[key] = now
	e.mu.Unlock()
	attrs := append(nodeJobLogAttrs(grant, job), "stage", stage, "error_type", fmt.Sprintf("%T", err))
	var remote *relaycontract.RemoteError
	var control *nodeRemoteError
	switch {
	case errors.As(err, &remote):
		attrs = append(attrs, "http_status", remote.StatusCode, "remote_code", remote.Code, "remote_request_id", remote.RequestID)
	case errors.As(err, &control):
		attrs = append(attrs, "http_status", control.status)
	case errors.Is(err, ErrNodeLogin):
		attrs = append(attrs, "reason", "node_login_required")
	case errors.Is(err, ErrNodeUnavailable):
		attrs = append(attrs, "reason", "node_unavailable")
	case errors.Is(err, context.DeadlineExceeded):
		attrs = append(attrs, "reason", "deadline_exceeded")
	}
	e.logger.WarnContext(ctx, "在线 Agent 阶段失败", attrs...)
}
