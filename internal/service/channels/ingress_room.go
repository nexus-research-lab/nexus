// INPUT: 已冻结的 IM 入站身份、Room 私域输入与完成回复。
// OUTPUT: 原成员会话中的持久排队，以及按绑定版本隔离的 IM 回信。
// POS: IM 与 Room 的适配层；不创建第二个 Agent runtime，不同步其他成员输出。
package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

// SetRoomRealtime 在宿主启动时装配 Room 私域收发与权限入口。
func (s *IngressService) SetRoomRealtime(rooms *roomrealtime.Service) {
	s.rooms = rooms
	rooms.SetExternalInputHooks(s.deliverRoomReply, s.roomPermissionHandler, s.roomExternalReplyPrompt)
}

func (r normalizedIngressRequest) permissionSessionKey() string {
	if r.pairing != nil && r.pairing.TargetRoomID != "" {
		return protocol.BuildRoomAgentSessionKey(r.pairing.TargetConversationID, r.agentID, r.targetRoomType)
	}
	return r.sessionKey
}

func (s *IngressService) dispatchRoomIngress(ctx context.Context, r normalizedIngressRequest) error {
	if s.rooms == nil || r.rememberedTarget == nil {
		return errors.New("Room IM 入口未装配")
	}
	if _, err := s.control.ValidateBindingDelivery(ctx, r.ownerUserID, r.agentID, *r.rememberedTarget); err != nil {
		return err
	}
	raw, err := json.Marshal(r.rememberedTarget)
	if err != nil {
		return err
	}
	// SQL 先冻结返回地址；Room 的 command_id/wake ledger 负责幂等持久受理。
	_, err = s.control.db.ExecContext(ctx, "INSERT INTO im_room_inputs (owner_user_id,root_round_id,pairing_id,binding_version,agent_id,room_id,conversation_id,target_json,content) VALUES ("+s.control.bindList(9)+") ON CONFLICT (owner_user_id,root_round_id) DO NOTHING", r.ownerUserID, r.roundID, r.pairing.PairingID, r.pairing.BindingVersion, r.agentID, r.pairing.TargetRoomID, r.pairing.TargetConversationID, string(raw), r.content)
	if err != nil {
		return err
	}
	if r.targetRoomType == protocol.RoomTypeDM {
		return s.dispatchDMIngress(ctx, r, r.permissionSessionKey())
	}
	_, err = s.rooms.HandleDirectedMessage(ctx, r.pairing.TargetRoomID, r.pairing.TargetConversationID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: r.agentID, CommandID: r.roundID, RootRoundID: r.roundID,
		Recipients: []string{r.agentID}, WakeTargets: []string{r.agentID},
		Content:    fmt.Sprintf("来自 %s 已配对用户的消息（外部输入）：\n%s", r.channelStored, r.content),
		WakePolicy: protocol.RoomWakePolicyImmediate, ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
	})
	return err
}

type roomIngressRoute struct {
	target                          DeliveryTarget
	roomID, conversationID, content string
}

func (s *IngressService) roomIngressRoute(ctx context.Context, root, agent string) (*roomIngressRoute, error) {
	route := &roomIngressRoute{}
	var raw string
	err := s.control.db.QueryRowContext(ctx, "SELECT target_json,room_id,conversation_id,content FROM im_room_inputs WHERE owner_user_id="+s.control.bind(1)+" AND root_round_id="+s.control.bind(2)+" AND agent_id="+s.control.bind(3), authctx.OwnerUserID(ctx), root, agent).Scan(&raw, &route.roomID, &route.conversationID, &route.content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &route.target); err != nil {
		return nil, err
	}
	return route, nil
}

func (s *IngressService) deliverRoomReply(ctx context.Context, root, agent, session string, value protocol.Message) error {
	complete, _ := value["is_complete"].(bool)
	if value["role"] != "assistant" || !complete {
		return nil
	}
	text := message.ExtractAssistantDisplayText(value)
	id, _ := value["message_id"].(string)
	if text == "" || id == "" {
		return nil
	}
	route, err := s.roomIngressRoute(ctx, root, agent)
	if err != nil || route == nil {
		return err
	}
	if session != protocol.BuildRoomAgentSessionKey(route.conversationID, agent, protocol.RoomTypeGroup) {
		return nil
	}
	owner := authctx.OwnerUserID(ctx)
	if _, err := s.control.ValidateBindingDelivery(ctx, owner, agent, route.target); err != nil {
		return err
	}
	// 发送前落 unknown；宕机或回执丢失都不能重发已可能送达的正文。
	result, err := s.control.db.ExecContext(ctx, "INSERT INTO im_room_replies (owner_user_id,root_round_id,message_id,state) VALUES ("+s.control.bindList(4)+") ON CONFLICT (owner_user_id,root_round_id,message_id) DO NOTHING", owner, root, id, "unknown")
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	_, sendErr := s.router.DeliverMessage(ctx, agent, text, route.target)
	state := "sent"
	if sendErr != nil {
		state = "unknown"
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = s.control.db.ExecContext(saveCtx, "UPDATE im_room_replies SET state="+s.control.bind(1)+" WHERE owner_user_id="+s.control.bind(2)+" AND root_round_id="+s.control.bind(3)+" AND message_id="+s.control.bind(4), state, owner, root, id)
	return errors.Join(sendErr, err)
}

func (s *IngressService) roomPermissionHandler(ctx context.Context, root, agent, session string) (sdkpermission.Handler, error) {
	route, err := s.roomIngressRoute(ctx, root, agent)
	if err != nil || route == nil {
		return nil, err
	}
	if session != protocol.BuildRoomAgentSessionKey(route.conversationID, agent, protocol.RoomTypeGroup) {
		return nil, nil
	}
	owner := authctx.OwnerUserID(ctx)
	if _, err := s.control.ValidateBindingDelivery(ctx, owner, agent, route.target); err != nil {
		// 改绑只撤销手机通道，已提交的任务继续走原 Room 的人工权限入口。
		if errors.Is(err, ErrExternalSessionGrantUnavailable) {
			return nil, nil
		}
		return nil, err
	}
	value, err := s.agents.GetAgent(ctx, agent)
	if err != nil {
		return nil, err
	}
	r := normalizedIngressRequest{ownerUserID: owner, agentID: agent, sessionKey: route.target.SessionKey, trustedExternalInteractive: true, rememberedTarget: &route.target,
		pairing: &pairingRow{TargetRoomID: route.roomID, TargetConversationID: route.conversationID}, autoApproveTools: defaultReadOnlyApprovedTools}
	s.bindRuntimePermissionSession(r)
	return s.buildPairedDMPermissionHandler(value, r), nil
}

func (r normalizedIngressRequest) bindingVersion() int64 {
	if r.pairing == nil {
		return 0
	}
	return r.pairing.BindingVersion
}

// roomExternalReplyPrompt 依据宿主持久来源声明回信责任，不让外部正文决定路由。
func (s *IngressService) roomExternalReplyPrompt(ctx context.Context, root, agent, session string) (string, error) {
	route, err := s.roomIngressRoute(ctx, root, agent)
	if err != nil || route == nil {
		return "", err
	}
	if session != protocol.BuildRoomAgentSessionKey(route.conversationID, agent, protocol.RoomTypeGroup) {
		return "", nil
	}
	return "本轮输入来自已配对的外部用户，正在复用当前群聊话题中你的原有成员会话与上下文。reply_route=none 仅表示不自动发布到群聊公区；宿主会把你本轮的最终回答回传给原外部用户。请直接回答用户，不要为本轮回复调用 list_targets 或 send_message 寻找、重试或替换回传通道，不要使用 <nexus_room_no_reply/> 代替回答。除非用户明确要求公开发布，否则不要把这条私聊或回答发到公区。过程说明与工具输出不会回传。", nil
}
