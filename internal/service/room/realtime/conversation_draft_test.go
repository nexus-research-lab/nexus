package realtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type systemOnlyRoomContextStore struct {
	contextValue *protocol.ConversationContextAggregate
	userCalls    int
	systemCalls  int
	startedCalls int
	startedID    string
}

func (s *systemOnlyRoomContextStore) GetConversationContext(
	context.Context,
	string,
) (*protocol.ConversationContextAggregate, error) {
	s.userCalls++
	return nil, errors.New("request-scoped lookup must not be used by startup recovery")
}

func (s *systemOnlyRoomContextStore) GetConversationContextForSystem(
	context.Context,
	string,
) (*protocol.ConversationContextAggregate, error) {
	s.systemCalls++
	return s.contextValue, nil
}

func (*systemOnlyRoomContextStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (*systemOnlyRoomContextStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (s *systemOnlyRoomContextStore) MarkConversationStarted(
	_ context.Context,
	conversationID string,
	_ time.Time,
) error {
	s.startedCalls++
	s.startedID = conversationID
	return nil
}

func (*systemOnlyRoomContextStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

func TestRoomRequestHasCanonicalUserInput(t *testing.T) {
	tests := []struct {
		name    string
		request ChatRequest
		want    bool
	}{
		{name: "visible user", request: ChatRequest{}, want: true},
		{name: "internal", request: ChatRequest{Internal: true}},
		{
			name: "hidden",
			request: ChatRequest{
				InputOptions: sdkprotocol.OutboundMessageOptions{HiddenFromUser: true},
			},
		},
		{
			name: "synthetic",
			request: ChatRequest{
				InputOptions: sdkprotocol.OutboundMessageOptions{Synthetic: true},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := roomRequestHasCanonicalUserInput(test.request); got != test.want {
				t.Fatalf("roomRequestHasCanonicalUserInput() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidateRoomChatRequestRejectsNonInternalHiddenOrSyntheticInput(t *testing.T) {
	service := &Service{}
	base := ChatRequest{
		SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-input-kind"),
		ConversationID: "conversation-input-kind",
		Content:        "input",
	}
	tests := []struct {
		name         string
		internal     bool
		inputOptions sdkprotocol.OutboundMessageOptions
		wantErr      bool
	}{
		{
			name:         "hidden user",
			inputOptions: sdkprotocol.OutboundMessageOptions{HiddenFromUser: true},
			wantErr:      true,
		},
		{
			name:         "synthetic user",
			inputOptions: sdkprotocol.OutboundMessageOptions{Synthetic: true},
			wantErr:      true,
		},
		{
			name:         "internal hidden synthetic",
			internal:     true,
			inputOptions: sdkprotocol.OutboundMessageOptions{HiddenFromUser: true, Synthetic: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			request.Internal = test.internal
			request.InputOptions = test.inputOptions
			_, _, err := service.validateChatRequest(request)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateChatRequest() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestQueuedRoomUserInputConsumesDraftOnlyAfterMaterialization(t *testing.T) {
	const (
		conversationID = "conversation-queued-user-draft"
		roomID         = "room-queued-user-draft"
	)
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:       roomID,
			RoomType: protocol.RoomTypeGroup,
		},
		Conversation: protocol.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
	}
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	rooms := &systemOnlyRoomContextStore{contextValue: contextValue}
	service := &Service{
		rooms:       rooms,
		roomHistory: workspacestore.NewRoomHistoryStore(filepath.Join(stateRoot, "users")),
		permission:  permissionctx.NewContext(),
	}
	item := protocol.InputQueueItem{
		ID:              "queued-user-round",
		SourceMessageID: "queued-user-message",
		Source:          protocol.InputQueueSourceUser,
		Content:         "排队后再物化",
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)

	if err := service.syncQueuedPublicUserMessage(
		t.Context(),
		sessionKey,
		contextValue,
		item,
		"",
		false,
	); err != nil {
		t.Fatalf("同步未物化 queue 失败: %v", err)
	}
	if rooms.startedCalls != 0 {
		t.Fatalf("尚未进入 canonical 历史的 queue 不得消费 draft: calls=%d", rooms.startedCalls)
	}

	if err := service.syncQueuedPublicUserMessage(
		t.Context(),
		sessionKey,
		contextValue,
		item,
		"",
		true,
	); err != nil {
		t.Fatalf("物化 queue 用户消息失败: %v", err)
	}
	if rooms.startedCalls != 1 || rooms.startedID != conversationID {
		t.Fatalf(
			"queue 用户消息物化后应消费对应 draft: calls=%d id=%s",
			rooms.startedCalls,
			rooms.startedID,
		)
	}
}
