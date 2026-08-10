// INPUT: configuration-verified owner-main DM identity and safe model/human requests.
// OUTPUT: redacted model views plus an out-of-band human QR/code presentation.
// POS: conversational Channel authorization cross-layer contract.
package channelauthorization

import (
	"time"

	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

type Actor = configurationsvc.Actor

type StartInput struct {
	ChannelType string `json:"channel_type"`
	AccountID   string `json:"account_id,omitempty"`
}

// View is safe for the model transcript. It never contains QR/device payload,
// internal login IDs, verification codes, tokens, credentials, or ciphertext.
type View struct {
	FlowID                  string     `json:"flow_id"`
	ChannelType             string     `json:"channel_type"`
	AccountBinding          string     `json:"account_binding"`
	ResolvedAccountID       string     `json:"resolved_account_id,omitempty"`
	Status                  string     `json:"status"`
	StartControlVersion     int64      `json:"start_control_version"`
	CommittedControlVersion int64      `json:"committed_control_version,omitempty"`
	Generation              string     `json:"generation"`
	HumanActionRequired     bool       `json:"human_action_required"`
	Message                 string     `json:"message"`
	ExpiresAt               time.Time  `json:"expires_at"`
	FinishedAt              *time.Time `json:"finished_at,omitempty"`
}

const (
	PresentationKindQRCode           = "qr_code"
	PresentationKindVerificationCode = "verification_code"
)

// HumanPresentation is delivered only to the authenticated native UI. It must
// never be serialized into an MCP tool result or assistant transcript.
type HumanPresentation struct {
	FlowID                 string    `json:"flow_id"`
	PresentationToken      string    `json:"presentation_token"`
	Kind                   string    `json:"kind"`
	ChannelType            string    `json:"channel_type"`
	AccountBinding         string    `json:"account_binding"`
	QRPayload              string    `json:"qr_payload,omitempty"`
	QRPayloadType          string    `json:"qr_payload_type,omitempty"`
	Prompt                 string    `json:"prompt"`
	PrincipalUserID        string    `json:"-"`
	PrincipalAuthMethod    string    `json:"-"`
	PrincipalAuthSessionID string    `json:"-"`
	AgentID                string    `json:"-"`
	BusinessSessionKey     string    `json:"-"`
	RootRoundID            string    `json:"-"`
	RuntimeLeaseSessionKey string    `json:"-"`
	RuntimeLeaseRoundID    string    `json:"-"`
	ExpiresAt              time.Time `json:"expires_at"`
}

// HumanVerificationCodeSubmission is built from a trusted UI/session route.
// Code must be passed directly to SubmitHumanVerificationCode and never logged,
// persisted, included in audit, or placed in an MCP argument.
type HumanVerificationCodeSubmission struct {
	FlowID                 string
	PresentationToken      string
	OwnerUserID            string
	PrincipalUserID        string
	PrincipalAuthSessionID string
	AgentID                string
	BusinessSessionKey     string
	RootRoundID            string
	RuntimeLeaseSessionKey string
	RuntimeLeaseRoundID    string
	Code                   string
}

type Completion struct {
	FlowID                  string    `json:"flow_id"`
	ChannelType             string    `json:"channel_type"`
	AccountBinding          string    `json:"account_binding"`
	ResolvedAccountID       string    `json:"resolved_account_id,omitempty"`
	Status                  string    `json:"status"`
	OutcomeCode             string    `json:"outcome_code,omitempty"`
	OutcomeMessage          string    `json:"outcome_message,omitempty"`
	StartControlVersion     int64     `json:"start_control_version"`
	CommittedControlVersion int64     `json:"committed_control_version,omitempty"`
	Generation              string    `json:"generation"`
	CompletedAt             time.Time `json:"completed_at"`
}
