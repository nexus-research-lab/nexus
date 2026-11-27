// domain_integration_test.go 覆盖 Agent/渠道/Provider 删除、成员变更、情绪与 Skill 目录配置集成测试。
package configuration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	appruntime "github.com/nexus-research-lab/nexus/internal/app/runtime"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func TestAgentDeleteRevokesChannelStateRuntimeAndNotifies(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Channel Delete Worker")

	if _, err := fixture.services.ChannelControl.UpsertChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		channels.UpsertChannelConfigRequest{
			AgentID:     worker.AgentID,
			Credentials: map[string]string{"bot_token": "telegram-token"},
		},
	); err != nil {
		t.Fatal(err)
	}
	pairing, err := fixture.services.ChannelControl.CreatePairing(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.CreatePairingRequest{
			ChannelType: channels.ChannelTypeTelegram,
			ChatType:    "dm",
			ExternalRef: "chat-delete",
			AgentID:     worker.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.DB.Exec(`
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json
) VALUES (?, 'telegram', 'account-delete', 'connected', '{}')`,
		worker.OwnerUserID,
	); err != nil {
		t.Fatal(err)
	}
	versionBefore, err := fixture.services.ChannelControl.GetChannelControlVersion(
		fixture.ownerCtx,
		worker.OwnerUserID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.services.Channels.GetForOwner(worker.OwnerUserID, channels.ChannelTypeTelegram) == nil {
		t.Fatal("测试前应存在指向待删除 Agent 的 Channel runtime")
	}

	notifier := &recordingConfigurationNotifier{}
	fixture.services.Configuration.SetNotifier(notifier)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:agent-channel-delete",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "delete", Target: worker.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "agent-channel-delete-01",
		Domain:           configurationsvc.DomainAgents,
		Operation:        "delete",
		Target:           worker.AgentID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	result, err := fixture.services.Configuration.ApplyChange(fixture.ownerCtx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !notifier.hasAgent(worker.AgentID) {
		t.Fatalf("Agent 删除应成功并通知目录失效: result=%+v notified=%v", result, notifier.hasAgent(worker.AgentID))
	}
	if !hasConfigurationCheck(result.Checks, "configuration_target_deleted") {
		t.Fatalf("Agent 删除缺少写后不存在证明: %+v", result)
	}
	if _, err = fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID); err == nil {
		t.Fatal("Agent 删除后目标仍存在")
	}
	if fixture.services.Channels.GetForOwner(worker.OwnerUserID, channels.ChannelTypeTelegram) != nil {
		t.Fatal("Agent 删除后 Channel runtime 仍存在")
	}
	if exists, err := fixture.services.ChannelControl.HasChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
	); err != nil || exists {
		t.Fatalf("Agent 删除后 Channel config 残留: exists=%v err=%v", exists, err)
	}
	if exists, err := fixture.services.ChannelControl.HasChannelAccount(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		"account-delete",
	); err != nil || exists {
		t.Fatalf("Agent 删除后 Channel account 残留: exists=%v err=%v", exists, err)
	}
	if exists, err := fixture.services.ChannelControl.HasPairing(
		fixture.ownerCtx,
		worker.OwnerUserID,
		pairing.PairingID,
	); err != nil || exists {
		t.Fatalf("Agent 删除后 pairing 残留: exists=%v err=%v", exists, err)
	}
	versionAfter, err := fixture.services.ChannelControl.GetChannelControlVersion(
		fixture.ownerCtx,
		worker.OwnerUserID,
	)
	if err != nil || versionAfter != versionBefore+1 {
		t.Fatalf("Agent 删除应只推进一次 Channel version: before=%d after=%d err=%v", versionBefore, versionAfter, err)
	}
}

type recordingConfigurationNotifier struct {
	mu       sync.Mutex
	agentIDs []string
}

func (n *recordingConfigurationNotifier) AgentChanged(_ context.Context, agentID, _ string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.agentIDs = append(n.agentIDs, agentID)
}

func (*recordingConfigurationNotifier) RoomChanged(context.Context, string, string, string) {}

func (*recordingConfigurationNotifier) RoomMemberChanged(context.Context, string, string, bool) {}

func (n *recordingConfigurationNotifier) hasAgent(agentID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Contains(n.agentIDs, agentID)
}

func TestAgentEmotionConversationControlUsesTrustedScopeAndVersionCAS(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Emotion Worker")
	other := fixture.createAgent(t, "Other Emotion Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:emotion",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainEmotion},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := inspection.Domains[configurationsvc.DomainEmotion]
	payload, err := json.Marshal(snapshot.Values)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Authority != configurationsvc.AuthorityAgentSelf ||
		snapshot.Scope.Kind != configurationsvc.ScopeKindAgent ||
		snapshot.Scope.ID != worker.AgentID ||
		snapshot.StateVersion != 1 ||
		len(snapshot.Access.AllowedOperations) != 3 {
		t.Fatalf("unexpected self emotion scope: %+v", inspection)
	}
	if strings.Contains(string(payload), worker.WorkspacePath) ||
		strings.Contains(string(payload), "state_path") {
		t.Fatalf("emotion inspection leaked absolute workspace state path: %s", payload)
	}

	input := json.RawMessage(`{
		"mood":"curious",
		"energy":8,
		"valence":7,
		"description":"engaged with this problem"
	}`)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Target:    other.AgentID,
			Input:     input,
		},
	); err == nil {
		t.Fatal("Agent must not target another Agent emotion state")
	}
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Input:     input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequiresConfirmation ||
		plan.Scope.ID != worker.AgentID ||
		plan.Target != worker.AgentID ||
		plan.StateVersion != snapshot.StateVersion ||
		plan.RuntimeEffect != "next_round" {
		t.Fatalf("unexpected emotion plan: %+v", plan)
	}
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID:        "emotion-self-base-001",
			Domain:           configurationsvc.DomainEmotion,
			Operation:        "set_base",
			Input:            input,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("emotion write lacked version proof: %+v", result)
	}
	view, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		worker.AgentID,
		"dm:"+actor.SessionKey,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Version != plan.StateVersion+1 || view.Base.Mood != "curious" {
		t.Fatalf("emotion base did not persist: %+v", view)
	}
}

func TestRoomEmotionControlOnlyTouchesCurrentAgentAndConversation(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Emotion Room Host")
	member := fixture.createAgent(t, "Emotion Room Member")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs: []string{host.AgentID, member.AgentID},
			Name:     "Emotion Room", HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	memberActor := configurationsvc.Actor{
		OwnerUserID:    member.OwnerUserID,
		AgentID:        member.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			member.AgentID,
			roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &memberActor)
	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		memberActor,
		[]string{configurationsvc.DomainEmotion},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	emotion := inspection.Domains[configurationsvc.DomainEmotion]
	if inspection.Authority != configurationsvc.AuthorityRoomMember ||
		emotion.Scope.ID != member.AgentID ||
		len(emotion.Access.AllowedOperations) != 2 {
		t.Fatalf("unexpected Room emotion boundary: %+v", inspection)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Input: json.RawMessage(`{
				"mood":"forged","energy":1,"valence":1,"description":"forged"
			}`),
		},
	); err == nil {
		t.Fatal("Room member must not change Agent-wide base emotion")
	}

	input := json.RawMessage(`{
		"mood":"encouraged",
		"valence":8,
		"trigger":"the current Room collaboration went well"
	}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_context",
			Input:     input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			RequestID:        "emotion-room-context-001",
			Domain:           configurationsvc.DomainEmotion,
			Operation:        "set_context",
			Input:            input,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope.ID != member.AgentID ||
		!hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Room emotion apply escaped current Agent scope: %+v", result)
	}
	view, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		member.AgentID,
		"room:"+roomContext.Conversation.ID,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Context == nil ||
		view.Context.Mood != "encouraged" ||
		view.Context.Trigger != "the current Room collaboration went well" {
		t.Fatalf("Room conversation emotion not persisted: %+v", view)
	}
	hostView, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		host.AgentID,
		"room:"+roomContext.Conversation.ID,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if hostView.Context != nil {
		t.Fatalf("member emotion mutation changed Room host state: %+v", hostView)
	}
}

type memberTestControl struct {
	configurationTestPrincipalVerifier
	members []authsvc.ControlMember
	writes  int
	role    string
}

func (c *memberTestControl) ResolveActivePrincipalRole(context.Context, string) (string, error) {
	return c.role, nil
}
func (c *memberTestControl) ManageMembers(_ context.Context, owner, session, operation, target string, input json.RawMessage, version int64) (json.RawMessage, error) {
	if session != "admin-session" || owner == "" {
		return nil, errors.New("missing trusted binding")
	}
	if operation == "list" {
		return json.Marshal(c.members)
	}
	c.writes++
	var value authsvc.CreateControlMemberInput
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, err
	}
	if value.Password != "human-password" {
		return nil, errors.New("secret not materialized")
	}
	member := authsvc.ControlMember{UserID: "control-member", Username: value.Username, DisplayName: value.DisplayName, Role: value.Role, MembershipStatus: "active", UpdatedAt: time.Now().UTC()}
	c.members = append(c.members, member)
	return json.Marshal(member)
}

func TestMemberConfigurationRequiresHumanSecretAndLiveAdmin(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	control := &memberTestControl{role: authctx.RoleOwner, members: []authsvc.ControlMember{}}
	fixture.services.Configuration.SetPrincipalVerifiers(control, control)
	session := "admin-session"
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: fixture.main.OwnerUserID, Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword, SessionID: &session})
	actor := configurationsvc.Actor{OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID, SessionKey: "agent:" + fixture.main.AgentID + ":ws:dm:members", ContextKind: configurationsvc.ContextKindAgent, ContextID: fixture.main.AgentID, PrincipalRole: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword, AuthSessionID: session}
	bindConfigurationTestRound(t, fixture.services, &actor)
	request := configurationsvc.ChangeRequest{RequestID: "members-create-001", Domain: configurationsvc.DomainMembers, Operation: "create", Input: json.RawMessage(`{"username":"new-user","display_name":"New","role":"member","password":{"$secret":"member-password"}}`)}
	plan, err := fixture.services.Configuration.PlanChange(ctx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresConfirmation || len(plan.SecretSlots) != 1 {
		t.Fatal("missing human password approval")
	}
	request.PlanDigest = plan.PlanDigest
	request.ExpectedRevision = plan.CurrentRevision
	if _, err = fixture.services.Configuration.ApplyChangeFromCLI(ctx, actor, request, configurationsvc.CLIApplyOptions{Confirmed: true}); err == nil {
		t.Fatal("CLI confirmation bypassed human card")
	}
	normalizedBypass := request
	normalizedBypass.Domain = " MEMBERS "
	if _, bypassErr := fixture.services.Configuration.ApplyChangeFromCLI(ctx, actor, normalizedBypass, configurationsvc.CLIApplyOptions{Confirmed: true}); bypassErr == nil || !strings.Contains(bypassErr.Error(), "实时确认卡片") {
		t.Fatalf("normalized domain bypass: %v", bypassErr)
	}
	if _, err = fixture.services.Configuration.ApplyChange(ctx, actor, request); err == nil {
		t.Fatal("missing approval accepted")
	}
	token, err := fixture.services.Configuration.IssueRuntimeCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	sender := &memberTestSender{events: make(chan protocol.EventMessage, 8)}
	fixture.services.Permission.BindSession(actor.SessionKey, sender)
	lease := fixture.services.Permission.BindSessionRoute(actor.LeaseSessionKey, permissionctx.RouteContext{DispatchSessionKey: actor.SessionKey, AgentID: actor.AgentID, RoundID: actor.RoundID, AgentRoundID: actor.LeaseRoundID})
	defer fixture.services.Permission.UnbindSessionRoute(lease)
	body, _ := json.Marshal(map[string]any{"action": "apply", "change": request, "confirmed": true})
	httpRequest := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/internal/runtime/configuration", bytes.NewReader(body)).WithContext(ctx)
	httpRequest.RemoteAddr = "127.0.0.1:1234"
	httpRequest.Header.Set(protocol.NexusConfigCapabilityHeader, token)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		appruntime.NewConfigurationHandler(fixture.services.Configuration, fixture.services.Permission)(response, httpRequest)
	}()
	select {
	case event := <-sender.events:
		if event.EventType != protocol.EventTypePermissionRequest {
			t.Fatalf("unexpected event: %s", event.EventType)
		}
		if !fixture.services.Permission.HandlePermissionResponse(ctx, actor.SessionKey, map[string]any{"request_id": event.Data["request_id"], "decision": "allow", "configuration_secrets": map[string]any{"member-password": "human-password"}}) {
			t.Fatal("approval rejected")
		}
	case <-done:
		t.Fatalf("broker did not wait for approval: %s", response.Body.String())
	case <-time.After(10 * time.Second):
		t.Fatal("missing approval event")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("broker did not resume")
	}
	if response.Code != http.StatusOK {
		t.Fatalf("broker apply: %d %s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("human-password")) {
		t.Fatal("response leaked password")
	}
	if control.writes != 1 {
		t.Fatalf("writes=%d", control.writes)
	}
	if _, err = fixture.services.Configuration.ApplyChange(ctx, actor, request); err != nil {
		t.Fatal(err)
	}
	if control.writes != 1 {
		t.Fatal("replay duplicated write")
	}
	control.role = authctx.RoleMember
	if _, err = fixture.services.Configuration.Inspect(ctx, actor, []string{configurationsvc.DomainMembers}, false); err == nil {
		t.Fatalf("平台 member 不得读取部署用户目录: %v", err)
	}
}

type memberTestSender struct{ events chan protocol.EventMessage }

func (*memberTestSender) Key() string    { return "member-test" }
func (*memberTestSender) IsClosed() bool { return false }
func (s *memberTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestPrivateSkillSourceConversationUsesSecretsCASAndWriteAfterRead(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	const (
		initialToken = "conversation-private-token-v1"
		rotatedToken = "conversation-private-token-v2"
		skillID      = "conversation-private-skill"
	)
	archive := privateSkillArchive(t, skillID)
	archiveSum := sha256.Sum256(archive)
	var tokenState struct {
		sync.RWMutex
		value string
	}
	tokenState.value = initialToken

	mux := http.NewServeMux()
	var registry *httptest.Server
	mux.HandleFunc("/registry/api/skills", func(writer http.ResponseWriter, request *http.Request) {
		tokenState.RLock()
		expectedToken := tokenState.value
		tokenState.RUnlock()
		if request.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		response := map[string]any{
			"skills": []map[string]any{{
				"id": skillID, "name": skillID, "title": "Conversation Private Skill",
				"description": "private source conversation test", "version": "1.0.0",
				"tags":         []string{"private"},
				"download_url": registry.URL + "/registry/download/skill.zip",
				"sha256":       hex.EncodeToString(archiveSum[:]), "size": len(archive),
				"readme_markdown": "# Conversation Private Skill",
			}},
			"total": 1,
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode private registry response: %v", err)
		}
	})
	mux.HandleFunc("/registry/download/skill.zip", func(writer http.ResponseWriter, request *http.Request) {
		tokenState.RLock()
		expectedToken := tokenState.value
		tokenState.RUnlock()
		if request.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(archive)
	})
	registry = httptest.NewServer(mux)
	t.Cleanup(registry.Close)

	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:private-skill-source",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	createInput := json.RawMessage(`{
		"name":"Conversation Registry",
		"url":"` + registry.URL + `/registry",
		"auth_type":"bearer",
		"token":{"$secret":"private-source-create-token"}
	}`)
	createPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "create_private_source", Input: createInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if createPlan.StateVersion <= 0 || !createPlan.RequiresConfirmation ||
		len(createPlan.SecretSlots) != 1 {
		t.Fatalf("private source create plan did not bind catalog/approval/secret: %+v", createPlan)
	}
	createRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-create-001",
		Domain:    configurationsvc.DomainSkills, Operation: "create_private_source", Input: createInput,
		ExpectedRevision: createPlan.CurrentRevision,
		PlanDigest:       createPlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		createRequest,
		createPlan,
		map[string]string{"private-source-create-token": initialToken},
	)
	created, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		createRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(created.Checks, "skill_private_source_creation_verified") ||
		!hasConfigurationCheck(created.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source create lacked write-after-read/version proof: %+v", created)
	}
	sourceID := privateSourceID(t, fixture.services.Skills, fixture.ownerCtx)
	state, err := fixture.services.Skills.GetCatalogSourceState(fixture.ownerCtx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Exists || !state.Deletable || state.AuthType != "bearer" ||
		!state.CredentialConfigured || state.CatalogVersion != createPlan.StateVersion+1 {
		t.Fatalf("private source state after create = %+v", state)
	}

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainSkills},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	inspectionJSON, err := json.Marshal(inspection)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(inspectionJSON, []byte(initialToken)) ||
		!bytes.Contains(inspectionJSON, []byte(`"credential_configured":true`)) {
		t.Fatalf("private source inspect leaked or omitted safe credential state: %s", inspectionJSON)
	}

	tokenState.Lock()
	tokenState.value = rotatedToken
	tokenState.Unlock()
	updateInput := json.RawMessage(`{
		"name":"Conversation Registry Rotated",
		"token":{"$secret":"private-source-rotate-token"}
	}`)
	updatePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: updateInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	updateRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-update-001",
		Domain:    configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision,
		PlanDigest:       updatePlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		updateRequest,
		updatePlan,
		map[string]string{"private-source-rotate-token": rotatedToken},
	)
	updated, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		updateRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(updated.Checks, "skill_source_configuration_verified") ||
		!hasConfigurationCheck(updated.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source update lacked write-after-read/version proof: %+v", updated)
	}

	staleInput := json.RawMessage(`{"enabled":false}`)
	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: staleInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	staleRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-stale-001",
		Domain:    configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: staleInput,
		ExpectedRevision: stalePlan.CurrentRevision,
		PlanDigest:       stalePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		staleRequest,
		stalePlan,
	)
	concurrentName := "Changed through HTTP service path"
	if _, err = fixture.services.Skills.UpdateExternalSkillSource(
		fixture.ownerCtx,
		sourceID,
		skillsvc.ExternalSkillSourceRequest{Name: &concurrentName},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		staleRequest,
	); err == nil {
		t.Fatal("private source plan must be invalidated by a non-conversation catalog write")
	}

	importInput := json.RawMessage(`{"skill_id":"` + skillID + `"}`)
	importPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "import_private", Target: sourceID, Input: importInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	importRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-import-001",
		Domain:    configurationsvc.DomainSkills, Operation: "import_private", Target: sourceID, Input: importInput,
		ExpectedRevision: importPlan.CurrentRevision,
		PlanDigest:       importPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		importRequest,
		importPlan,
	)
	imported, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		importRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(imported.Checks, "skill_catalog_publication_verified") ||
		!hasConfigurationCheck(imported.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private Skill import lacked publication/version proof: %+v", imported)
	}

	deletePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "delete_private_source", Target: sourceID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	deleteRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-delete-001",
		Domain:    configurationsvc.DomainSkills, Operation: "delete_private_source", Target: sourceID,
		ExpectedRevision: deletePlan.CurrentRevision,
		PlanDigest:       deletePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		deleteRequest,
		deletePlan,
	)
	deleted, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		deleteRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(deleted.Checks, "skill_private_source_deletion_verified") ||
		!hasConfigurationCheck(deleted.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source delete lacked absence/version proof: %+v", deleted)
	}
	deletedSource, err := fixture.services.Skills.GetCatalogSourceState(fixture.ownerCtx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	importedSkill, err := fixture.services.Skills.GetCatalogSkillState(fixture.ownerCtx, skillID)
	if err != nil {
		t.Fatal(err)
	}
	if deletedSource.Exists || !importedSkill.Exists {
		t.Fatalf("source deletion affected wrong state: source=%+v skill=%+v", deletedSource, importedSkill)
	}

	changes, err := fixture.services.Configuration.ListChanges(
		fixture.ownerCtx,
		actor,
		configurationsvc.DomainSkills,
		20,
	)
	if err != nil {
		t.Fatal(err)
	}
	auditJSON, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(auditJSON, []byte(initialToken)) || bytes.Contains(auditJSON, []byte(rotatedToken)) {
		t.Fatalf("private source audit leaked a bearer token: %s", auditJSON)
	}
}

func privateSkillArchive(t *testing.T, name string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(name + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte("---\nname: " + name + "\ntitle: Conversation Private Skill\ndescription: private source conversation test\n---\n\n# Conversation Private Skill\n")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func privateSourceID(
	t *testing.T,
	service *skillsvc.Service,
	ctx context.Context,
) string {
	t.Helper()
	sources, err := service.ListExternalSkillSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.Deletable {
			return source.SourceID
		}
	}
	t.Fatal("private source not found")
	return ""
}
