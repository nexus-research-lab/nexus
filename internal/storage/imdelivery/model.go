// INPUT: Host-verified IM delivery origins and human feedback identities.
// OUTPUT: Immutable, owner-scoped delivery and reply facts.
// POS: IM scenario persistence model; independent of Room private reply routes.
package imdelivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Source is a host-owned return address, never a model-supplied destination.
type Source struct {
	Kind             string `json:"kind"`
	AgentID          string `json:"agent_id"`
	SessionKey       string `json:"session_key"`
	SessionCreatedAt string `json:"session_created_at"`
	RoundID          string `json:"round_id"`
	CallID           string `json:"call_id"`
	RoomID           string `json:"room_id,omitempty"`
	ConversationID   string `json:"conversation_id,omitempty"`
	JobID            string `json:"job_id,omitempty"`
	RunID            string `json:"run_id,omitempty"`
}

type Delivery struct {
	ReturnRevoked    bool   `json:"return_revoked"`
	ID               string `json:"delivery_id"`
	OwnerUserID      string `json:"owner_user_id"`
	Source           Source `json:"source"`
	SourceTitle      string `json:"source_title"`
	SourceAgentName  string `json:"source_agent_name"`
	TargetAgentID    string `json:"target_agent_id"`
	TargetSessionKey string `json:"target_session_key"`
	TargetCreatedAt  string `json:"target_created_at"`
	PairingID        string `json:"pairing_id"`
	Channel          string `json:"channel"`
	Content          string `json:"content"`
	CreatedAt        int64  `json:"created_at"`
	State            string `json:"send_state"`
	ReceiptJSON      string `json:"receipt_json"`
}

// Input is saved by paired IM ingress before dispatch, outside workspace history.
type Input struct {
	ID          string `json:"message_id"`
	OwnerUserID string `json:"owner_user_id"`
	AgentID     string `json:"agent_id"`
	SessionKey  string `json:"session_key"`
	RoundID     string `json:"round_id"`
	PairingID   string `json:"pairing_id"`
	Content     string `json:"content"`
	Sender      string `json:"sender"`
}

type Reply struct {
	ID               string   `json:"reply_id"`
	OwnerUserID      string   `json:"owner_user_id"`
	DeliveryID       string   `json:"delivery_id"`
	Input            Input    `json:"input"`
	ContentSources   []string `json:"content_source_message_ids"`
	Content          string   `json:"content"`
	ForwardedContent string   `json:"forwarded_content"`
	State            string   `json:"admission_state"`
	CreatedAt        int64    `json:"created_at"`
}

func StableID(prefix string, values ...string) string {
	raw, _ := json.Marshal(values)
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:20])
}
