// INPUT: authoritative queue location plus the normalized queued item.
// OUTPUT: canonical, payload-bound admission identity and a single-use claim.
// POS: queue admission trust model; workspace JSON fields never become authority alone.
package queueadmission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	StatusPending  = "pending"
	StatusClaimed  = "claimed"
	StatusConsumed = "consumed"
	StatusRevoked  = "revoked"
)

// Binding is the complete server-side identity of a queue admission.
type Binding struct {
	OwnerUserID     string
	Scope           protocol.InputQueueScope
	QueueItemID     string
	AgentID         string
	SessionKey      string
	RoomID          string
	ConversationID  string
	SourceMessageID string
	TargetAgentIDs  []string
	PayloadDigest   string
}

// PrincipalBinding is the host-auth database identity of the human who
// submitted a direct WebSocket input. SessionID is an opaque database record
// ID, never a browser cookie, bearer token, password, or desktop credential.
type PrincipalBinding struct {
	UserID     string
	AuthMethod string
	SessionID  string
}

// Admission adds the authenticated human principal that originally submitted
// the direct WebSocket input. It must be freshly revalidated before a
// destructive configuration change; no role is durably delegated here.
type Admission struct {
	Binding
	Principal PrincipalBinding
}

// Claim is an opaque one-time lease plus the host-auth binding loaded from the
// database trust root. Only its creator can release or consume it.
type Claim struct {
	Binding
	Principal PrincipalBinding
	Token     string
}

type digestPayload struct {
	Source         protocol.InputQueueSource   `json:"source"`
	SourceAgentID  string                      `json:"source_agent_id,omitempty"`
	HandoffID      string                      `json:"handoff_id,omitempty"`
	Content        string                      `json:"content"`
	Attachments    []protocol.ChatAttachment   `json:"attachments,omitempty"`
	TargetAgentIDs []string                    `json:"target_agent_ids,omitempty"`
	DeliveryPolicy protocol.ChatDeliveryPolicy `json:"delivery_policy"`
	ReplyRoute     protocol.RoomReplyRoute     `json:"reply_route,omitempty"`
	RootRoundID    string                      `json:"root_round_id,omitempty"`
	HopIndex       int                         `json:"hop_index,omitempty"`
	ExpiresAt      int64                       `json:"expires_at,omitempty"`
}

// NewBinding projects an already resolved physical queue location and normalized
// item into the DB trust identity. Callers must separately reject item/location
// mismatches before invoking it.
func NewBinding(
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) (Binding, error) {
	scope, err := canonicalAdmissionScope(location.Scope)
	if err != nil {
		return Binding{}, err
	}
	if item.Source != protocol.InputQueueSourceUser {
		return Binding{}, errors.New("queue admission requires a direct user source")
	}
	targetAgentIDs := normalizedTargets(item.AgentID, item.TargetAgentIDs)
	hopIndex := item.HopIndex
	if hopIndex < 0 {
		hopIndex = 0
	}
	payload := digestPayload{
		Source:         protocol.NormalizeInputQueueSource(string(item.Source)),
		SourceAgentID:  strings.TrimSpace(item.SourceAgentID),
		HandoffID:      strings.TrimSpace(item.HandoffID),
		Content:        strings.TrimSpace(item.Content),
		Attachments:    protocol.NormalizeChatAttachments(item.Attachments, item.AgentID),
		TargetAgentIDs: targetAgentIDs,
		DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		ReplyRoute:     item.ReplyRoute,
		RootRoundID:    strings.TrimSpace(item.RootRoundID),
		HopIndex:       hopIndex,
		ExpiresAt:      item.ExpiresAt,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Binding{}, err
	}
	sum := sha256.Sum256(encoded)
	binding := Binding{
		OwnerUserID:     strings.TrimSpace(location.OwnerUserID),
		Scope:           scope,
		QueueItemID:     strings.TrimSpace(item.ID),
		AgentID:         strings.TrimSpace(item.AgentID),
		SessionKey:      strings.TrimSpace(location.SessionKey),
		RoomID:          strings.TrimSpace(location.RoomID),
		ConversationID:  strings.TrimSpace(location.ConversationID),
		SourceMessageID: strings.TrimSpace(item.SourceMessageID),
		TargetAgentIDs:  targetAgentIDs,
		PayloadDigest:   hex.EncodeToString(sum[:]),
	}
	if err = binding.Validate(); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

// Validate rejects incomplete or non-user queue identities.
func (b Binding) Validate() error {
	scope, err := canonicalAdmissionScope(b.Scope)
	if err != nil {
		return err
	}
	if strings.TrimSpace(b.OwnerUserID) == "" {
		return errors.New("queue admission owner_user_id is required")
	}
	if strings.TrimSpace(b.QueueItemID) == "" {
		return errors.New("queue admission queue_item_id is required")
	}
	if strings.TrimSpace(b.AgentID) == "" {
		return errors.New("queue admission agent_id is required")
	}
	if strings.TrimSpace(b.SessionKey) == "" {
		return errors.New("queue admission session_key is required")
	}
	if strings.TrimSpace(b.PayloadDigest) == "" {
		return errors.New("queue admission payload_digest is required")
	}
	if scope == protocol.InputQueueScopeRoom {
		if strings.TrimSpace(b.RoomID) == "" || strings.TrimSpace(b.ConversationID) == "" {
			return errors.New("room queue admission requires room_id and conversation_id")
		}
	}
	return nil
}

func canonicalAdmissionScope(value protocol.InputQueueScope) (protocol.InputQueueScope, error) {
	scope := protocol.InputQueueScope(strings.ToLower(strings.TrimSpace(string(value))))
	switch scope {
	case protocol.InputQueueScopeDM, protocol.InputQueueScopeRoom:
		return scope, nil
	default:
		return "", errors.New("queue admission scope is invalid")
	}
}

func normalizedTargets(agentID string, values []string) []string {
	result := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	for _, value := range append([]string{agentID}, values...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
