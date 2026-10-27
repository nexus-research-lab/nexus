// INPUT: 已冻结的 IM 入站身份、Room 公区输入与完成回复。
// OUTPUT: 以外部用户身份公开发言并 @ 绑定 Agent 的群聊轮次，以及按根轮次统一回传的 IM 回信。
// POS: IM 与 Room 的适配层；不创建第二个 Agent runtime，回信只由宿主转发，模型不参与选路。
package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
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
	// 群聊绑定：外部消息作为公区用户发言并定向唤醒绑定 Agent，群成员与 app 都能看到原话和流式回复；
	// 根轮次即入站 round，后续 @ 交接沿用同一根，宿主据此统一回传。
	return s.rooms.HandleChat(ctx, roomrealtime.ChatRequest{
		SessionKey:           protocol.BuildRoomSharedSessionKey(r.pairing.TargetConversationID),
		RoomID:               r.pairing.TargetRoomID,
		ConversationID:       r.pairing.TargetConversationID,
		Content:              r.content,
		TargetAgentIDs:       []string{r.agentID},
		RoundID:              r.roundID,
		BroadcastUserMessage: true,
		ExecutionOrigin:      "channel",
		ExternalSource:       r.externalSource(),
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Metadata: channelmessage.RuntimeMetadata(r.message),
		},
	})
}

// externalSource 只投影展示所需的渠道与发送者称呼，不暴露平台账号原值。
func (r normalizedIngressRequest) externalSource() map[string]string {
	source := map[string]string{"channel": normalizeChannelType(r.channelStored)}
	name := ""
	if r.message != nil {
		name = strings.TrimSpace(r.message.SenderName)
	}
	if name == "" && r.pairing != nil {
		name = strings.TrimSpace(r.pairing.ExternalName.String)
	}
	if name != "" {
		source["name"] = name
	}
	return source
}

type roomIngressRoute struct {
	target                                   DeliveryTarget
	agentID, roomID, conversationID, content string
}

// roomIngressRoute 读取根轮次冻结的回传地址；agent 非空时只匹配绑定 Agent（权限与提示），
// 为空时匹配整条根轮次（回信：被 @ 接手的成员同样回传）。
func (s *IngressService) roomIngressRoute(ctx context.Context, root, agent string) (*roomIngressRoute, error) {
	route := &roomIngressRoute{}
	var raw string
	query := "SELECT target_json,agent_id,room_id,conversation_id,content FROM im_room_inputs WHERE owner_user_id=" + s.control.bind(1) + " AND root_round_id=" + s.control.bind(2)
	args := []any{authctx.OwnerUserID(ctx), root}
	if agent != "" {
		query += " AND agent_id=" + s.control.bind(3)
		args = append(args, agent)
	}
	err := s.control.db.QueryRowContext(ctx, query, args...).Scan(&raw, &route.agentID, &route.roomID, &route.conversationID, &route.content)
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
	route, err := s.roomIngressRoute(ctx, root, "")
	if err != nil || route == nil {
		return err
	}
	if session != protocol.BuildRoomAgentSessionKey(route.conversationID, agent, protocol.RoomTypeGroup) {
		return nil
	}
	owner := authctx.OwnerUserID(ctx)
	// 回传资格只认绑定 Agent 的配对；改绑后旧根轮次的任何成员都不再回信。
	if _, err := s.control.ValidateBindingDelivery(ctx, owner, route.agentID, route.target); err != nil {
		return err
	}
	if agent != route.agentID {
		text = s.prefixRoomReplySpeaker(ctx, agent, text)
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
	_, sendErr := s.router.DeliverMessage(ctx, route.agentID, text, route.target)
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
	return "本轮最新的群聊用户消息来自已配对的外部用户（经即时通讯渠道发来），已公开发在群里并点名由你处理。像回复群成员一样直接在群里作答：宿主会把本轮你的最终回答，以及你在群里 @ 其他成员后他们的公开回复，自动转回该外部用户。不要调用 send_message 的 external_session 或 delivery_source 自行回传，也不要使用 <nexus_room_no_reply/> 跳过回答；需要其他成员协助时直接在回答里 @ 对方。过程说明与工具输出不会回传。", nil
}

// prefixRoomReplySpeaker 让外部用户分清是谁在回复；读取失败时退回无前缀，不阻断回传。
func (s *IngressService) prefixRoomReplySpeaker(ctx context.Context, agentID string, text string) string {
	value, err := s.agents.GetAgent(ctx, agentID)
	if err != nil || value == nil || strings.TrimSpace(value.Name) == "" {
		return text
	}
	return "【" + strings.TrimSpace(value.Name) + "】" + text
}

// HostOwnsRoomExternalReply 判断 Room 根轮次是否来自已配对 IM 入站；这类轮次只由宿主回传，模型不得自行选路。
func (s *ControlService) HostOwnsRoomExternalReply(ctx context.Context, ownerUserID string, rootRoundID string) (bool, error) {
	rootRoundID = strings.TrimSpace(rootRoundID)
	if rootRoundID == "" {
		return false, nil
	}
	var found int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM im_room_inputs WHERE owner_user_id="+s.bind(1)+" AND root_round_id="+s.bind(2)+" LIMIT 1", normalizeChannelOwnerUserID(ownerUserID), rootRoundID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
