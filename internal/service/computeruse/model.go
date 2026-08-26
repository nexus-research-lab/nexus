// INPUT: pinned release metadata, local package state, native doctor output, and process state.
// OUTPUT: closed host-facing Computer Use status and installation contracts.
// POS: HTTP settings, package manager, supervisor, and runtime command share these types.
package computeruse

import (
	"context"
	"errors"
	"time"
)

const (
	ManifestSchemaVersion = 1
	InstallSchemaVersion  = 1
	ProtocolVersion       = "nexus.cua.v1"
)

var (
	ErrUnavailable       = errors.New("Computer Use host capability is unavailable")
	ErrDisabled          = errors.New("Computer Use is not enabled for this user")
	ErrNotInstalled      = errors.New("Nexus Computer Use Runtime is not installed")
	ErrExternallyManaged = errors.New("Nexus Computer Use Runtime is externally managed")
	ErrProtocolMismatch  = errors.New("Nexus Computer Use Runtime protocol mismatch")
	ErrSidecarNotReady   = errors.New("Nexus Computer Use Runtime sidecar is not ready")
	ErrEpochChanged      = errors.New("Nexus Computer Use Runtime sidecar epoch changed")
	ErrRestartThrottled  = errors.New("Nexus Computer Use Runtime restart is temporarily throttled")
)

// PublicErrorMessage returns a stable settings/API message without exposing
// package paths, transport endpoints, process arguments, or native diagnostics.
func PublicErrorMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrUnavailable):
		return ErrUnavailable.Error()
	case errors.Is(err, ErrDisabled):
		return ErrDisabled.Error()
	case errors.Is(err, ErrNotInstalled):
		return ErrNotInstalled.Error()
	case errors.Is(err, ErrExternallyManaged):
		return ErrExternallyManaged.Error()
	case errors.Is(err, ErrProtocolMismatch):
		return ErrProtocolMismatch.Error()
	case errors.Is(err, ErrSidecarNotReady):
		return ErrSidecarNotReady.Error()
	case errors.Is(err, ErrEpochChanged):
		return ErrEpochChanged.Error()
	case errors.Is(err, ErrRestartThrottled):
		return ErrRestartThrottled.Error()
	case errors.Is(err, context.Canceled):
		return "Computer Use operation was canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "Computer Use operation timed out"
	default:
		return "Computer Use operation failed; run doctor and inspect the local diagnostic log"
	}
}

type PackageSource string

const (
	PackageSourceManaged  PackageSource = "managed"
	PackageSourceOverride PackageSource = "environment"
)

// PackageStatus is safe for the authenticated settings surface.
type PackageStatus struct {
	Available       bool          `json:"available"`
	Installed       bool          `json:"installed"`
	Source          PackageSource `json:"source,omitempty"`
	Version         string        `json:"version,omitempty"`
	TargetVersion   string        `json:"target_version,omitempty"`
	ProtocolVersion string        `json:"protocol_version,omitempty"`
	Platform        string        `json:"platform"`
	Architecture    string        `json:"architecture"`
	ExecutablePath  string        `json:"-"`
	CanInstall      bool          `json:"can_install"`
	CanUpdate       bool          `json:"can_update"`
	CanRemove       bool          `json:"can_remove"`
	Message         string        `json:"message,omitempty"`
}

// ReleaseManifest is authenticated by the configured manifest digest before decoding.
type ReleaseManifest struct {
	SchemaVersion   int               `json:"schema_version"`
	Version         string            `json:"version"`
	ProtocolVersion string            `json:"protocol_version"`
	Artifacts       []ReleaseArtifact `json:"artifacts"`
}

type ReleaseArtifact struct {
	OS            string `json:"os"`
	Architecture  string `json:"architecture"`
	URL           string `json:"url"`
	Format        string `json:"format"`
	ArchiveSHA256 string `json:"archive_sha256"`
	BinaryPath    string `json:"binary_path"`
	BinarySHA256  string `json:"binary_sha256"`
}

type installRecord struct {
	SchemaVersion   int           `json:"schema_version"`
	Version         string        `json:"version"`
	ProtocolVersion string        `json:"protocol_version"`
	Platform        string        `json:"platform"`
	Architecture    string        `json:"architecture"`
	BinaryName      string        `json:"binary_name"`
	BinarySHA256    string        `json:"binary_sha256"`
	ArchiveSHA256   string        `json:"archive_sha256"`
	InstalledAt     time.Time     `json:"installed_at"`
	Source          PackageSource `json:"source"`
}

type currentPackage struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"version"`
}

// DoctorReport combines package identity with the runtime's native diagnostic.
type DoctorReport struct {
	Package         PackageStatus     `json:"package"`
	Healthy         bool              `json:"healthy"`
	RuntimeVersion  string            `json:"runtime_version,omitempty"`
	ProtocolVersion string            `json:"protocol_version,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	Capabilities    map[string]any    `json:"capabilities,omitempty"`
	Permissions     map[string]string `json:"permissions,omitempty"`
	Message         string            `json:"message,omitempty"`
}

type SidecarState string

const (
	SidecarStopped  SidecarState = "stopped"
	SidecarStarting SidecarState = "starting"
	SidecarReady    SidecarState = "ready"
	SidecarStopping SidecarState = "stopping"
	SidecarFailed   SidecarState = "failed"
)

type SidecarStatus struct {
	State           SidecarState `json:"state"`
	Epoch           uint64       `json:"epoch"`
	PID             int          `json:"-"`
	Version         string       `json:"version,omitempty"`
	ProtocolVersion string       `json:"protocol_version,omitempty"`
	StartedAt       time.Time    `json:"started_at,omitempty"`
	UnexpectedExits int          `json:"unexpected_exits"`
	LastError       string       `json:"last_error,omitempty"`
	StdoutTail      string       `json:"-"`
	StderrTail      string       `json:"-"`
}

type Status struct {
	Enabled     bool          `json:"enabled"`
	Package     PackageStatus `json:"package"`
	Sidecar     SidecarStatus `json:"sidecar"`
	Doctor      *DoctorReport `json:"doctor,omitempty"`
	NextActions []string      `json:"next_actions,omitempty"`
}
