// INPUT: Host-owned IM delivery records and verified communication identity.
// OUTPUT: Exact-session feedback without transferring previous round authority.
// POS: IM scenario adapter; existing contact and Room messaging remain independent.
package communication

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

type DeliverySourceQuery struct {
	DeliveryID string
	Query      string
	Offset     int
	Limit      int
}
type DeliverySourceView struct {
	DeliveryID        string         `json:"delivery_id"`
	AgentName         string         `json:"agent_name"`
	SessionTitle      string         `json:"session_title"`
	Content           string         `json:"content"`
	CreatedAt         int64          `json:"created_at"`
	SendState         string         `json:"send_state"`
	CanReply          bool           `json:"can_reply"`
	UnavailableReason string         `json:"unavailable_reason,omitempty"`
	Replies           []ReplyReceipt `json:"replies,omitempty"`
}
type DeliverySources struct {
	Sources        []DeliverySourceView `json:"delivery_sources"`
	CurrentInputID string               `json:"current_input_message_id,omitempty"`
	NextOffset     int                  `json:"next_offset"`
}
type ReplyReceipt struct {
	DeliveryID string `json:"delivery_id"`
	ReplyID    string `json:"reply_id"`
	Status     string `json:"status"`
}

func (s *Service) ListDeliverySources(ctx context.Context, actor Actor, q DeliverySourceQuery) (*DeliverySources, error) {
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit < 1 || q.Limit > 50 || q.Offset < 0 || q.Offset > 10000 || len(q.Query) > 500 {
		return nil, newInputError("invalid delivery query bounds")
	}
	if q.DeliveryID != "" && (q.Query != "" || q.Offset != 0) {
		return nil, newInputError("delivery_id cannot be combined with search or pagination")
	}
	scoped, _, err := s.authorize(ctx, actor)
	if err != nil {
		return nil, err
	}
	if s.im == nil {
		return nil, errors.New("IM 回传未装配")
	}
	if _, err = s.im.inputs.IMDeliveryPairing(scoped, actor.OwnerUserID, actor.AgentID, actor.SessionKey); err != nil {
		return nil, err
	}
	records := []imdelivery.Delivery{}
	if q.DeliveryID != "" {
		d, e := s.im.store.GetDelivery(scoped, actor.OwnerUserID, q.DeliveryID)
		if e != nil {
			return nil, e
		}
		if d.TargetSessionKey != actor.SessionKey || d.TargetAgentID != actor.AgentID {
			return nil, imdelivery.ErrUnavailable
		}
		records = append(records, d)
	} else {
		records, err = s.im.store.List(scoped, actor.OwnerUserID, actor.SessionKey, q.Query, q.Offset, q.Limit)
		if err != nil {
			return nil, err
		}
	}
	result := &DeliverySources{Sources: []DeliverySourceView{}}
	if len(records) == q.Limit && q.DeliveryID == "" {
		result.NextOffset = q.Offset + q.Limit
	}
	if input, e := s.im.inputs.IMDeliveryInput(scoped, actor.OwnerUserID, actor.AgentID, actor.SessionKey, actor.RoundID, actor.InputContent); e == nil {
		result.CurrentInputID = input.ID
	}
	for _, d := range records {
		v := DeliverySourceView{DeliveryID: d.ID, AgentName: d.SourceAgentName, SessionTitle: d.SourceTitle, CreatedAt: d.CreatedAt, SendState: d.State, Content: d.Content, CanReply: true}
		if q.DeliveryID == "" {
			runes := []rune(v.Content)
			if len(runes) > 500 {
				v.Content = string(runes[:500]) + "…"
			}
		}
		if e := s.validateIMReturn(scoped, d); e != nil {
			v.CanReply = false
			v.UnavailableReason = e.Error()
		}
		if q.DeliveryID != "" {
			replies, e := s.im.store.Replies(scoped, actor.OwnerUserID, d.ID)
			if e != nil {
				return nil, e
			}
			for _, r := range replies {
				v.Replies = append(v.Replies, ReplyReceipt{d.ID, r.ID, r.State})
			}
		}
		result.Sources = append(result.Sources, v)
	}
	return result, nil
}
