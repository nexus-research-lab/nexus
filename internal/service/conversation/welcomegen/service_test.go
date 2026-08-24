package welcomegen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type fakeProviderResolver struct {
	calls   [][2]string
	failFor map[[2]string]error
}

func (f *fakeProviderResolver) ResolveLLMConfig(
	_ context.Context,
	provider string,
	model string,
) (*clientopts.RuntimeConfig, error) {
	key := [2]string{provider, model}
	f.calls = append(f.calls, key)
	if err := f.failFor[key]; err != nil {
		return nil, err
	}
	return &clientopts.RuntimeConfig{Provider: provider, Model: model}, nil
}

type fakePreferences struct {
	value preferencessvc.Preferences
	err   error
}

func (f fakePreferences) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return f.value, f.err
}

type fakeAgents struct{}

func (fakeAgents) GetAgent(context.Context, string) (*protocol.Agent, error) {
	return nil, errors.New("unexpected GetAgent")
}

func (fakeAgents) GetAgentsByIDs(context.Context, []string) ([]protocol.Agent, error) {
	return nil, errors.New("unexpected GetAgentsByIDs")
}

type fakeRoomEvents struct {
	roomID         string
	conversationID string
	reason         string
}

func (f *fakeRoomEvents) BroadcastRoomResyncRequired(
	_ context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	f.roomID = roomID
	f.conversationID = conversationID
	f.reason = reason
}

func TestSchedulePersistsHostWelcomeOnceAndBroadcastsResync(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "state"))
	provider := &fakeProviderResolver{}
	service := NewService(
		config.Config{WorkspacePath: root},
		provider,
		fakePreferences{value: preferencessvc.Preferences{
			DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
				Provider: "background",
				Model:    "mini",
			},
		}},
		fakeAgents{},
	)
	service.runAsync = func(job func()) { job() }
	var generatedRequest llm.GenerateTextRequest
	generatedCount := 0
	service.generateText = func(_ context.Context, request llm.GenerateTextRequest) (string, error) {
		generatedCount++
		generatedRequest = request
		return "大家好，我是主持人，这个 Room 的群主。直接发任务给我，或用 @AgentName 指定成员。", nil
	}
	events := &fakeRoomEvents{}
	service.SetRoomResyncBroadcaster(events)

	aggregate := protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:                   "room-1",
			OwnerUserID:          "owner-1",
			RoomType:             protocol.RoomTypeGroup,
			Name:                 "发布协作室",
			Description:          "一起完成发布",
			HostAgentID:          "host-1",
			HostAutoReplyEnabled: true,
		},
		Conversation: protocol.ConversationRecord{ID: "conversation-1"},
		Sessions: []protocol.SessionRecord{
			{AgentID: "worker-1", IsPrimary: true},
			{AgentID: "host-1"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "worker-1", Name: "执行者"},
			{AgentID: "host-1", Name: "主持人"},
		},
	}
	service.Schedule(context.Background(), aggregate)
	service.Schedule(context.Background(), aggregate)

	if generatedCount != 1 {
		t.Fatalf("已存在欢迎语不应重复调用模型: %d", generatedCount)
	}
	if generatedRequest.Config.Provider != "background" || generatedRequest.Config.Model != "mini" {
		t.Fatalf("欢迎语未使用后台模型: %+v", generatedRequest.Config)
	}
	if !strings.Contains(generatedRequest.Messages[0].Content, `"speaker_name":"主持人"`) ||
		!strings.Contains(generatedRequest.Messages[0].Content, `"welcome_kind":"room_host"`) ||
		!strings.Contains(generatedRequest.Messages[0].Content, `"host_auto_reply_enabled":true`) {
		t.Fatalf("Room 欢迎语 prompt 缺少群主规则: %s", generatedRequest.Messages[0].Content)
	}
	messages, err := workspacestore.NewRoomHistoryStore(root).ReadMessages(
		"owner-1",
		"conversation-1",
		nil,
	)
	if err != nil {
		t.Fatalf("读取 Room 欢迎语失败: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("欢迎语应幂等写入一次: %+v", messages)
	}
	if messages[0]["agent_id"] != "host-1" {
		t.Fatalf("Room 欢迎语未归因到群主: %+v", messages[0])
	}
	if !strings.Contains(fmt.Sprint(messages[0]["content"]), "想了解 Nexus") {
		t.Fatalf("Room 欢迎语缺少产品功能求助入口: %+v", messages[0]["content"])
	}
	metadata, _ := messages[0]["metadata"].(map[string]any)
	if metadata["subtype"] != "conversation_welcome" ||
		metadata["generated_by"] != "background_model" ||
		metadata["welcome_kind"] != "room_host" {
		t.Fatalf("欢迎语元信息不正确: %+v", metadata)
	}
	if events.roomID != "room-1" || events.conversationID != "conversation-1" ||
		events.reason != "conversation_welcome_created" {
		t.Fatalf("欢迎语 resync 投影不正确: %+v", events)
	}
}

func TestResolveLLMConfigFallsBackFromBackgroundToDefaultModel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "state"))
	provider := &fakeProviderResolver{failFor: map[[2]string]error{
		{"background", "mini"}: errors.New("background unavailable"),
	}}
	service := NewService(
		config.Config{WorkspacePath: root},
		provider,
		fakePreferences{value: preferencessvc.Preferences{
			DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
				Provider: "background",
				Model:    "mini",
			},
			DefaultAgentOptions: protocol.Options{
				Provider: "default-provider",
				Model:    "default-model",
			},
		}},
		fakeAgents{},
	)
	resolved, source, err := service.resolveLLMConfig(
		context.Background(),
		protocol.ConversationContextAggregate{Room: protocol.RoomRecord{OwnerUserID: "owner-1"}},
		protocol.Agent{Options: protocol.Options{Provider: "agent-provider", Model: "agent-model"}},
	)
	if err != nil {
		t.Fatalf("解析欢迎语模型失败: %v", err)
	}
	if resolved.Provider != "default-provider" || resolved.Model != "default-model" || source != "default_model" {
		t.Fatalf("后台模型失败后未回退默认模型: config=%+v source=%s", resolved, source)
	}
	wantCalls := [][2]string{{"background", "mini"}, {"default-provider", "default-model"}}
	if len(provider.calls) != len(wantCalls) {
		t.Fatalf("模型回退调用次数不正确: %+v", provider.calls)
	}
	for index, want := range wantCalls {
		if provider.calls[index] != want {
			t.Fatalf("模型回退顺序不正确: got=%+v want=%+v", provider.calls, wantCalls)
		}
	}
}

func TestNoHostWelcomeUsesPrimaryAgentAndRequiresMention(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "state"))
	service := NewService(
		config.Config{WorkspacePath: root},
		nil,
		nil,
		fakeAgents{},
	)
	aggregate := protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			RoomType:             protocol.RoomTypeGroup,
			HostAutoReplyEnabled: true,
		},
		Sessions: []protocol.SessionRecord{
			{AgentID: "secondary"},
			{AgentID: "primary", IsPrimary: true},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "secondary", Name: "次成员"},
			{AgentID: "primary", Name: "主成员"},
		},
	}
	speaker, _, err := service.resolveParticipants(context.Background(), aggregate)
	if err != nil {
		t.Fatalf("解析无群主欢迎语成员失败: %v", err)
	}
	if speaker.AgentID != "primary" {
		t.Fatalf("无群主时应使用主成员介绍: %+v", speaker)
	}
	fallback := fallbackWelcome(aggregate, speaker)
	if strings.Contains(fallback, "群主") || !strings.Contains(fallback, "@AgentName") ||
		!strings.Contains(fallback, "想了解 Nexus") {
		t.Fatalf("无群主静态欢迎语规则不正确: %s", fallback)
	}
}

func TestNexusMainWelcomeUsesDedicatedControlEntryPrompt(t *testing.T) {
	aggregate := protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{RoomType: protocol.RoomTypeDM},
	}
	speaker := protocol.Agent{AgentID: "main", Name: "Nexus", IsMain: true}
	if got := resolveWelcomeKind(aggregate, speaker); got != welcomeKindNexusMainDM {
		t.Fatalf("Nexus 主智能体欢迎语类型 = %q", got)
	}
	prompt := welcomeSystemPrompt(welcomeKindNexusMainDM)
	if !strings.Contains(prompt, "宿主控制入口") || !strings.Contains(prompt, "创建和管理智能体与 Room") {
		t.Fatalf("Nexus 主智能体欢迎语未使用独立权限提示词: %s", prompt)
	}
	fallback := fallbackWelcome(aggregate, speaker)
	if !strings.Contains(fallback, "main Agent") || !strings.Contains(fallback, "control entry") ||
		!strings.Contains(fallback, "where to find a feature") {
		t.Fatalf("Nexus 主智能体静态欢迎语身份不明确: %s", fallback)
	}
}

func TestProductHelpInvitationMatchesConversationRouting(t *testing.T) {
	cases := []struct {
		name      string
		aggregate protocol.ConversationContextAggregate
		speaker   protocol.Agent
		want      string
	}{
		{
			name: "普通中文 DM 可以直接询问",
			aggregate: protocol.ConversationContextAggregate{
				Room: protocol.RoomRecord{RoomType: protocol.RoomTypeDM},
			},
			speaker: protocol.Agent{Name: "小助理"},
			want:    "也可以直接问我",
		},
		{
			name: "自动接管的 Room 可以直接询问群主",
			aggregate: protocol.ConversationContextAggregate{
				Room: protocol.RoomRecord{
					RoomType:             protocol.RoomTypeGroup,
					HostAgentID:          "host-1",
					HostAutoReplyEnabled: true,
				},
			},
			speaker: protocol.Agent{AgentID: "host-1", Name: "主持人"},
			want:    "也可以直接问我",
		},
		{
			name: "未自动接管的 Room 要求指定成员",
			aggregate: protocol.ConversationContextAggregate{
				Room: protocol.RoomRecord{
					RoomType:    protocol.RoomTypeGroup,
					HostAgentID: "host-1",
				},
			},
			speaker: protocol.Agent{AgentID: "host-1", Name: "主持人"},
			want:    "用 @AgentName 向成员提问",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := appendProductHelpInvitation("欢迎开始使用。", testCase.aggregate, testCase.speaker)
			if !strings.Contains(got, testCase.want) {
				t.Fatalf("欢迎语产品求助入口 = %q，期望包含 %q", got, testCase.want)
			}
		})
	}

	directAggregate := protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{RoomType: protocol.RoomTypeDM},
	}
	directSpeaker := protocol.Agent{Name: "小助理"}
	longWelcome := appendProductHelpInvitation(
		strings.Repeat("长", welcomeMaxRunes),
		directAggregate,
		directSpeaker,
	)
	if len([]rune(longWelcome)) > welcomeMaxRunes || !strings.Contains(longWelcome, "也可以直接问我") {
		t.Fatalf("长欢迎语未在长度限制内保留产品求助入口: %d %q", len([]rune(longWelcome)), longWelcome)
	}
	existingInvitation := "想了解 Nexus 的功能可以问我。"
	if got := appendProductHelpInvitation(existingInvitation, directAggregate, directSpeaker); got != existingInvitation {
		t.Fatalf("已有产品求助入口不应重复追加: %q", got)
	}
}

func TestWelcomeIdentityValidationRejectsMissingOrFalseHostClaims(t *testing.T) {
	if err := validateWelcome("大家好，我是 Nova，欢迎来到 Room。", welcomeKindRoomHost, "Nova"); err == nil {
		t.Fatal("群主欢迎语缺少群主身份时应拒绝")
	}
	if err := validateWelcome("大家好，我是 Nova，这个 Room 的群主。", welcomeKindRoomHost, "Nova"); err != nil {
		t.Fatalf("明确群主身份的欢迎语被拒绝: %v", err)
	}
	if err := validateWelcome("我是 Nova，这个 Room 的群主，请 @AgentName。", welcomeKindRoomMember, "Nova"); err == nil {
		t.Fatal("无群主介绍成员冒充群主时应拒绝")
	}
	if err := validateWelcome("我是 Nova，请用 @AgentName 指定成员。", welcomeKindRoomMember, "Nova"); err != nil {
		t.Fatalf("正确的无群主欢迎语被拒绝: %v", err)
	}
}

func TestPersistDirectWelcomeUsesAgentOverlay(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "state"))
	workspacePath := filepath.Join(root, "agent-workspace")
	service := NewService(config.Config{WorkspacePath: root}, nil, nil, fakeAgents{})
	aggregate := protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:       "dm-room",
			RoomType: protocol.RoomTypeDM,
		},
		Conversation: protocol.ConversationRecord{ID: "dm-conversation"},
	}
	speaker := protocol.Agent{
		AgentID:       "agent-1",
		Name:          "助手",
		WorkspacePath: workspacePath,
	}
	if err := service.persistWelcome(aggregate, speaker, welcomeGeneration{
		text:   "你好，我是助手。",
		source: "static_fallback",
	}); err != nil {
		t.Fatalf("写入 DM 欢迎语失败: %v", err)
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(
		"dm-conversation",
		"agent-1",
		protocol.RoomTypeDM,
	)
	messages, err := workspacestore.NewAgentHistoryStore(root).ReadMessages(
		workspacePath,
		protocol.Session{SessionKey: sessionKey, AgentID: "agent-1"},
		nil,
	)
	if err != nil {
		t.Fatalf("读取 DM 欢迎语失败: %v", err)
	}
	if len(messages) != 1 || messages[0]["agent_id"] != "agent-1" {
		t.Fatalf("DM 欢迎语未写入 Agent overlay: %+v", messages)
	}
}
