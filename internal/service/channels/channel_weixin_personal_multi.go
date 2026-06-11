package channels

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type personalWeixinMultiAccountChannel struct {
	mu       sync.RWMutex
	accounts map[string]*personalWeixinChannel
}

func newPersonalWeixinMultiAccountChannel(accounts []*personalWeixinChannel) *personalWeixinMultiAccountChannel {
	result := &personalWeixinMultiAccountChannel{
		accounts: make(map[string]*personalWeixinChannel, len(accounts)),
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		key := strings.TrimSpace(account.accountID)
		if key == "" {
			key = strings.TrimSpace(account.userID)
		}
		if key == "" {
			key = fmt.Sprintf("account-%d", len(result.accounts)+1)
		}
		result.accounts[key] = account
	}
	return result
}

func (c *personalWeixinMultiAccountChannel) ChannelType() string {
	return ChannelTypeWeixinPersonal
}

func (c *personalWeixinMultiAccountChannel) Start(ctx context.Context) error {
	for _, account := range c.snapshotAccounts() {
		if err := account.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *personalWeixinMultiAccountChannel) Stop(ctx context.Context) error {
	for _, account := range c.snapshotAccounts() {
		_ = account.Stop(ctx)
	}
	return nil
}

func (c *personalWeixinMultiAccountChannel) SetIngress(ingress IngressAcceptor) {
	for _, account := range c.snapshotAccounts() {
		account.SetIngress(ingress)
	}
}

func (c *personalWeixinMultiAccountChannel) SendDeliveryMessage(
	ctx context.Context,
	target DeliveryTarget,
	text string,
) (DeliveryResult, error) {
	account, err := c.accountForTarget(target)
	if err != nil {
		return DeliveryResult{}, err
	}
	return account.SendDeliveryMessage(ctx, target, text)
}

func (c *personalWeixinMultiAccountChannel) SendDeliveryTyping(ctx context.Context, target DeliveryTarget, active bool) error {
	account, err := c.accountForTarget(target)
	if err != nil {
		return err
	}
	return account.SendDeliveryTyping(ctx, target, active)
}

func (c *personalWeixinMultiAccountChannel) accountForTarget(target DeliveryTarget) (*personalWeixinChannel, error) {
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

func (c *personalWeixinMultiAccountChannel) snapshotAccounts() []*personalWeixinChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*personalWeixinChannel, 0, len(c.accounts))
	for _, account := range c.accounts {
		if account == nil {
			continue
		}
		result = append(result, account)
	}
	return result
}
