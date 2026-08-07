// INPUT: trusted owner-main DM identity and one server-created Channel login.
// OUTPUT: durable flow, exact access binding, and secret-free terminal audit models.
// POS: Channel authorization storage contract; model/tool input never supplies these bindings.
package channelauthorization

import (
	"errors"
	"strings"
	"time"
)

const (
	StatusStarting           = "starting"
	StatusRunning            = "running"
	StatusVerifyCodeRequired = "verify_code_required"
	StatusSucceeded          = "succeeded"
	StatusError              = "error"
	StatusExpired            = "expired"
	StatusCancelled          = "cancelled"
	StatusRestartInvalidated = "restart_invalidated"
)

var (
	ErrFlowNotFound = errors.New("channel authorization flow not found")
	ErrFlowConflict = errors.New("channel authorization flow is no longer current")
	ErrActiveFlow   = errors.New("another channel authorization flow is already active")
)

// Binding is the complete server-side identity fixed at flow creation.
type Binding struct {
	OwnerUserID            string
	PrincipalUserID        string
	PrincipalRole          string
	PrincipalAuthMethod    string
	PrincipalAuthSessionID string
	AgentID                string
	BusinessSessionKey     string
	RootRoundID            string
	RuntimeLeaseSessionKey string
	RuntimeLeaseRoundID    string
	ChannelType            string
	AccountBinding         string
}

// Flow is the durable state machine. Encrypted fields are never projected to
// MCP results or audit records.
type Flow struct {
	Binding
	FlowID                     string
	ResolvedAccountID          string
	StartControlVersion        int64
	CommittedControlVersion    int64
	FlowGeneration             string
	ProcessGeneration          string
	Status                     string
	RuntimeRefEncrypted        string
	HumanPresentationEncrypted string
	OutcomeCode                string
	OutcomeMessage             string
	ExpiresAt                  time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	FinishedAt                 *time.Time
}

// TerminalUpdate contains only safe terminal metadata. Finish always scrubs
// runtime and presentation ciphertext in the same transaction.
type TerminalUpdate struct {
	Status                  string
	ResolvedAccountID       string
	CommittedControlVersion int64
	OutcomeCode             string
	OutcomeMessage          string
	FinishedAt              time.Time
	AuditID                 string
}

// Audit is the immutable completion record. It intentionally has no QR,
// device-code, runtime-ref, verification-code, or credential columns.
type Audit struct {
	Binding
	AuditID                 string
	FlowID                  string
	ResolvedAccountID       string
	StartControlVersion     int64
	CommittedControlVersion int64
	FlowGeneration          string
	Status                  string
	OutcomeCode             string
	OutcomeMessage          string
	CreatedAt               time.Time
	CompletedAt             time.Time
}

func (b Binding) Validate() error {
	required := map[string]string{
		"owner_user_id":             b.OwnerUserID,
		"principal_user_id":         b.PrincipalUserID,
		"agent_id":                  b.AgentID,
		"business_session_key":      b.BusinessSessionKey,
		"root_round_id":             b.RootRoundID,
		"runtime_lease_session_key": b.RuntimeLeaseSessionKey,
		"runtime_lease_round_id":    b.RuntimeLeaseRoundID,
		"channel_type":              b.ChannelType,
		"account_binding":           b.AccountBinding,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return errors.New("channel authorization " + name + " is required")
		}
	}
	if strings.TrimSpace(b.OwnerUserID) != strings.TrimSpace(b.PrincipalUserID) {
		return errors.New("channel authorization principal must match owner scope")
	}
	switch strings.TrimSpace(b.PrincipalAuthMethod) {
	case "password":
		if strings.TrimSpace(b.PrincipalAuthSessionID) == "" {
			return errors.New("channel authorization password session is required")
		}
	case "local":
		if strings.TrimSpace(b.PrincipalAuthSessionID) != "" {
			return errors.New("channel authorization local principal cannot carry a password session")
		}
	default:
		return errors.New("channel authorization requires an interactive human authentication method")
	}
	return nil
}

func IsActiveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case StatusStarting, StatusRunning, StatusVerifyCodeRequired:
		return true
	default:
		return false
	}
}

func IsTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case StatusSucceeded, StatusError, StatusExpired, StatusCancelled, StatusRestartInvalidated:
		return true
	default:
		return false
	}
}
