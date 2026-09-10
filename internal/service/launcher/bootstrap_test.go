package launcher

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type fakeLauncherSessionReader struct {
	err       error
	pages     map[string]*protocol.MessagePage
	requested []string
	limits    []int
}

func (f *fakeLauncherSessionReader) ListDirectorySessions(context.Context) ([]protocol.Session, error) {
	return nil, nil
}

func (f *fakeLauncherSessionReader) GetSessionMessagesPage(
	_ context.Context,
	sessionKey string,
	request sessionsvc.MessagePageRequest,
) (*protocol.MessagePage, error) {
	f.requested = append(f.requested, sessionKey)
	f.limits = append(f.limits, request.Limit)
	if f.err != nil {
		return nil, f.err
	}
	if page, ok := f.pages[sessionKey]; ok {
		return page, nil
	}
	return &protocol.MessagePage{}, nil
}

func TestPreviewFailureUsesExportedRequestLogger(t *testing.T) {
	var output bytes.Buffer
	logger := logx.New(logx.Options{Output: &output, Format: "json"}).With("request_id", "req-preview")
	ctx := logx.WithLogger(context.Background(), logger)
	service := &Service{session: &fakeLauncherSessionReader{err: errors.New("history unavailable")}}
	items := []BootstrapConversation{{SessionKey: "session-a", RoomID: "room-a", RoomType: protocol.RoomTypeDM}}
	service.attachLatestReplyPreviews(ctx, items)
	for _, want := range []string{"req-preview", "session-a", "room-a", "duration_ms", "history unavailable"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("导出诊断缺少 %q: %s", want, output.String())
		}
	}
	if items[0].LastReplyPreview != "" {
		t.Fatal("读取失败不得填充预览")
	}
}

func TestAttachLatestReplyPreviewsIncludesDMConversations(t *testing.T) {
	sharedRoomKey := protocol.BuildRoomSharedSessionKey("conversation-room")
	reader := &fakeLauncherSessionReader{
		pages: map[string]*protocol.MessagePage{
			"agent-session-dm": {
				Items: []protocol.Message{
					{
						"role": "assistant",
						"content": []any{
							map[string]any{"type": "text", "text": "DM latest reply"},
						},
					},
				},
			},
			sharedRoomKey: {
				Items: []protocol.Message{
					{
						"role": "assistant",
						"content": []any{
							map[string]any{"type": "text", "text": "Room latest reply"},
						},
					},
				},
			},
		},
	}
	service := &Service{session: reader}
	items := []BootstrapConversation{
		{
			SessionKey: "agent-session-dm",
			AgentID:    "agent-a",
			RoomType:   protocol.RoomTypeDM,
			Title:      "DM",
		},
		{
			SessionKey:     "room-slot-a",
			RoomID:         "room-1",
			ConversationID: "conversation-room",
			RoomType:       "room",
			Title:          "Room",
		},
		{
			SessionKey:     "room-slot-b",
			RoomID:         "room-1",
			ConversationID: "conversation-room",
			RoomType:       "room",
			Title:          "Room duplicate",
		},
		{
			SessionKey:  "external-dm",
			AgentID:     "agent-b",
			RoomType:    protocol.RoomTypeDM,
			ChannelType: protocol.SessionChannelDiscord,
			Title:       "External DM",
		},
	}

	service.attachLatestReplyPreviews(context.Background(), items)

	if got := items[0].LastReplyPreview; got != "DM latest reply" {
		t.Fatalf("DM preview = %q, want %q", got, "DM latest reply")
	}
	if got := items[1].LastReplyPreview; got != "Room latest reply" {
		t.Fatalf("Room preview = %q, want %q", got, "Room latest reply")
	}
	if got := items[2].LastReplyPreview; got != "" {
		t.Fatalf("duplicate Room preview = %q, want empty", got)
	}
	if got := items[3].LastReplyPreview; got != "" {
		t.Fatalf("external conversation preview = %q, want empty", got)
	}

	wantRequests := []string{"agent-session-dm", sharedRoomKey}
	if !reflect.DeepEqual(reader.requested, wantRequests) {
		t.Fatalf("requested session keys = %#v, want %#v", reader.requested, wantRequests)
	}
	for _, limit := range reader.limits {
		if limit != 2 {
			t.Fatalf("message page limit = %d, want 2", limit)
		}
	}
}

func TestLatestReplyPreviewUsesFinalBody(t *testing.T) {
	text := func(value string) map[string]any {
		return map[string]any{"type": "text", "text": value}
	}
	thinking := map[string]any{"type": "thinking", "thinking": "重要邮件查询超时了，需要检查结果"}
	tool := map[string]any{"type": "tool_use", "name": "listImportantMails"}
	artifact := map[string]any{"type": "workspace_file_artifact"}
	for _, test := range []struct {
		name    string
		content any
		result  map[string]any
		want    string
	}{
		{
			name: "process text before tools does not prefix final reply",
			content: []any{thinking, text("重要邮件查询超时了"), tool,
				map[string]any{"type": "tool_result", "content": "internal result"},
				thinking, text("近一周没有需要紧急处理的邮件："), text("待办邮件：0 封")},
			want: "近一周没有需要紧急处理的邮件： 待办邮件：0 封",
		},
		{
			name:    "typed blocks and trailing files preserve final body",
			content: []map[string]any{text("正在整理"), tool, text("报告已完成"), artifact},
			want:    "报告已完成",
		},
		{name: "thinking only", content: []any{thinking}},
		{name: "tools after commentary are not a final body", content: []any{text("正在查询"), tool}},
		{name: "legacy string", content: "  正文\n内容  ", want: "正文 内容"},
		{name: "result-only fallback", content: []any{thinking, tool}, result: map[string]any{"result": "最终结果"}, want: "最终结果"},
		{name: "body takes precedence over result", content: []any{text("正文")}, result: map[string]any{"result": "其他结果"}, want: "正文"},
		{name: "interrupted reply", content: []any{text("未完成")}, result: map[string]any{"subtype": "interrupted"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			messages := []protocol.Message{{"role": "assistant", "content": test.content, "result_summary": test.result}}
			if got := latestReplyPreview(messages); got != test.want {
				t.Fatalf("preview = %q, want %q", got, test.want)
			}
		})
	}

	t.Run("unfinished process keeps previous body", func(t *testing.T) {
		messages := []protocol.Message{
			{"role": "assistant", "content": []any{text("上一条正文")}},
			{"role": "user", "content": "继续查询"},
			{"role": "assistant", "content": []any{text("正在查询"), tool, thinking}},
		}
		if got := latestReplyPreview(messages); got != "上一条正文" {
			t.Fatalf("preview = %q, want previous body", got)
		}
	})
}
