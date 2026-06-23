package adapters

type telegramSendMessageResponse struct {
	OK          *bool  `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

type telegramUpdatesEnvelope struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description,omitempty"`
	Result      []telegramUpdate `json:"result,omitempty"`
}

type telegramUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *telegramMessage `json:"message,omitempty"`
	EditedMessage *telegramMessage `json:"edited_message,omitempty"`
}

type telegramMessage struct {
	MessageID       int           `json:"message_id"`
	MessageThreadID int           `json:"message_thread_id,omitempty"`
	Text            string        `json:"text,omitempty"`
	Caption         string        `json:"caption,omitempty"`
	From            *telegramUser `json:"from,omitempty"`
	Chat            telegramChat  `json:"chat"`
}

type telegramUser struct {
	ID    int64 `json:"id"`
	IsBot bool  `json:"is_bot"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
