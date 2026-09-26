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
func (c *Client) ClaimDelivery(ctx context.Context, token, claimID, agentID string) (*relaycontract.Delivery, error) {
	var result struct {
		Delivery *relaycontract.Delivery `json:"delivery"`
	}
	err := c.do(ctx, http.MethodPost, "/node/deliveries/claim", nil, token, claimID, map[string]string{"agent_id": agentID}, &result)
	return result.Delivery, err
}
func (c *Client) SettleDelivery(ctx context.Context, token, id, leaseID string, failed bool, failureCodes ...string) (relaycontract.Delivery, error) {
	action := "renew"
	if failed {
		action = "fail"
	}
	var result relaycontract.Delivery
	input := map[string]string{"lease_id": leaseID}
	if failed && len(failureCodes) == 1 && failureCodes[0] != "" {
		input["failure_code"] = failureCodes[0]
	}
	err := c.do(ctx, http.MethodPost, "/node/deliveries/"+url.PathEscape(id)+"/"+action, nil, token, "", input, &result)
	return result, err
}
func (c *Client) DeliveryOutput(ctx context.Context, token, id, outputID string, input relaycontract.DeliveryOutput) error {
	var result relaycontract.MessageCommit
	return c.do(ctx, http.MethodPost, "/node/deliveries/"+url.PathEscape(id)+"/outputs", nil, token, outputID, input, &result)
}

// RenewDelivery 仅共享白名单运行状态，省略状态时保留原状态。
func (c *Client) RenewDelivery(ctx context.Context, token, id, leaseID, executionState string) (relaycontract.Delivery, error) {
	var result relaycontract.Delivery
	input := map[string]string{"lease_id": leaseID}
	if executionState != "" {
		input["execution_state"] = executionState
	}
	err := c.do(ctx, http.MethodPost, "/node/deliveries/"+url.PathEscape(id)+"/renew", nil, token, "", input, &result)
	return result, err
}
