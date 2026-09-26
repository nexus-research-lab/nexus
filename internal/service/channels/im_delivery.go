// INPUT: Host-owned origin and an already resolved external IM destination.
// OUTPUT: Durable per-delivery origin, stable projection and physical-send facts.
// POS: IM scenario adapter, independent of Room private messaging semantics.
package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

type imSourceKey struct{}
type imProjectionKey struct{}
type imPairingResolver interface {
	IMDeliveryPairing(context.Context, string, string, string) (string, error)
}

// WithIMDeliverySource is called only after the communications runtime actor is verified.
func WithIMDeliverySource(ctx context.Context, source imdelivery.Source) context.Context {
	return context.WithValue(ctx, imSourceKey{}, source)
}
func (r *Router) SetIMDeliverySupport(store *imdelivery.Repository, grants imPairingResolver) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.imDeliveries = store
	r.imGrants = grants
}

func (r *Router) stageIMDelivery(ctx context.Context, source imdelivery.Source, agent, session, text string) (imdelivery.Delivery, error) {
	if r.imDeliveries == nil || r.imGrants == nil {
		return imdelivery.Delivery{}, errors.New("IM delivery persistence is not configured")
	}
	owner := authctx.OwnerUserID(ctx)
	pairing, err := r.imGrants.IMDeliveryPairing(ctx, owner, agent, session)
	if err != nil {
		return imdelivery.Delivery{}, err
	}
	target, err := r.resolveDeliverySession(ctx, session)
	if err != nil || target == nil {
		return imdelivery.Delivery{}, errors.New("IM target session unavailable")
	}
	d := imdelivery.Delivery{OwnerUserID: owner, Source: source, TargetAgentID: agent, TargetSessionKey: session, TargetCreatedAt: target.CreatedAt.UTC().Format(time.RFC3339Nano), PairingID: pairing, Channel: protocol.ParseSessionKey(session).Channel, Content: text}
	identity := []string{owner, source.AgentID, source.SessionKey, source.RoundID, source.CallID, session}
	if source.Kind == "automation" {
		identity = []string{owner, "automation", source.RunID, session}
	}
	d.ID = imdelivery.StableID("im_delivery_", identity...)
	// A retry uses the first immutable origin snapshot, including its old epoch.
	if old, e := r.imDeliveries.GetDelivery(ctx, owner, d.ID); e == nil {
		if old.ReturnRevoked {
			return d, imdelivery.ErrUnavailable
		}
		if old.Source.Kind != source.Kind || old.Source.AgentID != source.AgentID || old.Source.SessionKey != source.SessionKey || old.Content != text || old.TargetSessionKey != session || old.PairingID != pairing || old.TargetCreatedAt != d.TargetCreatedAt {
			return d, imdelivery.ErrConflict
		}
		return old, nil
	}
	original, err := r.resolveIMSourceSession(ctx, source.SessionKey)
	if err != nil && source.Kind != "automation" {
		return d, err
	}
	if original != nil {
		if original.ChatType != protocol.RoomTypeGroup && original.AgentID != "" && original.AgentID != source.AgentID {
			return d, errors.New("source session agent mismatch")
		}
		d.Source.SessionCreatedAt = original.CreatedAt.UTC().Format(time.RFC3339Nano)
		d.SourceTitle = original.Title
		if original.RoomID != nil {
			d.Source.RoomID = *original.RoomID
		}
		if original.ConversationID != nil {
			d.Source.ConversationID = *original.ConversationID
		}
	} else if source.Kind != "automation" {
		return d, errors.New("source session unavailable")
	}
	if a, e := r.agents.GetAgent(ctx, source.AgentID); e == nil && a != nil {
		if a.OwnerUserID != owner {
			return d, errors.New("source owner mismatch")
		}
		d.SourceAgentName = a.Name
	}
	return r.imDeliveries.SaveDelivery(ctx, d)
}

func (r *Router) deliverTrackedIM(ctx context.Context, source imdelivery.Source, agent, text, session string) (DeliveryResult, error) {
	if source.CallID == "" {
		return DeliveryResult{}, errors.New("IM delivery unavailable: runtime MCP tool-use ID is missing; use a Bridge version that preserves call metadata")
	}
	d, err := r.stageIMDelivery(ctx, source, agent, session, text)
	if err != nil {
		return DeliveryResult{}, err
	}
	result := DeliveryResult{DeliveryID: d.ID, Target: DeliveryTarget{Mode: DeliveryModeLast, SessionKey: session}}
	if d.ReceiptJSON != "" {
		if err = json.Unmarshal([]byte(d.ReceiptJSON), &result); err != nil {
			return result, err
		}
	}
	if d.State == "sent" {
		return result, nil
	}
	claimed, err := r.imDeliveries.ClaimSend(ctx, d.OwnerUserID, d.ID, false)
	if err != nil {
		return result, err
	}
	if !claimed {
		return result, fmt.Errorf("投递 %s 的发送结果待核对，不自动重发", d.ID)
	}
	projector := r.sessionProjector(ctx, agent, ChannelTypeWebSocket)
	if projector == nil {
		_ = r.imDeliveries.FinishSend(ctx, d.OwnerUserID, d.ID, "not_sent", "")
		return result, errors.New("IM projector unavailable")
	}
	projectionCtx := context.WithValue(ctx, imProjectionKey{}, d)
	if _, err = projector.SendAgentDeliveryMessage(projectionCtx, agent, result.Target, text); err != nil {
		finishErr := r.imDeliveries.FinishSend(ctx, d.OwnerUserID, d.ID, "not_sent", "")
		return result, errors.Join(err, finishErr)
	}
	result, err = r.DeliverMessage(r.withIMDeliveryProgress(ctx, d), agent, text, result.Target)
	result.DeliveryID = d.ID
	raw, _ := json.Marshal(result)
	if err != nil {
		// 保留部分成功的回执，未知分段仍禁止自动重发。
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		persistErr := r.imDeliveries.FinishSend(persistCtx, d.OwnerUserID, d.ID, "unknown", string(raw))
		return result, fmt.Errorf("投递 %s 结果待核对: %w", d.ID, errors.Join(err, persistErr))
	}
	if err = r.imDeliveries.FinishSend(ctx, d.OwnerUserID, d.ID, "sent", string(raw)); err != nil {
		return result, err
	}
	return result, nil
}

func imProjectionMetadata(ctx context.Context) (imdelivery.Delivery, bool) {
	d, ok := ctx.Value(imProjectionKey{}).(imdelivery.Delivery)
	return d, ok
}

// A producer may be a hidden contact/automation Session. Directory visibility
// restricts external destinations, not a verified producer's return identity.
func (r *Router) resolveIMSourceSession(ctx context.Context, key string) (*protocol.Session, error) {
	if resolver, ok := r.sessions.(interface {
		GetSession(context.Context, string) (*protocol.Session, error)
	}); ok {
		return resolver.GetSession(ctx, key)
	}
	return r.resolveDeliverySession(ctx, key)
}

// withIMDeliveryProgress 复用现有回执列保存每段进度；持久化失败就停止后续发送。
func (r *Router) withIMDeliveryProgress(ctx context.Context, d imdelivery.Delivery) context.Context {
	return channelcontract.WithDeliveryProgress(ctx, func(result DeliveryResult) error {
		result.DeliveryID = d.ID
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return r.imDeliveries.FinishSend(persistCtx, d.OwnerUserID, d.ID, "unknown", string(raw))
	})
}
