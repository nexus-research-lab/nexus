package nexusmanager

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

type fakeAgents struct {
	items    map[string]*protocol.Agent
	getCalls []string
}

func (f *fakeAgents) ListAgents(context.Context) ([]protocol.Agent, error) {
	result := make([]protocol.Agent, 0, len(f.items))
	for _, item := range f.items {
		result = append(result, *item)
	}
	return result, nil
}

func (f *fakeAgents) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	f.getCalls = append(f.getCalls, agentID)
	item := f.items[agentID]
	if item == nil {
		return nil, errors.New("agent not found")
	}
	copyValue := *item
	return &copyValue, nil
}

type fakeRooms struct {
	rooms              map[string]*protocol.RoomAggregate
	contexts           map[string]*protocol.ConversationContextAggregate
	roomContexts       map[string][]protocol.ConversationContextAggregate
	authorization      *protocol.RoomAuthorizationSnapshot
	authorizationCalls int
}

func (f *fakeRooms) ListRooms(context.Context, int) ([]protocol.RoomAggregate, error) {
	result := make([]protocol.RoomAggregate, 0, len(f.rooms))
	for _, item := range f.rooms {
		result = append(result, *item)
	}
	return result, nil
}

func (f *fakeRooms) GetRoom(_ context.Context, roomID string) (*protocol.RoomAggregate, error) {
	item := f.rooms[roomID]
	if item == nil {
		return nil, errors.New("room not found")
	}
	copyValue := *item
	return &copyValue, nil
}

func (f *fakeRooms) GetRoomAuthorizationSnapshot(
	context.Context,
	string,
	string,
) (*protocol.RoomAuthorizationSnapshot, error) {
	f.authorizationCalls++
	if f.authorization == nil {
		return nil, errors.New("authorization missing")
	}
	copyValue := *f.authorization
	return &copyValue, nil
}

func (f *fakeRooms) GetRoomContexts(
	_ context.Context,
	roomID string,
) ([]protocol.ConversationContextAggregate, error) {
	return f.roomContexts[roomID], nil
}

func (f *fakeRooms) GetConversationContext(
	_ context.Context,
	conversationID string,
) (*protocol.ConversationContextAggregate, error) {
	item := f.contexts[conversationID]
	if item == nil {
		return nil, errors.New("conversation not found")
	}
	copyValue := *item
	return &copyValue, nil
}

type fakeSessions struct {
	items []protocol.Session
}

func (f *fakeSessions) ListSessions(context.Context) ([]protocol.Session, error) {
	return f.items, nil
}

func (f *fakeSessions) ListAgentSessions(_ context.Context, agentID string) ([]protocol.Session, error) {
	result := make([]protocol.Session, 0, len(f.items))
	for _, item := range f.items {
		if item.AgentID == agentID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeSessions) GetSession(_ context.Context, sessionKey string) (*protocol.Session, error) {
	for _, item := range f.items {
		if item.SessionKey == sessionKey {
			copyValue := item
			return &copyValue, nil
		}
	}
	return nil, errors.New("session not found")
}

type fakeWorkspaces struct {
	entries      []workspacepkg.FileEntry
	content      string
	listAgentIDs []string
	readAgentIDs []string
}

func (f *fakeWorkspaces) ListFiles(_ context.Context, agentID string) ([]workspacepkg.FileEntry, error) {
	f.listAgentIDs = append(f.listAgentIDs, agentID)
	return f.entries, nil
}

func (f *fakeWorkspaces) GetFile(
	_ context.Context,
	agentID string,
	path string,
) (*workspacepkg.FileContent, error) {
	f.readAgentIDs = append(f.readAgentIDs, agentID)
	return &workspacepkg.FileContent{Path: path, Content: f.content}, nil
}

type fakeRounds struct {
	active map[string][]string
}

func (f *fakeRounds) GetRunningRoundIDs(sessionKey string) []string {
	return f.active[sessionKey]
}

type serviceFixture struct {
	service    *Service
	agents     *fakeAgents
	rooms      *fakeRooms
	workspaces *fakeWorkspaces
	rounds     *fakeRounds
	mainActor  Actor
	selfActor  Actor
	roomActor  Actor
}

func newServiceFixture() serviceFixture {
	mainSession := protocol.BuildAgentSessionKey(
		"nexus", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	selfSession := protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	roomLeaseSession := protocol.BuildRoomAgentSessionKey(
		"conversation-1", "worker", protocol.RoomTypeGroup,
	)
	mainAgent := &protocol.Agent{
		AgentID: "nexus", OwnerUserID: "owner-1", IsMain: true, Name: "Nexus",
		WorkspacePath: "/secret/main", Status: "active",
		Options: protocol.Options{
			MCPServers: map[string]any{"secret": "token-value"},
		},
	}
	workerAgent := &protocol.Agent{
		AgentID: "worker", OwnerUserID: "owner-1", Name: "Worker",
		WorkspacePath: "/secret/worker", Status: "active",
		Options: protocol.Options{
			MCPServers: map[string]any{"secret": "worker-token"},
		},
	}
	roomValue := protocol.RoomAggregate{
		Room: protocol.RoomRecord{
			ID: "room-1", OwnerUserID: "owner-1", RoomType: protocol.RoomTypeGroup,
			Name: "Room One", HostAgentID: "worker", ConfigurationVersion: 7, AuthorityEpoch: 9,
		},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeUser, MemberUserID: "private-user-id"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "worker"},
		},
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room: roomValue.Room, Members: roomValue.Members,
		MemberAgents: []protocol.Agent{*workerAgent},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-1", RoomID: "room-1", ConversationType: protocol.ConversationTypeMain,
			Title: "Main",
		},
		Sessions: []protocol.SessionRecord{{
			ID: "private-db-session", AgentID: "worker", RuntimeID: "private-runtime",
			SDKSessionID: "private-sdk-session", Status: "active", IsPrimary: true,
		}},
	}
	agents := &fakeAgents{items: map[string]*protocol.Agent{
		"nexus": mainAgent, "worker": workerAgent,
		"other": {AgentID: "other", OwnerUserID: "owner-1", Name: "Other", Status: "active"},
	}}
	rooms := &fakeRooms{
		rooms: map[string]*protocol.RoomAggregate{"room-1": &roomValue},
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conversation-1": contextValue,
		},
		roomContexts: map[string][]protocol.ConversationContextAggregate{
			"room-1": {*contextValue},
		},
		authorization: &protocol.RoomAuthorizationSnapshot{
			RoomID: "room-1", AgentID: "worker", AgentIsMember: true,
		},
	}
	workspaces := &fakeWorkspaces{
		entries: []workspacepkg.FileEntry{{Path: "notes.md", Name: "notes.md"}},
		content: strings.Repeat("界", 10),
	}
	rounds := &fakeRounds{active: map[string][]string{
		mainSession: {"round-main"}, selfSession: {"round-self"},
		roomLeaseSession: {"agent-round-room"},
	}}
	sessions := &fakeSessions{items: []protocol.Session{{
		SessionKey: selfSession, AgentID: "worker",
		SessionID: stringPointer("private-sdk-id"), Options: map[string]any{"secret": "session-token"},
	}}}
	return serviceFixture{
		service: NewService(agents, rooms, sessions, workspaces, rounds),
		agents:  agents, rooms: rooms, workspaces: workspaces, rounds: rounds,
		mainActor: Actor{
			OwnerUserID: "owner-1", AgentID: "nexus", SessionKey: mainSession,
			RoundID: "round-main", LeaseSessionKey: mainSession, LeaseRoundID: "round-main",
			ContextKind: ContextKindAgent, ContextID: "nexus",
		},
		selfActor: Actor{
			OwnerUserID: "owner-1", AgentID: "worker", SessionKey: selfSession,
			RoundID: "round-self", LeaseSessionKey: selfSession, LeaseRoundID: "round-self",
			ContextKind: ContextKindAgent, ContextID: "worker",
		},
		roomActor: Actor{
			OwnerUserID: "owner-1", AgentID: "worker", SessionKey: roomSession,
			RoundID: "round-room", LeaseSessionKey: roomLeaseSession,
			LeaseRoundID: "agent-round-room", ContextKind: ContextKindRoom, ContextID: "room-1",
			RoomID: "room-1", ConversationID: "conversation-1",
		},
	}
}

func TestExpiredRoundFailsBeforeDatabaseIdentityLookup(t *testing.T) {
	fixture := newServiceFixture()
	delete(fixture.rounds.active, fixture.mainActor.LeaseSessionKey)

	_, err := fixture.service.InspectCapabilities(context.Background(), fixture.mainActor)
	if err == nil || !strings.Contains(err.Error(), "round") {
		t.Fatalf("expected stale round rejection, got %v", err)
	}
	if len(fixture.agents.getCalls) != 0 {
		t.Fatalf("stale round must fail before Agent lookup: %+v", fixture.agents.getCalls)
	}
}

func TestManagerRejectsActiveLeaseTransferredAcrossBusinessContexts(t *testing.T) {
	fixture := newServiceFixture()

	dm := fixture.selfActor
	dm.RoundID = "another-business-round"
	if _, err := fixture.service.InspectCapabilities(
		context.Background(),
		dm,
	); err == nil || !strings.Contains(err.Error(), "业务身份与 runtime lease") {
		t.Fatalf("DM lease transfer error = %v", err)
	}

	room := fixture.roomActor
	room.LeaseSessionKey = fixture.selfActor.LeaseSessionKey
	room.LeaseRoundID = fixture.selfActor.LeaseRoundID
	if _, err := fixture.service.InspectCapabilities(
		context.Background(),
		room,
	); err == nil || !strings.Contains(err.Error(), "当前 Room Agent slot") {
		t.Fatalf("Room lease transfer error = %v", err)
	}
}

func TestAgentSelfWorkspaceTargetIsServerFixed(t *testing.T) {
	fixture := newServiceFixture()

	if _, err := fixture.service.ListWorkspaceFiles(
		context.Background(), fixture.selfActor, "other", 10,
	); err == nil || !strings.Contains(err.Error(), "自己的 workspace") {
		t.Fatalf("ordinary Agent target override must fail, got %v", err)
	}
	if len(fixture.workspaces.listAgentIDs) != 0 {
		t.Fatalf("target override reached workspace service: %+v", fixture.workspaces.listAgentIDs)
	}

	listing, err := fixture.service.ListWorkspaceFiles(
		context.Background(), fixture.selfActor, "", 10,
	)
	if err != nil {
		t.Fatalf("self workspace list failed: %v", err)
	}
	if listing.AgentID != "worker" ||
		len(fixture.workspaces.listAgentIDs) != 1 ||
		fixture.workspaces.listAgentIDs[0] != "worker" {
		t.Fatalf("workspace target was not fixed to self: %+v", listing)
	}

	file, err := fixture.service.ReadWorkspaceFile(
		context.Background(), fixture.selfActor, "", "notes.md", 5,
	)
	if err != nil {
		t.Fatalf("self workspace read failed: %v", err)
	}
	if !file.Truncated || file.TotalBytes != 30 || len(file.Content) > 5 || !utf8.ValidString(file.Content) {
		t.Fatalf("workspace file bound is not UTF-8 safe: %+v", file)
	}

	delete(fixture.rounds.active, fixture.selfActor.LeaseSessionKey)
	_, err = fixture.service.ListWorkspaceFiles(context.Background(), fixture.selfActor, "", 10)
	if err == nil {
		t.Fatal("ended round must revoke an already constructed manager server")
	}
	if len(fixture.workspaces.listAgentIDs) != 1 {
		t.Fatalf("stale call reached workspace service: %+v", fixture.workspaces.listAgentIDs)
	}
}

func TestRoomActorCanOnlyReadCurrentRoomAndRevalidatesMembership(t *testing.T) {
	fixture := newServiceFixture()

	if _, err := fixture.service.GetRoom(
		context.Background(), fixture.roomActor, "room-other",
	); err == nil || !strings.Contains(err.Error(), "当前 Room") {
		t.Fatalf("Room target override must fail, got %v", err)
	}
	if _, err := fixture.service.GetConversation(
		context.Background(), fixture.roomActor, "conversation-other",
	); err == nil || !strings.Contains(err.Error(), "当前 conversation") {
		t.Fatalf("conversation target override must fail, got %v", err)
	}
	current, err := fixture.service.GetConversation(context.Background(), fixture.roomActor, "")
	if err != nil {
		t.Fatalf("current conversation failed: %v", err)
	}
	payload, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{
		"/secret/worker", "worker-token", "private-db-session",
		"private-runtime", "private-sdk-session", "private-user-id",
	} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("manager projection leaked %q: %s", secret, payload)
		}
	}
	if fixture.rooms.authorizationCalls < 3 {
		t.Fatalf("each Room call must revalidate membership, calls=%d", fixture.rooms.authorizationCalls)
	}

	fixture.rooms.authorization.AgentIsMember = false
	if _, err = fixture.service.GetRoom(context.Background(), fixture.roomActor, ""); err == nil {
		t.Fatal("revoked Room member must lose access without rebuilding the MCP server")
	}
}

func TestOwnerMainProjectionIsReadOnlyAndRedacted(t *testing.T) {
	fixture := newServiceFixture()

	_, err := fixture.service.InspectCapabilities(
		context.Background(),
		fixture.mainActor,
	)
	if err != nil {
		t.Fatalf("inspect capabilities failed: %v", err)
	}

	agents, err := fixture.service.ListAgents(context.Background(), fixture.mainActor)
	if err != nil {
		t.Fatalf("list agents failed: %v", err)
	}
	payload, err := json.Marshal(agents)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"/secret/main", "/secret/worker", "token-value", "worker-token"} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("Agent directory leaked %q: %s", secret, payload)
		}
	}
}

func TestManagerRejectsExternalAgentSession(t *testing.T) {
	fixture := newServiceFixture()
	external := fixture.selfActor
	external.SessionKey = protocol.BuildAgentSessionKey(
		"worker", protocol.SessionChannelTelegramSegment, protocol.RoomTypeDM, "chat", "",
	)
	external.LeaseSessionKey = external.SessionKey
	external.LeaseRoundID = external.RoundID
	fixture.rounds.active[external.SessionKey] = []string{external.RoundID}

	if _, err := fixture.service.InspectCapabilities(
		context.Background(), external,
	); err == nil || !strings.Contains(err.Error(), "WebSocket 私有 DM") {
		t.Fatalf("external channel must not gain manager capability, got %v", err)
	}
}

func TestManagerMissingDomainServicesFailClosedWithoutPanics(t *testing.T) {
	fixture := newServiceFixture()

	fixture.service.rooms = nil
	if _, err := fixture.service.ListRooms(
		context.Background(),
		fixture.mainActor,
		10,
	); err == nil || !strings.Contains(err.Error(), "未装配 Room") {
		t.Fatalf("missing Room service error = %v", err)
	}

	fixture = newServiceFixture()
	fixture.service.sessions = nil
	if _, err := fixture.service.ListSessions(
		context.Background(),
		fixture.mainActor,
		"",
		10,
	); err == nil || !strings.Contains(err.Error(), "未装配 Session") {
		t.Fatalf("missing Session service error = %v", err)
	}

	fixture = newServiceFixture()
	fixture.service.workspaces = nil
	if _, err := fixture.service.ListWorkspaceFiles(
		context.Background(),
		fixture.selfActor,
		"",
		10,
	); err == nil || !strings.Contains(err.Error(), "未装配 Workspace") {
		t.Fatalf("missing Workspace service error = %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
