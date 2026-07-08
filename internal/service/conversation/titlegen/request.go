package titlegen

import "strings"

// Request 描述一次标题生成请求。
type Request struct {
	OwnerUserID              string
	SessionKey               string
	Provider                 string
	Model                    string
	Content                  string
	FallbackTitle            string
	SessionTitle             string
	SessionMessageCount      int
	ConversationID           string
	ConversationRoomID       string
	ConversationTitle        string
	ConversationRoomName     string
	ConversationMessageCount int
}

func (r Request) targetKey() string {
	if conversationID := strings.TrimSpace(r.ConversationID); conversationID != "" {
		return "conversation:" + conversationID
	}
	if sessionKey := strings.TrimSpace(r.SessionKey); sessionKey != "" {
		return "session:" + sessionKey
	}
	return ""
}

func (r Request) shouldCheckSessionTitle() bool {
	return strings.TrimSpace(r.SessionKey) != "" &&
		r.SessionMessageCount >= 0 &&
		(r.SessionMessageCount == 0 || isDefaultSessionTitle(r.SessionTitle))
}

func (r Request) shouldCheckConversationTitle() bool {
	return strings.TrimSpace(r.ConversationID) != "" &&
		r.ConversationMessageCount >= 0 &&
		(r.ConversationMessageCount == 0 || isDefaultConversationTitle(r.ConversationTitle, r.ConversationRoomName))
}
