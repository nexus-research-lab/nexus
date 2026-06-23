package channels

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"

	_ "modernc.org/sqlite"
)

type stubAgentResolver struct {
	agentByID map[string]*protocol.Agent
}

func (r *stubAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	item := r.agentByID[strings.TrimSpace(agentID)]
	if item == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return item, nil
}

func (r *stubAgentResolver) GetDefaultAgent(_ context.Context) (*protocol.Agent, error) {
	for _, item := range r.agentByID {
		if item != nil && item.IsMain {
			return item, nil
		}
	}
	for _, item := range r.agentByID {
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

type stubPermissionSender struct {
	key    string
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (s *stubPermissionSender) Key() string {
	return s.key
}

func (s *stubPermissionSender) IsClosed() bool {
	return false
}

func (s *stubPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *stubPermissionSender) Events() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]protocol.EventMessage, len(s.events))
	copy(result, s.events)
	return result
}

type recordingDeliveryChannel struct {
	channelType string
	startErr    error

	mu      sync.Mutex
	starts  int
	stops   int
	targets []DeliveryTarget
	texts   []string
}

func (c *recordingDeliveryChannel) ChannelType() string {
	return c.channelType
}

func (c *recordingDeliveryChannel) Start(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts++
	return c.startErr
}

func (c *recordingDeliveryChannel) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stops++
	return nil
}

func (c *recordingDeliveryChannel) SendDeliveryMessage(_ context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets, target)
	c.texts = append(c.texts, text)
	return channelcontract.NewDeliveryResult(target, nil), nil
}

func (c *recordingDeliveryChannel) sentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.targets)
}

type recordingReceiptDeliveryChannel struct {
	recordingDeliveryChannel
	receipt *channelmessage.Receipt
}

func (c *recordingReceiptDeliveryChannel) SendDeliveryMessage(
	ctx context.Context,
	target DeliveryTarget,
	text string,
) (DeliveryResult, error) {
	if _, err := c.recordingDeliveryChannel.SendDeliveryMessage(ctx, target, text); err != nil {
		return DeliveryResult{}, err
	}
	return channelcontract.NewDeliveryResult(target, c.receipt), nil
}

type adoptingDeliveryChannel struct {
	recordingDeliveryChannel
	adopted DeliveryChannel
}

func (c *adoptingDeliveryChannel) AdoptReplacedChannel(replaced DeliveryChannel) bool {
	c.adopted = replaced
	return true
}

func extractAssistantText(message protocol.Message) string {
	items, ok := message["content"].([]map[string]any)
	if !ok {
		rawItems, ok := message["content"].([]any)
		if !ok {
			return ""
		}
		items = make([]map[string]any, 0, len(rawItems))
		for _, raw := range rawItems {
			payload, ok := raw.(map[string]any)
			if ok {
				items = append(items, payload)
			}
		}
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if stringValue(item["type"]) != "text" {
			continue
		}
		text := stringValue(item["text"])
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func newChannelTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := `
	CREATE TABLE automation_delivery_routes (
	    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
	    agent_id VARCHAR(64) NOT NULL,
	    session_key VARCHAR(512) NOT NULL DEFAULT '',
	    mode VARCHAR(32) NOT NULL,
	    channel VARCHAR(64),
	    "to" VARCHAR(255),
	    account_id VARCHAR(64),
	    thread_id VARCHAR(255),
	    enabled BOOLEAN NOT NULL,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
	);
	CREATE TABLE im_channel_configs (
	    owner_user_id VARCHAR(64) NOT NULL,
	    channel_type VARCHAR(32) NOT NULL,
	    agent_id VARCHAR(64) NOT NULL,
	    status VARCHAR(32) NOT NULL DEFAULT 'configured',
	    config_json TEXT NOT NULL DEFAULT '{}',
	    credentials_encrypted TEXT,
	    last_error TEXT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    PRIMARY KEY (owner_user_id, channel_type)
	);
	CREATE TABLE im_channel_accounts (
	    owner_user_id VARCHAR(64) NOT NULL,
	    channel_type VARCHAR(32) NOT NULL,
	    account_id VARCHAR(255) NOT NULL,
	    user_id VARCHAR(255) NOT NULL DEFAULT '',
	    status VARCHAR(32) NOT NULL DEFAULT 'connected',
	    config_json TEXT NOT NULL DEFAULT '{}',
	    credentials_encrypted TEXT,
	    last_error TEXT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    PRIMARY KEY (owner_user_id, channel_type, account_id)
	);
	CREATE TABLE im_pairings (
	    pairing_id VARCHAR(64) NOT NULL PRIMARY KEY,
		    owner_user_id VARCHAR(64) NOT NULL,
		    channel_type VARCHAR(32) NOT NULL,
		    account_id VARCHAR(255) NOT NULL DEFAULT '',
		    chat_type VARCHAR(16) NOT NULL,
	    external_ref VARCHAR(255) NOT NULL,
	    thread_id VARCHAR(255) NOT NULL DEFAULT '',
	    external_name VARCHAR(255),
	    agent_id VARCHAR(64) NOT NULL,
	    status VARCHAR(32) NOT NULL DEFAULT 'pending',
	    source VARCHAR(32) NOT NULL DEFAULT 'manual',
	    last_message_at DATETIME,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		    UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id)
		);`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化 delivery schema 失败: %v", err)
	}
	return db
}
