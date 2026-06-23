package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

type PersonalWeixinMultiAccountChannel struct {
	mu       sync.RWMutex
	accounts map[string]*PersonalWeixinChannel
}

func NewPersonalWeixinMultiAccountChannel(accounts []*PersonalWeixinChannel) *PersonalWeixinMultiAccountChannel {
	result := &PersonalWeixinMultiAccountChannel{
		accounts: make(map[string]*PersonalWeixinChannel, len(accounts)),
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		key := personalWeixinAccountKey(account)
		if key == "" {
			key = fmt.Sprintf("account-%d", len(result.accounts)+1)
		}
		result.accounts[key] = account
	}
	return result
}

func (c *PersonalWeixinMultiAccountChannel) ChannelType() string {
	return channelcontract.ChannelTypeWeixinPersonal
}

func (c *PersonalWeixinMultiAccountChannel) Start(ctx context.Context) error {
	return startPersonalWeixinAccounts(ctx, c.snapshotAccounts(), (*PersonalWeixinChannel).Start, (*PersonalWeixinChannel).Stop)
}

func startPersonalWeixinAccounts(
	ctx context.Context,
	accounts []*PersonalWeixinChannel,
	start func(*PersonalWeixinChannel, context.Context) error,
	stop func(*PersonalWeixinChannel, context.Context) error,
) error {
	started := make([]*PersonalWeixinChannel, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if err := start(account, ctx); err != nil {
			for _, item := range started {
				_ = stop(item, ctx)
			}
			return err
		}
		started = append(started, account)
	}
	return nil
}

func (c *PersonalWeixinMultiAccountChannel) Stop(ctx context.Context) error {
	for _, account := range c.snapshotAccounts() {
		_ = account.Stop(ctx)
	}
	return nil
}

func (c *PersonalWeixinMultiAccountChannel) SetIngress(ingress channelcontract.IngressAcceptor) {
	for _, account := range c.snapshotAccounts() {
		account.SetIngress(ingress)
	}
}

func (c *PersonalWeixinMultiAccountChannel) AdoptReplacedChannel(replaced channelcontract.DeliveryChannel) bool {
	accounts := personalWeixinAccountsByKey(replaced)
	if len(accounts) == 0 {
		return false
	}
	adopted := false
	stale := make([]*PersonalWeixinChannel, 0)
	c.mu.Lock()
	for key, account := range accounts {
		if key == "" || account == nil {
			continue
		}
		if current := c.accounts[key]; current != nil && strings.TrimSpace(current.token) != strings.TrimSpace(account.token) {
			stale = append(stale, account)
			continue
		}
		c.accounts[key] = account
		adopted = true
	}
	c.mu.Unlock()
	for _, account := range stale {
		_ = account.Stop(context.Background())
	}
	return adopted
}

func (c *PersonalWeixinMultiAccountChannel) SendDeliveryMessage(
	ctx context.Context,
	target channelcontract.DeliveryTarget,
	text string,
) (channelcontract.DeliveryResult, error) {
	account, err := c.accountForTarget(target)
	if err != nil {
		return channelcontract.DeliveryResult{}, err
	}
	return account.SendDeliveryMessage(ctx, target, text)
}

func (c *PersonalWeixinMultiAccountChannel) SendDeliveryTyping(ctx context.Context, target channelcontract.DeliveryTarget, active bool) error {
	account, err := c.accountForTarget(target)
	if err != nil {
		return err
	}
	return account.SendDeliveryTyping(ctx, target, active)
}

func (c *PersonalWeixinMultiAccountChannel) accountForTarget(target channelcontract.DeliveryTarget) (*PersonalWeixinChannel, error) {
	normalized := target.Normalized()
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.accounts) == 0 {
		return nil, fmt.Errorf("personal weixin channel has no logged-in accounts")
	}
	if accountID := strings.TrimSpace(normalized.AccountID); accountID != "" {
		if account := c.accounts[accountID]; account != nil {
			return account, nil
		}
		return nil, fmt.Errorf("personal weixin account is not connected: %s", accountID)
	}
	if len(c.accounts) == 1 {
		for _, account := range c.accounts {
			return account, nil
		}
	}
	return nil, fmt.Errorf("personal weixin delivery target requires account_id when multiple accounts are connected")
}

func (c *PersonalWeixinMultiAccountChannel) snapshotAccounts() []*PersonalWeixinChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*PersonalWeixinChannel, 0, len(c.accounts))
	for _, account := range c.accounts {
		if account == nil {
			continue
		}
		result = append(result, account)
	}
	return result
}

func (c *PersonalWeixinMultiAccountChannel) snapshotAccountMap() map[string]*PersonalWeixinChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]*PersonalWeixinChannel, len(c.accounts))
	for key, account := range c.accounts {
		key = strings.TrimSpace(key)
		if key == "" || account == nil {
			continue
		}
		result[key] = account
	}
	return result
}

func personalWeixinAccountsByKey(channel channelcontract.DeliveryChannel) map[string]*PersonalWeixinChannel {
	switch typed := channel.(type) {
	case *PersonalWeixinChannel:
		key := personalWeixinAccountKey(typed)
		if key == "" {
			return nil
		}
		return map[string]*PersonalWeixinChannel{key: typed}
	case *PersonalWeixinMultiAccountChannel:
		return typed.snapshotAccountMap()
	default:
		return nil
	}
}

func personalWeixinAccountKey(account *PersonalWeixinChannel) string {
	if account == nil {
		return ""
	}
	if key := strings.TrimSpace(account.accountID); key != "" {
		return key
	}
	return strings.TrimSpace(account.userID)
}
