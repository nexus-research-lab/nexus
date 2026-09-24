// INPUT: 已持久领取的 Delivery、当前 Node 授权和 Room 原生事件。
// OUTPUT: 完整 assistant/final 持久发件箱、精确取消及本地审批会话。
// POS: 远程任务适配到现有 Room runtime；流、工具与思考不进入 Relay。
package team

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	teamstore "github.com/nexus-research-lab/nexus/internal/storage/teamrelay"
)

func (e *NodeExecutor) execute(ctx context.Context, grant teamstore.NodeGrant, job teamstore.NodeJob, token nodeToken) error {
	// 领取时的群成员范围与 Control 当前公开身份取交集，绝不扩成本机可执行成员。
	if len(job.Delivery.AgentIDs) > 0 {
		fresh, err := e.machineTokenWithDirectory(ctx, grant, job.Delivery.AgentIDs)
		if err != nil {
			return err
		}
		token = fresh
	}
	directory := deliveryAgentDirectory(job.Delivery, token.Directory, job.LocalAgentID)
	contextJSON, err := json.Marshal(job.Delivery.Messages)
	if err != nil {
		return err
	}
	if len(contextJSON) > 2<<20 {
		return errors.New("在线任务上下文超过上限")
	}
	content, publicContext, err := deliveryRoomContext(job.Delivery, job.LocalAgentID)
	if err != nil {
		return err
	}
	job.State = "running"
	if err = e.nodes.store.SaveNodeJob(ctx, job, "ready", nil); err != nil {
		return err
	}
	e.logger.Info("在线 Agent 开始本机执行", nodeJobLogAttrs(grant, job)...)
	observer := nodeObserver{executor: e, job: job, done: make(chan struct{}), failed: make(chan error, 1), output: make(chan struct{}, 1)}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		stopErr := e.stopJob(stopCtx, job)
		current, readErr := e.nodes.store.NodeJob(stopCtx, job.OwnerUserID, job.ID)
		if readErr != nil || current == nil {
			if readErr == nil {
				readErr = ErrNodeUnavailable
			}
			e.logFailure(stopCtx, "cleanup_read_job", grant, job, readErr)
			return
		}
		_, grantErr := e.activeGrant(stopCtx, grant)
		if current.State == "running" || stopErr != nil || (current.State == "draining" && errors.Is(grantErr, ErrNodeLogin)) {
			from := current.State
			current.State = "failed"
			if ctx.Err() != nil && stopErr == nil && from == "running" {
				current.State = "cancelled"
			}
			if stopErr != nil {
				current.State = "review_required"
			}
			e.logFailure(stopCtx, "stop_execution", grant, *current, stopErr)
			saveErr := e.nodes.store.SaveNodeJob(stopCtx, *current, from, nil)
			e.logFailure(stopCtx, "cleanup_save_job", grant, *current, saveErr)
			if saveErr == nil {
				e.logger.Info("在线 Agent 执行收尾", nodeJobLogAttrs(grant, *current)...)
			}
		}
		if current.State == "failed" || current.State == "cancelled" || current.State == "review_required" {
			_, settleErr := e.relay.SettleDelivery(stopCtx, token.Token, job.Delivery.ID, job.Delivery.LeaseID, true, current.FailureCode)
			e.logFailure(stopCtx, "fail_delivery", grant, *current, settleErr)
		}
	}()
	attachments, err := e.prepareDeliveryAttachments(ctx, job, token.Token)
	if err != nil {
		return err
	}
	job.Delivery.Messages = nil
	admitted := false
	err = e.start(ctx, roomrealtime.ChatRequest{SessionKey: protocol.BuildRoomSharedSessionKey(job.ConversationID), RoomID: job.RoomID, ConversationID: job.ConversationID, TargetAgentIDs: []string{job.LocalAgentID}, RoundID: job.RoundID, Content: content, Attachments: attachments, PublicContext: publicContext, PublicAgentDirectory: directory, UserMessageID: job.Delivery.MessageID, Internal: true, ExecutionOrigin: "relay", EventObserver: observer.observe}, func(admissionCtx context.Context) error {
		// Room 准备可能很慢；原生 round 注册后、任何 slot 启动前再次验证。
		if err := admissionCtx.Err(); err != nil {
			return err
		}
		if _, err := e.activeGrant(admissionCtx, grant); err != nil {
			return err
		}
		fresh, err := e.machineToken(admissionCtx, grant)
		if err != nil {
			return err
		}
		token = fresh
		if _, err = e.relay.SettleDelivery(admissionCtx, token.Token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
			return err
		}
		_, err = e.activeGrant(admissionCtx, grant)
		admitted = err == nil
		return err
	})
	if err != nil {
		e.logFailure(ctx, "runtime_start", grant, job, err)
		return err
	}
	if !admitted {
		return errors.New("本机未启动目标执行，不进入普通输入队列")
	}
	// 计时器只维护租约；完整输出由原生 Room 观察器在落盘后立即唤醒。
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	renewAt := time.Now().Add(5 * time.Second)
	retryOutput := false
	for {
		outputReady := false
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-observer.failed:
			return err
		case <-observer.done:
			current, err := e.nodes.store.NodeJob(ctx, job.OwnerUserID, job.ID)
			if err != nil {
				return err
			}
			if current == nil {
				return ErrNodeUnavailable
			}
			if current.State == "draining" {
				return e.drain(ctx, grant, *current, token.Token)
			}
			if current.State == "cancelled" {
				return nil
			}
			return errors.New("在线 Agent 本机执行失败")
		case <-ticker.C:
		case <-observer.output:
			outputReady = true
			// 终态已落盘时由上面的终态分支统一 drain，不能继续按 running 处理。
			select {
			case <-observer.done:
				continue
			default:
			}
		}
		if _, err = e.activeGrant(ctx, grant); err != nil {
			return err
		}
		if time.Now().After(token.ExpiresAt.Add(-15 * time.Second)) {
			token, err = e.machineToken(ctx, grant)
			if err != nil {
				e.logFailure(ctx, "refresh_machine_token", grant, job, err)
				return err
			}
		}
		if time.Now().After(renewAt) {
			if _, err = e.relay.SettleDelivery(ctx, token.Token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
				e.logFailure(ctx, "renew_execution_lease", grant, job, err)
				return err
			}
			renewAt = time.Now().Add(5 * time.Second)
		}
		if !outputReady && !retryOutput {
			continue
		}
		err = e.drain(ctx, grant, job, token.Token)
		retryOutput = err != nil
		if err != nil {
			// 输出回执未知可以重放同一 outbox；不重启 runtime，也不换 output ID。
			if nodeOutputRejected(err) {
				return err
			}
		}
	}
}

func deliveryAgentDirectory(delivery *relaycontract.Delivery, entries []nodeAgent, localAgentID string) map[string]string {
	allowed := make(map[string]bool, len(delivery.AgentIDs))
	for _, id := range delivery.AgentIDs {
		allowed[id] = true
	}
	directory := make(map[string]string)
	for _, entry := range entries {
		if !allowed[entry.AgentID] {
			continue
		}
		id := entry.AgentID
		if id == delivery.AgentID {
			id = localAgentID
		}
		directory[id] = entry.Name
	}
	return directory
}

func nodeOutputRejected(err error) bool {
	var remote *relaycontract.RemoteError
	return errors.As(err, &remote) && remote.StatusCode >= 400 && remote.StatusCode < 500 && remote.StatusCode != 401 && remote.StatusCode != 408 && remote.StatusCode != 429
}

type nodeObserver struct {
	executor *NodeExecutor
	job      teamstore.NodeJob
	mu       sync.Mutex
	done     chan struct{}
	failed   chan error
	output   chan struct{}
	closed   bool
}

func (o *nodeObserver) observe(ctx context.Context, event protocol.EventMessage) {
	if event.EventType != protocol.EventTypeMessage && event.EventType != protocol.EventTypeRoundStatus && event.EventType != protocol.EventTypeError {
		return
	}
	round := event.RoundID
	if round == "" {
		round, _ = event.Data["round_id"].(string)
	}
	if round != o.job.RoundID {
		return
	}
	if event.AgentID != "" && event.AgentID != o.job.LocalAgentID {
		return
	}
	// durable 也包含流式快照；只有明确完成的 assistant 才能出本机。
	if event.EventType == protocol.EventTypeMessage && event.DeliveryMode != protocol.DeliveryModeDurable {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	job, err := o.executor.nodes.store.NodeJob(persistCtx, o.job.OwnerUserID, o.job.ID)
	if err == nil && job == nil {
		err = ErrNodeUnavailable
	}
	if err == nil && job != nil && job.State == "running" {
		err = o.apply(persistCtx, job, event)
		if err == nil {
			select {
			case o.output <- struct{}{}:
			default:
			}
		}
	}
	if err != nil {
		o.closed = true
		o.executor.logFailure(persistCtx, "persist_runtime_event", teamstore.NodeGrant{NodeID: o.job.NodeID, OwnerUserID: o.job.OwnerUserID}, o.job, err)
		select {
		case o.failed <- err:
		default:
		}
	}
}

func (o *nodeObserver) apply(ctx context.Context, job *teamstore.NodeJob, event protocol.EventMessage) error {
	if event.EventType == protocol.EventTypeError {
		job.Failed = true
		job.FailureCode = "execution_failed"
		return o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil)
	}
	if event.EventType == protocol.EventTypeRoundStatus {
		terminal, _ := event.Data["is_terminal"].(bool)
		if !terminal {
			return nil
		}
		status, _ := event.Data["status"].(string)
		job.State = "failed"
		if status == "interrupted" || status == "cancelled" {
			job.State = "cancelled"
		}
		var err error
		if status == "finished" && job.CandidateText == "" && len(job.ArtifactIDs) > 0 {
			job.CandidateText = "文件已交付。"
		}
		if status == "finished" && !job.Failed && !job.CandidateSent && job.CandidateText != "" {
			job.State = "draining"
			err = o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", outputText(job.Delivery.LeaseID, "final", job.CandidateText, job.CandidateExecution, job.CandidateMentions))
		} else {
			err = o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil)
		}
		if err == nil {
			o.executor.logger.Info("在线 Agent 收到执行终态", append(nodeJobLogAttrs(teamstore.NodeGrant{NodeID: job.NodeID, OwnerUserID: job.OwnerUserID}, *job), "runtime_status", status, "failed", job.Failed, "has_candidate", job.CandidateText != "", "candidate_sent", job.CandidateSent)...)
			o.closed = true
			close(o.done)
		}
		return err
	}
	role, _ := event.Data["role"].(string)
	if role != "assistant" {
		return nil
	}
	if err := o.captureFiles(ctx, job, event); err != nil {
		job.FailureCode = "artifact_delivery_failed"
		return errors.Join(err, o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil))
	}
	if event.Data["is_complete"] != true {
		return nil
	}
	if summary, ok := event.Data["result_summary"].(map[string]any); ok {
		subtype, _ := summary["subtype"].(string)
		if (subtype != "" && subtype != "success") || summary["is_error"] == true {
			job.Failed = true
			job.FailureCode = "execution_failed"
		}
	}
	text := strings.TrimSpace(message.ExtractAssistantDisplayText(protocol.Message(event.Data)))
	if text == "" {
		return o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil)
	}
	if len(text) > 64<<10 {
		return errors.New("单条在线回复超过 64 KiB")
	}
	id := event.MessageID
	if id == "" {
		id, _ = event.Data["message_id"].(string)
	}
	if id == "" {
		return errors.New("完整回复缺少持久消息身份")
	}
	if job.CandidateID == id && job.CandidateSent {
		return o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil)
	}
	if job.CandidateID != "" && job.CandidateID != id && !job.CandidateSent {
		if err := o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", outputText(job.Delivery.LeaseID, "assistant", job.CandidateText, job.CandidateExecution, job.CandidateMentions)); err != nil {
			return err
		}
		job.Sequence++
		job.OutputBytes += len(job.CandidateText)
	}
	// 仅解码白名单统计；整份 runtime 消息含私人执行内容，不能直接透传。
	encoded, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	var execution relaycontract.ExecutionMetadata
	if err := json.Unmarshal(encoded, &execution); err != nil {
		return err
	}
	if job.CandidateID == id && job.CandidateExecution != nil && execution.Model == "" {
		execution.Model = job.CandidateExecution.Model
	}
	if execution.Model == "" {
		if summary, ok := event.Data["result_summary"].(map[string]any); ok {
			if models, ok := summary["model_usage"].(map[string]any); ok && len(models) == 1 {
				for model := range models {
					execution.Model = model
				}
			}
		}
	}
	job.CandidateExecution = nil
	if execution.Model != "" || execution.ResultSummary != nil {
		job.CandidateExecution = &execution
	}
	// 先投递旧候选，再替换当前消息的目标；空目标也必须清掉旧值。
	job.CandidateMentions = relayMentions(event.Data["agent_mentions"], job.LocalAgentID)
	job.CandidateID, job.CandidateText = id, text
	job.CandidateSent = event.Data["stop_reason"] == "tool_use"
	if job.CandidateSent {
		return o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", outputText(job.Delivery.LeaseID, "assistant", text, job.CandidateExecution, job.CandidateMentions))
	}
	return o.executor.nodes.store.SaveNodeJob(ctx, *job, "running", nil)
}

func relayMentions(value any, sourceAgentID string) []relaycontract.MessageMention {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var values []protocol.AgentMention
	if err := json.Unmarshal(encoded, &values); err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]relaycontract.MessageMention, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value.AgentID)
		if id == "" || id == sourceAgentID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, relaycontract.MessageMention{MemberType: "agent", MemberID: id})
	}
	return result
}

// deliveryRoomContext 只转换消息事实；触发选择和公区增量仍由 Room 机制处理。
func deliveryRoomContext(delivery *relaycontract.Delivery, localAgentID string) (string, []protocol.Message, error) {
	items := make([]protocol.Message, 0, len(delivery.Messages))
	trigger := ""
	found := false
	for _, item := range delivery.Messages {
		var lines []string
		for _, block := range item.Content.Blocks {
			if block.Type == "markdown" {
				lines = append(lines, block.Text)
			}
		}
		text := strings.Join(lines, "\n")
		role := "user"
		agentID := item.AuthorAgentID
		if item.AuthorType == "agent" {
			role = "assistant"
			if agentID == delivery.AgentID {
				agentID = localAgentID
			}
		}
		message := protocol.Message{"message_id": item.ID, "timestamp": item.CreatedAt.UnixMilli(), "role": role, "content": text, "agent_id": agentID, "agent_name": item.AuthorDisplayName, "is_complete": true}
		if role == "user" {
			message["author_user_id"] = item.AuthorUserID
			message["author_username"] = item.AuthorUsername
			message["author_display_name"] = item.AuthorDisplayName
		}
		items = append(items, message)
		if item.ID == delivery.MessageID {
			trigger = text
			found = strings.TrimSpace(text) != "" || len(item.Content.Attachments) > 0
			break
		}
	}
	if !found {
		return "", nil, errors.New("在线投递缺少精确触发消息")
	}
	return trigger, items, nil
}

func (e *NodeExecutor) prepareDeliveryAttachments(ctx context.Context, job teamstore.NodeJob, token string) ([]protocol.ChatAttachment, error) {
	var attachments []protocol.ChatAttachment
	for _, message := range job.Delivery.Messages {
		if message.ID != job.Delivery.MessageID {
			continue
		}
		for _, file := range message.Content.Attachments {
			// 下载受租约约束；每个文件前续租，最终仍由原生 admission 复核。
			if _, err := e.relay.SettleDelivery(ctx, token, job.Delivery.ID, job.Delivery.LeaseID, false); err != nil {
				return nil, err
			}
			readCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			data, err := e.relay.DeliveryFile(readCtx, token, job.Delivery.ID, job.Delivery.LeaseID, file)
			cancel()
			if err != nil {
				return nil, err
			}
			result, err := e.upload(ctx, job.RoomID, job.ConversationID, file.Name, "attachments/"+file.ID, bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			mimeType := http.DetectContentType(data)
			kind := protocol.ChatAttachmentKind("file")
			if strings.HasPrefix(mimeType, "image/") {
				kind = "image"
			}
			if strings.HasPrefix(mimeType, "text/") {
				kind = "text"
			}
			attachments = append(attachments, protocol.ChatAttachment{FileName: file.Name, WorkspacePath: result.Path, RoomID: job.RoomID, ConversationID: job.ConversationID, Scope: "room_conversation", Kind: kind, MIMEType: mimeType, Size: file.Size})
		}
	}
	return attachments, nil
}
