package communication_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestServiceListsAddressBookAndPublishesToMemberRoom(t *testing.T) {
	cfg := communicationTestConfig(t)
	handlertest.MigrateSQLiteFromDir(t, cfg.DatabaseURL, communicationMigrationDir(t))
	agents, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	rooms := serverapp.NewRoomServiceWithDB(cfg, db, agents)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "communication-owner", Role: authctx.RoleOwner,
	})
	amy, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatal(err)
	}
	devin, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Devin"})
	if err != nil {
		t.Fatal(err)
	}
	lucy, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Lucy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = agents.AddAgentContact(ctx, amy.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: devin.AgentID, Alias: "开发搭档",
	}); err != nil {
		t.Fatal(err)
	}
	roomContext, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, lucy.AgentID}, Name: "发布群",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = rooms.MarkConversationStarted(ctx, roomContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	topicContext, err := rooms.CreateConversation(
		ctx,
		roomContext.Room.ID,
		protocol.CreateConversationRequest{Title: "当前 Goal topic"},
	)
	if err != nil {
		t.Fatal(err)
	}
	otherRoomContext, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, lucy.AgentID}, Name: "另一个群",
	})
	if err != nil {
		t.Fatal(err)
	}

	sessionKey := protocol.BuildAgentSessionKey(
		amy.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	roundID := "round-communication"
	roomSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	roomLeaseSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID, amy.AgentID, roomContext.Room.RoomType,
	)
	topicSessionKey := protocol.BuildRoomSharedSessionKey(topicContext.Conversation.ID)
	topicLeaseSessionKey := protocol.BuildRoomAgentSessionKey(
		topicContext.Conversation.ID, amy.AgentID, roomContext.Room.RoomType,
	)
	roundVerifier := fixedRoundVerifier{
		sessionKey:           roundID,
		roomLeaseSessionKey:  "agent-round-room",
		topicLeaseSessionKey: "agent-round-topic",
	}
	actor := managersvc.Actor{
		OwnerUserID: "communication-owner", AgentID: amy.AgentID,
		SessionKey: sessionKey, RoundID: roundID,
		LeaseSessionKey: sessionKey, LeaseRoundID: roundID,
		ContextKind: managersvc.ContextKindAgent, ContextID: amy.AgentID,
	}
	transport := &recordingMessageTransport{}
	service := communicationsvc.NewService(agents, rooms, transport, roundVerifier)
	if opened, openErr := service.OpenContactChannel(ctx, amy.AgentID, devin.AgentID); !errors.Is(openErr, roomsvc.ErrRoomNotFound) || opened != nil {
		t.Fatalf("opening untouched contact created a room: %+v, err = %v", opened, openErr)
	}
	directResult, err := service.SendMessage(ctx, actor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeAgent,
		TargetID:   devin.AgentID,
		Content:    "请看一下实现",
	})
	if err != nil {
		t.Fatal(err)
	}
	if directResult.Status != "queued" || directResult.RoomID == "" ||
		transport.directCalls != 1 ||
		transport.directRequest.SourceAgentID != amy.AgentID ||
		len(transport.directRequest.WakeTargets) != 1 ||
		transport.directRequest.WakeTargets[0] != devin.AgentID ||
		transport.directRequest.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		transport.directRequest.ReplyRoute.Recipients[0] != amy.AgentID ||
		transport.directRequest.ReplyRoute.WakePolicy != protocol.RoomWakePolicyImmediate {
		t.Fatalf("direct send = %+v, request = %+v", directResult, transport.directRequest)
	}
	directRoom, err := rooms.GetRoom(ctx, directResult.RoomID)
	if err != nil || directRoom == nil || !directRoom.Room.IsContactChannel {
		t.Fatalf("direct room = %+v, err = %v", directRoom, err)
	}
	if _, err = service.SendMessage(ctx, actor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeAgent,
		TargetID:   devin.AgentID,
		Content:    "继续",
	}); err != nil {
		t.Fatal(err)
	}
	if transport.roomIDs[1] != directResult.RoomID {
		t.Fatalf("second direct send created another room: %+v", transport.roomIDs)
	}
	contactSession, err := rooms.CreateConversation(ctx, directResult.RoomID, protocol.CreateConversationRequest{
		Title: "第二个联络会话",
	})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := service.OpenContactChannel(ctx, amy.AgentID, devin.AgentID)
	if err != nil || opened.Room.ID != directResult.RoomID {
		t.Fatalf("owner open contact channel = %+v, err = %v", opened, err)
	}
	ownerResult, err := service.SendMessageAsAgent(ctx, amy.AgentID, communicationsvc.SendRequest{
		TargetType:     communicationsvc.TargetTypeAgent,
		TargetID:       devin.AgentID,
		ConversationID: contactSession.Conversation.ID,
		Content:        "从通讯客户端发送",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ownerResult.ConversationID != contactSession.Conversation.ID ||
		transport.conversationIDs[2] != contactSession.Conversation.ID ||
		transport.directRequest.ReplyRoute.WakePolicy != protocol.RoomWakePolicyNone {
		t.Fatalf("owner selected conversation = %+v, transport = %+v", ownerResult, transport.conversationIDs)
	}

	book, err := service.ListAddressBook(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Contacts) != 1 || book.Contacts[0].ContactAgentID != devin.AgentID ||
		book.Contacts[0].DirectRoomID != directResult.RoomID ||
		len(book.Rooms) != 2 || !addressBookHasRoom(book.Rooms, roomContext.Room.ID) ||
		!addressBookHasRoom(book.Rooms, otherRoomContext.Room.ID) {
		t.Fatalf("address book = %+v", book)
	}
	roomBook, err := service.ListAddressBook(ctx, managersvc.Actor{
		OwnerUserID: "communication-owner", AgentID: amy.AgentID,
		SessionKey: roomSessionKey, RoundID: "root-round-room",
		LeaseSessionKey: roomLeaseSessionKey, LeaseRoundID: "agent-round-room",
		ContextKind: managersvc.ContextKindRoom, ContextID: roomContext.Room.ID,
		RoomID: roomContext.Room.ID, ConversationID: roomContext.Conversation.ID,
	})
	if err != nil || roomBook.AgentID != amy.AgentID {
		t.Fatalf("Room runtime address book = %+v, err = %v", roomBook, err)
	}
	topicActor := managersvc.Actor{
		OwnerUserID: "communication-owner", AgentID: amy.AgentID,
		SessionKey: topicSessionKey, RoundID: "root-round-topic",
		LeaseSessionKey: topicLeaseSessionKey, LeaseRoundID: "agent-round-topic",
		ContextKind: managersvc.ContextKindRoom, ContextID: roomContext.Room.ID,
		RoomID: roomContext.Room.ID, ConversationID: topicContext.Conversation.ID,
		GoalCollaborationBinding: func() *protocol.GoalCollaborationBinding {
			return &protocol.GoalCollaborationBinding{
				GoalID: "goal-topic", ObjectiveRevision: 3,
			}
		},
	}
	topicResult, err := service.SendMessage(ctx, topicActor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeRoom,
		TargetID:   roomContext.Room.ID,
		Content:    "@Lucy 请核对当前 topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if topicResult.ConversationID != topicContext.Conversation.ID ||
		topicResult.RoutingSource != communicationsvc.RoutingSourceCurrentContext ||
		transport.publicConversationID != topicContext.Conversation.ID ||
		transport.publicRequest.RootRoundID != "root-round-topic" ||
		transport.publicRequest.GoalCollaborationBinding == nil ||
		transport.publicRequest.GoalCollaborationBinding.GoalID != "goal-topic" ||
		transport.publicRequest.GoalCollaborationBinding.ObjectiveRevision != 3 {
		t.Fatalf("current topic send = %+v, public request = %+v", topicResult, transport.publicRequest)
	}
	publicCalls := transport.publicCalls
	if _, err = service.SendMessage(ctx, topicActor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeRoom,
		TargetID:   otherRoomContext.Room.ID,
		Content:    "不能隐式跨 Room",
	}); err == nil || !strings.Contains(err.Error(), "必须显式指定 conversation_id") {
		t.Fatalf("cross-Room send without conversation_id = %v", err)
	}
	if transport.publicCalls != publicCalls {
		t.Fatalf("fail-closed cross-Room send reached transport: calls=%d", transport.publicCalls)
	}
	explicitCrossRoom, err := service.SendMessage(ctx, topicActor, communicationsvc.SendRequest{
		TargetType:     communicationsvc.TargetTypeRoom,
		TargetID:       otherRoomContext.Room.ID,
		ConversationID: otherRoomContext.Conversation.ID,
		Content:        "显式跨 Room",
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicitCrossRoom.ConversationID != otherRoomContext.Conversation.ID ||
		explicitCrossRoom.RoutingSource != communicationsvc.RoutingSourceExplicit {
		t.Fatalf("explicit cross-Room send = %+v", explicitCrossRoom)
	}
	directCalls := transport.directCalls
	goalDirect, err := service.SendMessage(ctx, topicActor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeAgent,
		TargetID:   devin.AgentID,
		Content:    "请私下复核当前 Goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if transport.directCalls != directCalls+1 ||
		transport.directRequest.RootRoundID != "root-round-topic" ||
		transport.directRequest.GoalCollaborationBinding == nil ||
		transport.directRequest.GoalCollaborationBinding.GoalID != "goal-topic" ||
		transport.directRequest.GoalCollaborationBinding.ObjectiveRevision != 3 ||
		goalDirect.RoutingSource != communicationsvc.RoutingSourceRoomMain {
		t.Fatalf("Goal-attributed direct send = %+v, request = %+v", goalDirect, transport.directRequest)
	}
	if err = agents.DeleteAgentContact(ctx, amy.AgentID, devin.AgentID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SendMessage(ctx, actor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeRoom,
		TargetID:   directResult.RoomID,
		Content:    "绕过已删除好友关系",
	}); err == nil || !strings.Contains(err.Error(), "不能作为群目标") {
		t.Fatalf("contact channel must not bypass deleted friendship: %v", err)
	}
	if _, err = agents.AddAgentContact(ctx, amy.AgentID, protocol.CreateAgentContactRequest{
		ContactAgentID: devin.AgentID, Alias: "重新添加",
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := service.OpenContactChannel(ctx, amy.AgentID, devin.AgentID)
	if err != nil || reopened.Room.ID != directResult.RoomID ||
		reopened.Conversation.ID != directResult.ConversationID {
		t.Fatalf("re-added contact did not restore room: %+v, err = %v", reopened, err)
	}
	restored, err := agents.GetAgentContact(ctx, amy.AgentID, devin.AgentID)
	if err != nil || restored.DirectRoomID != directResult.RoomID {
		t.Fatalf("restored contact = %+v, err = %v", restored, err)
	}

	realtime := realtimesvc.NewService(
		cfg, rooms, agents, runtimectx.NewManager(), permissionctx.NewContext(),
	)
	service = communicationsvc.NewService(agents, rooms, realtime, roundVerifier)
	result, err := service.SendMessage(ctx, actor, communicationsvc.SendRequest{
		TargetType: communicationsvc.TargetTypeRoom,
		TargetID:   roomContext.Room.ID,
		Content:    "同步完成",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "published" || result.RoomID != roomContext.Room.ID || result.MessageID == "" {
		t.Fatalf("send result = %+v", result)
	}
	if result.ConversationID != roomContext.Conversation.ID ||
		result.RoutingSource != communicationsvc.RoutingSourceRoomMain {
		t.Fatalf("Agent-context default Room route = %+v", result)
	}
	messages, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages(
		roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if serialized := fmt.Sprint(messages); !strings.Contains(serialized, "同步完成") ||
		!strings.Contains(serialized, "nexus_comms.send_message") {
		t.Fatalf("room history missing platform message: %+v", messages)
	}
}

type recordingMessageTransport struct {
	directCalls          int
	directRequest        protocol.CreateRoomDirectedMessageRequest
	roomIDs              []string
	conversationIDs      []string
	publicCalls          int
	publicRoomID         string
	publicConversationID string
	publicRequest        protocol.CreateRoomPublicMessageRequest
}

func (r *recordingMessageTransport) HandleDirectedMessage(
	_ context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	r.directCalls++
	r.directRequest = request
	r.roomIDs = append(r.roomIDs, roomID)
	r.conversationIDs = append(r.conversationIDs, conversationID)
	return &protocol.RoomDirectedMessageRecord{
		MessageID:  fmt.Sprintf("direct-%d", r.directCalls),
		WakePolicy: request.WakePolicy,
	}, nil
}

func (r *recordingMessageTransport) HandlePlatformPublicMessage(
	_ context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	r.publicCalls++
	r.publicRoomID = roomID
	r.publicConversationID = conversationID
	r.publicRequest = request
	return protocol.Message{"message_id": "public-recorded"}, nil
}

type fixedRoundVerifier map[string]string

func (v fixedRoundVerifier) GetRunningRoundIDs(sessionKey string) []string {
	if roundID := v[sessionKey]; roundID != "" {
		return []string{roundID}
	}
	return nil
}

func addressBookHasRoom(rooms []communicationsvc.RoomContact, roomID string) bool {
	for _, room := range rooms {
		if room.RoomID == roomID {
			return true
		}
	}
	return false
}

func communicationTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host: "127.0.0.1", Port: 18012, ProjectName: "communication-test",
		APIPrefix: "/nexus/v1", WebSocketPath: "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus", WorkspacePath: filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite", DatabaseURL: filepath.Join(root, "nexus.db"),
	}
}

func communicationMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
