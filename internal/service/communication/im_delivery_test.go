package communication

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) { os.Exit(handlertest.RunWithMinimalAppRoot(m)) }

type imTestRuntime struct{}

func (imTestRuntime) GetRunningRoundIDs(string) []string { return []string{"human-round"} }

type imTestTransport struct {
	calls              int
	allowPrivate       bool
	room, conversation string
	request            protocol.CreateRoomDirectedMessageRequest
}

func (t *imTestTransport) HandleDirectedMessage(_ context.Context, room, conversation string, request protocol.CreateRoomDirectedMessageRequest) (*protocol.RoomDirectedMessageRecord, error) {
	t.calls++
	if t.allowPrivate {
		t.room = room
		t.conversation = conversation
		t.request = request
		return &protocol.RoomDirectedMessageRecord{}, nil
	}
	return nil, errors.New("unexpected private messaging call")
}
func (t *imTestTransport) HandlePlatformPublicMessage(_ context.Context, _, _ string, _ protocol.CreateRoomPublicMessageRequest) (protocol.Message, error) {
	t.calls++
	return nil, errors.New("unexpected public call")
}

type imTestResolver map[string]protocol.Session

func (s imTestResolver) ResolveDeliverySession(_ context.Context, key string) (*protocol.Session, error) {
	v, ok := s[key]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

type imTestInputs struct {
	session, agent, pairing string
	inputs                  map[string]imdelivery.Input
}

func (i *imTestInputs) IMDeliveryPairing(_ context.Context, _, agent, session string) (string, error) {
	if session != i.session || agent != i.agent || i.pairing == "" {
		return "", errors.New("pairing unavailable")
	}
	return i.pairing, nil
}
func (i *imTestInputs) IMDeliveryInput(_ context.Context, _, agent, session, round, content string) (imdelivery.Input, error) {
	v := i.inputs[round]
	if v.ID == "" || v.SessionKey != session || v.AgentID != agent || v.Content != content {
		return v, errors.New("no human evidence")
	}
	return v, nil
}
func (i *imTestInputs) IMDeliveryContentInput(_ context.Context, _, agent, session, id string) (imdelivery.Input, error) {
	for _, v := range i.inputs {
		if v.ID == id && v.SessionKey == session && v.AgentID == agent {
			return v, nil
		}
	}
	return imdelivery.Input{}, errors.New("foreign human message")
}

type imTestReceiver struct {
	store   *imdelivery.Repository
	targets map[string]string
	fail    bool
}

func (r *imTestReceiver) AcceptIMDeliveryReply(ctx context.Context, d imdelivery.Delivery, reply imdelivery.Reply) error {
	if r.fail {
		return errors.New("queue unavailable")
	}
	r.targets[reply.ID] = d.Source.SessionKey
	_, err := r.store.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "pending", "accepted")
	return err
}

func TestIMScenarioRoutesFeedbackWithoutEnteringPrivateMessaging(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: filepath.Join(root, "db.sqlite"), WorkspacePath: filepath.Join(root, "workspace"), DefaultAgentID: "nexus"}
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: authctx.SystemUserID, Role: authctx.RoleOwner})
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	agent, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "工作智能体"})
	if err != nil {
		t.Fatal(err)
	}
	rooms := roomsvc.NewService(cfg, agents, roomrepo.NewSQLRepository("sqlite", db))
	transport := &imTestTransport{}
	service := NewService(agents, rooms, transport, imTestRuntime{}, nil)
	target := protocol.BuildAgentAccountSessionKey(agent.AgentID, "weixin-personal", "dm", "account", "human", "")
	now := time.Now().UTC()
	epoch := now.Format(time.RFC3339Nano)
	sessions := imTestResolver{target: {SessionKey: target, AgentID: agent.AgentID, ChatType: "dm", CreatedAt: now}}
	inputs := &imTestInputs{session: target, agent: agent.AgentID, pairing: "pair", inputs: map[string]imdelivery.Input{}}
	inputs.inputs["human-round"] = imdelivery.Input{ID: "M2", OwnerUserID: authctx.SystemUserID, SessionKey: target, AgentID: agent.AgentID, RoundID: "human-round", PairingID: "pair", Content: "第一份"}
	inputs.inputs["earlier"] = imdelivery.Input{ID: "M1", OwnerUserID: authctx.SystemUserID, SessionKey: target, AgentID: agent.AgentID, RoundID: "earlier", PairingID: "pair", Content: "增加人力"}
	store := imdelivery.NewRepository(cfg, db)
	receiver := &imTestReceiver{store: store, targets: map[string]string{}}
	service.SetIMDeliveryAdapter(store, sessions, inputs, receiver)
	for _, id := range []string{"A", "B"} {
		key := protocol.BuildAgentSessionKey(agent.AgentID, "ws", "dm", id, "")
		sessions[key] = protocol.Session{SessionKey: key, AgentID: agent.AgentID, ChatType: "dm", CreatedAt: now}
		d := imdelivery.Delivery{ID: id, OwnerUserID: authctx.SystemUserID, Source: imdelivery.Source{Kind: "tool", AgentID: agent.AgentID, SessionKey: key, SessionCreatedAt: epoch}, TargetAgentID: agent.AgentID, TargetSessionKey: target, TargetCreatedAt: epoch, PairingID: "pair", Content: "草案"}
		if _, err = store.SaveDelivery(ctx, d); err != nil {
			t.Fatal(err)
		}
		if _, err = store.ClaimSend(ctx, d.OwnerUserID, id, false); err != nil {
			t.Fatal(err)
		}
		if err = store.FinishSend(ctx, d.OwnerUserID, id, "sent", ""); err != nil {
			t.Fatal(err)
		}
	}
	actor := Actor{OwnerUserID: authctx.SystemUserID, AgentID: agent.AgentID, SessionKey: target, RoundID: "human-round", LeaseSessionKey: target, LeaseRoundID: "human-round", ContextKind: ContextKindAgent, ContextID: agent.AgentID, InputContent: "第一份"}
	listed, err := service.ListDeliverySources(ctx, actor, DeliverySourceQuery{})
	if err != nil || len(listed.Sources) != 2 || listed.CurrentInputID != "M2" {
		t.Fatalf("source lookup %+v %v", listed, err)
	}
	first, err := service.ReplyToDelivery(ctx, actor, "A", "增加人力", []string{"M1"})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := service.ReplyToDelivery(ctx, actor, "A", "增加人力", []string{"M1"})
	if err != nil || retry.ReplyID != first.ReplyID || len(receiver.targets) != 1 {
		t.Fatalf("duplicate admission %+v %v", retry, err)
	}
	if _, err = service.ReplyToDelivery(ctx, actor, "A", "批准", []string{"M1"}); !errors.Is(err, imdelivery.ErrConflict) {
		t.Fatalf("changed intent accepted %v", err)
	}
	receiver.fail = true
	if _, err = service.ReplyToDelivery(ctx, actor, "B", "增加人力", []string{"M1"}); err == nil {
		t.Fatal("queue failure was reported as accepted")
	}
	receiver.fail = false
	if err = service.RecoverIMReplies(ctx); err != nil {
		t.Fatal(err)
	}
	if len(receiver.targets) != 2 || transport.calls != 0 {
		t.Fatalf("wrong scenario dispatch: %+v private=%d", receiver.targets, transport.calls)
	}
	if receiver.targets[first.ReplyID] != protocol.BuildAgentSessionKey(agent.AgentID, "ws", "dm", "A", "") {
		t.Fatal("feedback went to wrong original session")
	}
	bad := actor
	bad.InputContent = "forged"
	if _, err = service.ReplyToDelivery(ctx, bad, "A", "forged", nil); err == nil {
		t.Fatal("untrusted input accepted")
	}
	if _, err = service.ReplyToDelivery(ctx, actor, "A", "text", []string{"other-session-message"}); err == nil {
		t.Fatal("foreign content source accepted")
	}

	// A Room source goes only to the exact original member inbox, with no public fallback.
	peer, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "协作智能体"})
	if err != nil {
		t.Fatal(err)
	}
	room, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agent.AgentID, peer.AgentID}, Name: "反馈源群", PrivateMessagesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := rooms.CreateConversation(ctx, room.Room.ID, protocol.CreateConversationRequest{Title: "独立任务"})
	if err != nil {
		t.Fatal(err)
	}
	groupKey := "exact-group-session"
	sessions[groupKey] = protocol.Session{SessionKey: groupKey, ChatType: protocol.RoomTypeGroup, CreatedAt: now}
	d := imdelivery.Delivery{ID: "group-origin", OwnerUserID: authctx.SystemUserID, Source: imdelivery.Source{Kind: "tool", AgentID: agent.AgentID, SessionKey: groupKey, SessionCreatedAt: epoch, RoomID: room.Room.ID, ConversationID: conversation.Conversation.ID}, TargetAgentID: agent.AgentID, TargetSessionKey: target, TargetCreatedAt: epoch, PairingID: "pair", Content: "群任务"}
	if _, err = store.SaveDelivery(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSend(ctx, d.OwnerUserID, d.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishSend(ctx, d.OwnerUserID, d.ID, "sent", ""); err != nil {
		t.Fatal(err)
	}
	transport.allowPrivate = true
	receipt, err := service.ReplyToDelivery(ctx, actor, d.ID, "增加人力", []string{"M1"})
	if err != nil || receipt.Status != "queued" {
		t.Fatalf("group admission %+v %v", receipt, err)
	}
	if transport.room != room.Room.ID || transport.conversation != conversation.Conversation.ID || transport.request.ReplyRoute.Mode != protocol.RoomReplyRouteNone || transport.request.RootRoundID != receipt.ReplyID || len(transport.request.Recipients) != 1 || transport.request.Recipients[0] != agent.AgentID {
		t.Fatalf("incorrect group route %+v", transport)
	}
	delete(sessions, groupKey)
	if _, err = service.ReplyToDelivery(ctx, actor, d.ID, "增加人力", []string{"M1"}); err == nil {
		t.Fatal("deleted source accepted")
	}
	inputs.pairing = "different-pairing"
	if _, err = service.ReplyToDelivery(ctx, actor, "A", "增加人力", []string{"M1"}); err == nil {
		t.Fatal("rebound pairing accepted old source")
	}
}
