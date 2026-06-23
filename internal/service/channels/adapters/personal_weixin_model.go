package adapters

const (
	DefaultPersonalWeixinBaseURL       = "https://ilinkai.weixin.qq.com"
	defaultPersonalWeixinBotType       = "3"
	defaultPersonalWeixinAppID         = "bot"
	defaultPersonalWeixinClientVersion = "132099"
	defaultPersonalWeixinBotAgent      = "Nexus/0.1.0"

	personalWeixinMessageTypeUser = 1
	personalWeixinMessageTypeBot  = 2
	personalWeixinMessageStateEnd = 2
	personalWeixinItemTypeText    = 1
	personalWeixinTypingActive    = 1
	personalWeixinTypingCancel    = 2
)

type PersonalWeixinClientConfig struct {
	BaseURL            string
	Token              string
	AccountID          string
	UserID             string
	BotAgent           string
	IlinkAppID         string
	IlinkClientVersion string
}

type PersonalWeixinQRCodeResponse struct {
	QRCode             string `json:"qrcode"`
	QRCodeImageContent string `json:"qrcode_img_content"`
}

type PersonalWeixinQRStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token,omitempty"`
	IlinkBotID   string `json:"ilink_bot_id,omitempty"`
	BaseURL      string `json:"baseurl,omitempty"`
	IlinkUserID  string `json:"ilink_user_id,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}

type personalWeixinGetUpdatesResponse struct {
	Ret                  int                     `json:"ret,omitempty"`
	ErrCode              int                     `json:"errcode,omitempty"`
	ErrMsg               string                  `json:"errmsg,omitempty"`
	Messages             []personalWeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf        string                  `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMS int                     `json:"longpolling_timeout_ms,omitempty"`
}

type personalWeixinConfigResponse struct {
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

type personalWeixinAPIStatus struct {
	Ret     int    `json:"ret,omitempty"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type personalWeixinMessage struct {
	Seq          int64                       `json:"seq,omitempty"`
	MessageID    int64                       `json:"message_id,omitempty"`
	FromUserID   string                      `json:"from_user_id,omitempty"`
	ToUserID     string                      `json:"to_user_id,omitempty"`
	ClientID     string                      `json:"client_id,omitempty"`
	CreateTimeMS int64                       `json:"create_time_ms,omitempty"`
	SessionID    string                      `json:"session_id,omitempty"`
	GroupID      string                      `json:"group_id,omitempty"`
	MessageType  int                         `json:"message_type,omitempty"`
	MessageState int                         `json:"message_state,omitempty"`
	ItemList     []personalWeixinMessageItem `json:"item_list,omitempty"`
	ContextToken string                      `json:"context_token,omitempty"`
}

type personalWeixinMessageItem struct {
	Type     int                    `json:"type,omitempty"`
	TextItem personalWeixinTextItem `json:"text_item,omitempty"`
	RefMsg   *personalWeixinRefMsg  `json:"ref_msg,omitempty"`
}

type personalWeixinTextItem struct {
	Text string `json:"text,omitempty"`
}

type personalWeixinRefMsg struct {
	Title string `json:"title,omitempty"`
}
