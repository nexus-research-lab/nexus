// INPUT: 已验证的 Node 令牌与持久 claim/output identity。
// OUTPUT: 只读任务提示、精确领取、续期、失败与完整输出回执。
// POS: 复用现有 Relay HTTP adapter，不承担执行或自动重试。
package relay

import (
	"context"
	"net/http"
	"net/url"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

func (c *Client) PendingAgents(ctx context.Context, token string) (relaycontract.PendingDeliveries, error) {
	var result relaycontract.PendingDeliveries
	err := c.do(ctx, http.MethodGet, "/node/deliveries/pending", nil, token, "", nil, &result)
	return result, err
}

func (c *Client) CancelPendingDelivery(ctx context.Context, token, roomID, id string) (relaycontract.Delivery, error) {
	var result relaycontract.Delivery
	err := c.do(ctx, http.MethodPost, "/rooms/"+url.PathEscape(roomID)+"/deliveries/"+url.PathEscape(id)+"/cancel", nil, token, "", nil, &result)
	return result, err
}

// ClaimDelivery 以持久 claim ID 作幂等键；重放返回同一租约，没有待办时返回 nil。
func (c *Client) ClaimDelivery(ctx context.Context, token, claimID, agentID string) (*relaycontract.Delivery, error) {
	var result struct {
		Delivery *relaycontract.Delivery `json:"delivery"`
	}
	err := c.command(ctx, http.MethodPost, "/node/deliveries/claim", token, claimID, map[string]string{"agent_id": agentID}, &result)
	return result.Delivery, err
}

// RenewDelivery 续租并只共享白名单运行状态；状态为空时 Relay 保留原状态。
func (c *Client) RenewDelivery(ctx context.Context, token, id, leaseID, executionState string) (relaycontract.Delivery, error) {
	input := map[string]string{"lease_id": leaseID}
	if executionState != "" {
		input["execution_state"] = executionState
	}
	return c.settleDelivery(ctx, token, id, "renew", input)
}

// FailDelivery 以可选白名单失败码结束租约。
func (c *Client) FailDelivery(ctx context.Context, token, id, leaseID, failureCode string) (relaycontract.Delivery, error) {
	input := map[string]string{"lease_id": leaseID}
	if failureCode != "" {
		input["failure_code"] = failureCode
	}
	return c.settleDelivery(ctx, token, id, "fail", input)
}

func (c *Client) settleDelivery(ctx context.Context, token, id, action string, input map[string]string) (relaycontract.Delivery, error) {
	var result relaycontract.Delivery
	err := c.do(ctx, http.MethodPost, "/node/deliveries/"+url.PathEscape(id)+"/"+action, nil, token, "", input, &result)
	return result, err
}

// DeliveryOutput 以持久 output ID 作幂等键提交一条完整输出。
func (c *Client) DeliveryOutput(ctx context.Context, token, id, outputID string, input relaycontract.DeliveryOutput) error {
	var result relaycontract.MessageCommit
	return c.command(ctx, http.MethodPost, "/node/deliveries/"+url.PathEscape(id)+"/outputs", token, outputID, input, &result)
}
