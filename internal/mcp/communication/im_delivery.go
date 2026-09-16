// INPUT: Closed list_targets query and send_message delivery_source arguments.
// OUTPUT: Existing-tool IM scenario dispatch with no model-controlled return address.
// POS: IM MCP adapter; private messaging keeps its own parameter contract.
package communication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
)

func listTargetsTool(svc *communicationsvc.Service, actor communicationsvc.Actor) sdktool.Tool {
	return sdktool.Tool{Name: "list_targets", AlwaysLoad: true, Description: "无参数列出好友、群与外部私聊。当前 IM 人类反馈需回传原任务时，使用 scope=delivery_sources 查询收到的投递来源；按内容消歧，不能只按最新一条猜测。delivery_id 查看单条及回传接受状态。", SearchHint: "Nexus 通讯目标 IM 回传 来源 联系人 群 list targets delivery sources", Annotations: &sdktool.ToolAnnotations{ReadOnly: true}, InputSchema: objectSchema(map[string]any{
		"scope":       map[string]any{"type": "string", "enum": []string{"address_book", "delivery_sources"}},
		"delivery_id": map[string]any{"type": "string", "minLength": 1},
		"query":       map[string]any{"type": "string", "maxLength": 500},
		"offset":      map[string]any{"type": "integer", "minimum": 0, "maximum": 10000},
		"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
	}, nil), Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
		if svc == nil {
			return errorResult(errors.New("平台通讯服务未装配")), nil
		}
		q, lookup, err := parseDeliveryQuery(args)
		if err != nil {
			return errorResult(err), nil
		}
		if lookup {
			v, e := svc.ListDeliverySources(ctx, actor, q)
			if e != nil {
				return errorResult(e), nil
			}
			return jsonResult(v), nil
		}
		v, e := svc.ListAddressBook(ctx, actor)
		if e != nil {
			return errorResult(e), nil
		}
		return jsonResult(v), nil
	}}
}
func parseDeliveryQuery(args map[string]any) (communicationsvc.DeliverySourceQuery, bool, error) {
	q := communicationsvc.DeliverySourceQuery{}
	if err := allowOnly(args, "scope", "delivery_id", "query", "offset", "limit"); err != nil {
		return q, false, err
	}
	for _, key := range []string{"scope", "delivery_id", "query"} {
		if value, ok := args[key]; ok {
			if _, ok = value.(string); !ok {
				return q, false, fmt.Errorf("%s must be a string", key)
			}
		}
	}
	scope := stringArg(args, "scope")
	if _, present := args["scope"]; present && scope == "" {
		return q, false, errors.New("empty target scope")
	}
	if scope == "" || scope == "address_book" {
		if err := allowOnly(args, "scope"); err != nil {
			return q, false, err
		}
		return q, false, nil
	}
	if scope != "delivery_sources" {
		return q, false, errors.New("unknown target scope")
	}
	q.DeliveryID = stringArg(args, "delivery_id")
	q.Query = stringArg(args, "query")
	if _, present := args["delivery_id"]; present && q.DeliveryID == "" {
		return q, true, errors.New("empty delivery_id")
	}
	if len(q.Query) > 500 {
		return q, true, errors.New("query too long")
	}
	if q.DeliveryID != "" {
		if err := allowOnly(args, "scope", "delivery_id"); err != nil {
			return q, true, err
		}
	}
	var err error
	q.Offset, err = boundedInteger(args, "offset", 0, 0, 10000)
	if err != nil {
		return q, true, err
	}
	q.Limit, err = boundedInteger(args, "limit", 10, 1, 50)
	return q, true, err
}
func boundedInteger(args map[string]any, key string, fallback, min, max int) (int, error) {
	value, ok := args[key]
	if !ok {
		return fallback, nil
	}
	n := 0
	switch v := value.(type) {
	case int:
		n = v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < float64(min) || v > float64(max) {
			return 0, fmt.Errorf("invalid %s", key)
		}
		n = int(v)
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}
func sendDeliveryReply(ctx context.Context, svc *communicationsvc.Service, actor communicationsvc.Actor, args map[string]any) (any, error) {
	if err := allowOnly(args, "destination", "target_id", "content", "content_source_message_ids"); err != nil {
		return nil, err
	}
	for _, key := range []string{"target_id", "content"} {
		if _, ok := args[key].(string); !ok {
			return nil, fmt.Errorf("%s must be a string", key)
		}
	}
	refs := []string{}
	if raw, ok := args["content_source_message_ids"]; ok {
		values, ok := raw.([]any)
		if !ok {
			return nil, errors.New("content_source_message_ids must be an array")
		}
		if len(values) == 0 || len(values) > 10 {
			return nil, errors.New("invalid content source count")
		}
		for _, v := range values {
			text, ok := v.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return nil, errors.New("invalid content message id")
			}
			refs = append(refs, text)
		}
	}
	if svc == nil {
		return nil, errors.New("平台通讯服务未装配")
	}
	return svc.ReplyToDelivery(ctx, actor, stringArg(args, "target_id"), stringArg(args, "content"), refs)
}

// imDeliveryCallID follows the SDK's optional tool-use identity contract. The
// fallback binds a canonical intent to this host-owned physical round; identical
// retries reuse the durable send receipt, while a new round has a fresh identity.
func imDeliveryCallID(sctx RuntimeContext, call *sdktool.CallContext, input map[string]any) (string, error) {
	actor := sctx.Actor
	if strings.TrimSpace(actor.SessionKey) == "" || strings.TrimSpace(actor.RoundID) == "" || strings.TrimSpace(actor.AgentID) == "" || strings.TrimSpace(actor.OwnerUserID) == "" {
		return "", errors.New("IM send requires a host-bound session and round")
	}
	if call != nil && strings.TrimSpace(call.ToolUseID) != "" {
		return strings.TrimSpace(call.ToolUseID), nil
	}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("canonicalize IM delivery input: %w", err)
	}
	parts := []string{actor.OwnerUserID, actor.AgentID, actor.SessionKey, actor.RoundID, sctx.CurrentAgentRoundID, "send_message", string(canonical)}
	raw, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "im-call-" + hex.EncodeToString(digest[:]), nil
}
