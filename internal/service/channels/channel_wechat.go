package channels

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrWeChatCallbackUnauthorized = errors.New("wechat callback verification failed")

type weChatChannel struct {
	corpID      string
	corpSecret  string
	agentID     string
	client      *http.Client
	baseURL     string
	ownerUserID string

	mu             sync.RWMutex
	accessToken    string
	tokenExpiresAt time.Time
}

type weChatAccessTokenEnvelope struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type weChatMessageEnvelope struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// WeChatIngressPreparation 表示通过企业微信通道配置校验后的回调明文或 URL 验证结果。
type WeChatIngressPreparation struct {
	Body        []byte
	Challenge   string
	OwnerUserID string
	CorpID      string
	AgentID     string
}

type weChatOuterCallback struct {
	ToUserName string `xml:"ToUserName"`
	AgentID    string `xml:"AgentID"`
	Encrypt    string `xml:"Encrypt"`
}

type weChatMessageCallback struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgID        string `xml:"MsgId"`
	AgentID      string `xml:"AgentID"`
}

func newWeChatChannel(corpID string, corpSecret string, agentID string, client *http.Client) *weChatChannel {
	if client == nil {
		client = defaultChannelHTTPClient
	}
	return &weChatChannel{
		corpID:     strings.TrimSpace(corpID),
		corpSecret: strings.TrimSpace(corpSecret),
		agentID:    strings.TrimSpace(agentID),
		client:     client,
		baseURL:    "https://qyapi.weixin.qq.com",
	}
}

func (c *weChatChannel) WithOwner(ownerUserID string) *weChatChannel {
	c.ownerUserID = strings.TrimSpace(ownerUserID)
	return c
}

func (c *weChatChannel) ChannelType() string {
	return ChannelTypeWeChat
}

func (c *weChatChannel) Start(context.Context) error {
	if strings.TrimSpace(c.corpID) == "" || strings.TrimSpace(c.corpSecret) == "" || strings.TrimSpace(c.agentID) == "" {
		return fmt.Errorf("wechat channel is not configured")
	}
	return nil
}

func (c *weChatChannel) Stop(context.Context) error {
	return nil
}

func (c *weChatChannel) SendDeliveryText(ctx context.Context, target DeliveryTarget, text string) error {
	if strings.TrimSpace(target.To) == "" {
		return fmt.Errorf("wechat delivery target requires to")
	}
	agentID, err := strconv.Atoi(strings.TrimSpace(c.agentID))
	if err != nil {
		return fmt.Errorf("wechat agent_id is invalid: %w", err)
	}
	token, err := c.accessTokenForDelivery(ctx)
	if err != nil {
		return err
	}
	targetKey := normalizeWeChatMessageTargetKey(target.AccountID)
	for _, chunk := range splitText(strings.TrimSpace(text), 900) {
		if err = c.sendTextChunk(ctx, token, agentID, targetKey, target.To, chunk); err != nil {
			c.clearAccessToken()
			return err
		}
	}
	return nil
}

func (c *weChatChannel) accessTokenForDelivery(ctx context.Context) (string, error) {
	if strings.TrimSpace(c.corpID) == "" || strings.TrimSpace(c.corpSecret) == "" {
		return "", fmt.Errorf("wechat channel is not configured")
	}
	now := time.Now()
	c.mu.RLock()
	if c.accessToken != "" && now.Before(c.tokenExpiresAt) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	endpoint := strings.TrimRight(c.baseURL, "/") + "/cgi-bin/gettoken?corpid=" +
		url.QueryEscape(c.corpID) + "&corpsecret=" + url.QueryEscape(c.corpSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return "", err
	}
	var envelope weChatAccessTokenEnvelope
	if err = decodeWeChatEnvelope(response, &envelope); err != nil {
		return "", err
	}
	if envelope.ErrCode != 0 {
		return "", fmt.Errorf("wechat access_token failed: errcode=%d errmsg=%s", envelope.ErrCode, strings.TrimSpace(envelope.ErrMsg))
	}
	token := strings.TrimSpace(envelope.AccessToken)
	if token == "" {
		return "", fmt.Errorf("wechat access_token returned empty token")
	}
	expiresIn := envelope.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	if expiresIn > 600 {
		expiresIn -= 300
	}
	c.mu.Lock()
	c.accessToken = token
	c.tokenExpiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	c.mu.Unlock()
	return token, nil
}

func (c *weChatChannel) sendTextChunk(
	ctx context.Context,
	token string,
	agentID int,
	targetKey string,
	targetValue string,
	text string,
) error {
	body := map[string]any{
		targetKey:                  strings.TrimSpace(targetValue),
		"msgtype":                  "text",
		"agentid":                  agentID,
		"text":                     map[string]string{"content": text},
		"safe":                     0,
		"enable_duplicate_check":   0,
		"duplicate_check_interval": 1800,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + "/cgi-bin/message/send?access_token=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	var envelope weChatMessageEnvelope
	if err = decodeWeChatEnvelope(response, &envelope); err != nil {
		return err
	}
	if envelope.ErrCode != 0 {
		return fmt.Errorf("wechat send message failed: errcode=%d errmsg=%s", envelope.ErrCode, strings.TrimSpace(envelope.ErrMsg))
	}
	return nil
}

func (c *weChatChannel) clearAccessToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpiresAt = time.Time{}
}

func decodeWeChatEnvelope(response *http.Response, target any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("wechat request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err = json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

// DecodeWeChatIngressCallback 将企业微信回调明文转换成统一通道入口请求。
func DecodeWeChatIngressCallback(raw []byte) (*IngressRequest, string, error) {
	var payload weChatMessageCallback
	if err := xml.Unmarshal(raw, &payload); err != nil {
		return nil, "", err
	}
	msgType := strings.ToLower(strings.TrimSpace(payload.MsgType))
	if msgType != "text" {
		return nil, "unsupported_msg_type", nil
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return nil, "empty_text", nil
	}
	ref := strings.TrimSpace(payload.FromUserName)
	if ref == "" {
		return nil, "empty_ref", nil
	}
	reqID := firstNonEmpty(payload.MsgID, strconv.FormatInt(payload.CreateTime, 10))
	return &IngressRequest{
		Channel:      ChannelTypeWeChat,
		ChatType:     "dm",
		Ref:          ref,
		Content:      content,
		RoundID:      reqID,
		ReqID:        reqID,
		ExternalName: ref,
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeWeChat,
			To:        ref,
			AccountID: "touser",
		},
	}, "", nil
}

func verifyWeChatCallbackSignature(token string, timestamp string, nonce string, encrypted string, signature string) error {
	expected := weChatCallbackSignature(token, timestamp, nonce, encrypted)
	actual := strings.ToLower(strings.TrimSpace(signature))
	if actual == "" || subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("%w: invalid wechat callback signature", ErrWeChatCallbackUnauthorized)
	}
	return nil
}

func weChatCallbackSignature(token string, timestamp string, nonce string, encrypted string) string {
	values := []string{
		strings.TrimSpace(token),
		strings.TrimSpace(timestamp),
		strings.TrimSpace(nonce),
		strings.TrimSpace(encrypted),
	}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:])
}

func decryptWeChatEncryptedPayload(encrypted string, encodingAESKey string, expectedReceiveID string) ([]byte, string, error) {
	aesKey, err := decodeWeChatAESKey(encodingAESKey)
	if err != nil {
		return nil, "", err
	}
	cipherText, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid wechat encrypted payload", ErrWeChatCallbackUnauthorized)
	}
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, "", fmt.Errorf("%w: invalid wechat encrypted payload length", ErrWeChatCallbackUnauthorized)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, "", err
	}
	plain := append([]byte(nil), cipherText...)
	cipher.NewCBCDecrypter(block, aesKey[:aes.BlockSize]).CryptBlocks(plain, plain)
	plain, err = removeWeChatPKCS7Padding(plain)
	if err != nil {
		return nil, "", err
	}
	if len(plain) < 20 {
		return nil, "", fmt.Errorf("%w: decrypted wechat payload too short", ErrWeChatCallbackUnauthorized)
	}
	messageLen := int(binary.BigEndian.Uint32(plain[16:20]))
	messageStart := 20
	messageEnd := messageStart + messageLen
	if messageLen < 0 || messageEnd > len(plain) {
		return nil, "", fmt.Errorf("%w: invalid wechat payload message length", ErrWeChatCallbackUnauthorized)
	}
	message := bytes.TrimSpace(plain[messageStart:messageEnd])
	receiveID := strings.TrimSpace(string(plain[messageEnd:]))
	if strings.TrimSpace(expectedReceiveID) != "" &&
		receiveID != "" &&
		receiveID != strings.TrimSpace(expectedReceiveID) {
		return nil, "", fmt.Errorf("%w: wechat receive id mismatch", ErrWeChatCallbackUnauthorized)
	}
	return message, receiveID, nil
}

func decodeWeChatAESKey(encodingAESKey string) ([]byte, error) {
	key := strings.TrimSpace(encodingAESKey)
	if key == "" {
		return nil, fmt.Errorf("%w: missing wechat encoding_aes_key", ErrWeChatCallbackUnauthorized)
	}
	if len(key) != 43 {
		return nil, fmt.Errorf("%w: invalid wechat encoding_aes_key length", ErrWeChatCallbackUnauthorized)
	}
	decoded, err := base64.StdEncoding.DecodeString(key + "=")
	if err != nil {
		return nil, fmt.Errorf("%w: invalid wechat encoding_aes_key", ErrWeChatCallbackUnauthorized)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("%w: invalid wechat aes key size", ErrWeChatCallbackUnauthorized)
	}
	return decoded, nil
}

func removeWeChatPKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty wechat decrypted payload", ErrWeChatCallbackUnauthorized)
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, fmt.Errorf("%w: invalid wechat padding", ErrWeChatCallbackUnauthorized)
	}
	if padding > len(data) {
		return nil, fmt.Errorf("%w: invalid wechat padding length", ErrWeChatCallbackUnauthorized)
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("%w: invalid wechat padding bytes", ErrWeChatCallbackUnauthorized)
		}
	}
	return data[:len(data)-padding], nil
}

func normalizeWeChatMessageTargetKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "party", "toparty", "department", "department_id":
		return "toparty"
	case "tag", "totag", "tag_id":
		return "totag"
	default:
		return "touser"
	}
}
