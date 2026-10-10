// INPUT: Host-owned runtime identity and a session/round scope.
// OUTPUT: A private, versioned scratch lease, refusing persisted cleanup failure.
// POS: The desktop host owns scratch creation and cleanup; the SDK/Bridge only
// receives the resulting SandboxResourcePolicy.
package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	policyVersion       = 1
	scratchDirName      = "sandbox"
	leaseMarkerName     = ".nexus-sandbox-lease.json"
	leaseMarkerVersion  = 1
	cleanupStateActive  = "active"
	cleanupStateUnknown = "cleanup_unknown"
)

var (
	// Serialize marker inspection/publication within this host. An overlapping
	// Acquire must not mistake a not-yet-published marker for crashed state.
	acquisitionGate = make(chan struct{}, 1)
	registryMu      sync.Mutex
	registry        = map[string]*sandboxResource{}
	byScope         = map[string]*sandboxResource{}
)

// Input identifies the host-owned scope for one runtime process.
type Input struct {
	OwnerUserID string
	SessionKey  string
	RoundID     string
	WriteScope  agentclient.SandboxWriteScope
	// Root is test-only/embedding override. Production callers leave it empty,
	// which places scratch beneath the canonical owner runtime root.
	Root string
}

// sandboxResource is the shared filesystem object for one owner/session scope.
// Every Acquire call returns a separate Lease handle, even when the resource is
// reused. A resource is removed only after its last handle is released.
type sandboxResource struct {
	mu                sync.Mutex
	path              string
	root              string
	scopeKey          string
	policy            agentclient.SandboxResourcePolicy
	marker            SandboxLeaseMarker
	base              *confinedfs.Root
	baseIdentity      os.FileInfo
	leafIdentity      os.FileInfo
	refs              int
	handles           map[*Lease]struct{}
	closed            bool
	cleanupErr        error
	uncertainLease    *Lease
	cleanupSupervisor *SandboxProcessSupervisor
	supervisedCleanup func(*sandboxResource) error
}

// Lease owns one reference to a shared scratch directory. Release is safe to
// call more than once; a failed final cleanup keeps the reference and resource
// registered so the host can retry it without allowing a replacement runtime.
type Lease struct {
	mu       sync.Mutex
	resource *sandboxResource
	// roundID belongs to this exact handle. A resource is shared by all rounds
	// of one owner/session, so the durable marker's RoundID is only the first
	// creation round and must not be reused as the current runtime identity.
	roundID  string
	released bool
}

// active reports whether this exact handle still owns an unreleased reference.
// A released handle intentionally retains its immutable resource metadata for
// diagnostics, but it must never be rebound to a new runtime generation.
func (l *Lease) active() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.released && l.resource != nil
}

func leasesShareResource(left, right *Lease) bool {
	if left == nil || right == nil {
		return left == right
	}
	left.mu.Lock()
	leftResource := left.resource
	left.mu.Unlock()
	right.mu.Lock()
	rightResource := right.resource
	right.mu.Unlock()
	return leftResource != nil && leftResource == rightResource
}

// SandboxLeaseMarker is the durable identity left beside a scratch lease.
// It intentionally records only scope and process metadata; it is not an
// execution receipt and does not authorize another process to adopt the lease.
type SandboxLeaseMarker struct {
	Version     int    `json:"version"`
	LeaseID     string `json:"lease_id"`
	OwnerUserID string `json:"owner_user_id"`
	SessionKey  string `json:"session_key"`
	RoundID     string `json:"round_id,omitempty"`
	RuntimeRoot string `json:"runtime_root"`
	ProcessID   int    `json:"process_id"`
	// ProcessStartTimeUnixNano disambiguates a reused PID on platforms that
	// can query the native process creation time (currently Windows). Zero is
	// retained for older markers and platforms without a safe identity probe;
	// those markers continue through the conservative liveness path below.
	ProcessStartTimeUnixNano int64     `json:"process_start_time_unix_nano,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	// CleanupState is durable so a host restart can distinguish an ordinary
	// active marker from a runtime whose close/cleanup was not proven. Empty is
	// treated as active for markers written by older versions.
	CleanupState     string    `json:"cleanup_state,omitempty"`
	CleanupError     string    `json:"cleanup_error,omitempty"`
	CleanupUpdatedAt time.Time `json:"cleanup_updated_at,omitempty"`
}

// SandboxResourceSweepInput is an explicit, owner-scoped recovery request.
// OlderThan must be positive. Apply=false performs a dry run; only a caller
// that deliberately sets Apply=true may remove dead, expired leases.
type SandboxResourceSweepInput struct {
	OwnerUserID string
	Root        string
	OlderThan   time.Duration
	Now         time.Time
	Apply       bool
}

// SandboxResourceRecord describes a marker discovered beneath one owner's
// canonical runtime root. Records with a live process or an active in-memory
// lease are never eligible for removal.
type SandboxResourceRecord struct {
	Path          string
	Marker        SandboxLeaseMarker
	Age           time.Duration
	ProcessActive bool
	leafIdentity  os.FileInfo
}

// SandboxResourceSweepResult preserves the dry-run candidates and records
// skipped because they are still active. Removal is never automatic at
// startup; callers must issue this explicit recovery operation.
type SandboxResourceSweepResult struct {
	Candidates []SandboxResourceRecord
	Removed    []SandboxResourceRecord
	Skipped    []SandboxResourceRecord
}

func ownerRuntimeRoot(owner string) string {
	stateRoot := strings.TrimSpace(os.Getenv("NEXUS_STATE_ROOT"))
	if stateRoot == "" {
		stateRoot = strings.TrimSpace(os.Getenv("NEXUS_CONFIG_DIR"))
	}
	if stateRoot == "" {
		if home, err := os.UserHomeDir(); err == nil {
			stateRoot = filepath.Join(home, ".nexus")
		} else {
			stateRoot = filepath.Join(".", ".nexus")
		}
	}
	if strings.HasPrefix(stateRoot, "~/") || stateRoot == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			stateRoot = filepath.Join(home, strings.TrimPrefix(stateRoot, "~/"))
		}
	}
	stateRoot = filepath.Clean(stateRoot)
	if (filepath.Base(stateRoot) == "app" || filepath.Base(stateRoot) == "config") && filepath.Base(filepath.Dir(stateRoot)) == ".nexus" {
		stateRoot = filepath.Dir(stateRoot)
	}
	return filepath.Join(stateRoot, "users", safeOwnerPathSegment(owner), "runtime")
}

func safeOwnerPathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "__system__"
	}
	var builder strings.Builder
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9', character == '-', character == '_', character == '.', character == '@':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	sanitized := builder.String()
	if sanitized == "" || sanitized == "." || sanitized == ".." || sanitized != trimmed || strings.HasSuffix(sanitized, ".") {
		sum := sha256.Sum256([]byte(trimmed))
		return sanitized + "-" + hex.EncodeToString(sum[:4])
	}
	return sanitized
}

// SandboxResourceInput is the public host input used by DM, Room and background runtimes.
type SandboxResourceInput = Input

// SandboxResourceLease is the public host-owned scratch lease.
type SandboxResourceLease = Lease

// Acquire creates a private scratch directory before SDK initialize. It
// canonicalizes the runtime root, fixes its directory handle before creating
// children, and never follows an existing symlink at the created leaf.
func Acquire(ctx context.Context, input Input) (*Lease, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case acquisitionGate <- struct{}{}:
		defer func() { <-acquisitionGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	owner := strings.TrimSpace(input.OwnerUserID)
	session := strings.TrimSpace(input.SessionKey)
	roundID := strings.TrimSpace(input.RoundID)
	if owner == "" || session == "" {
		return nil, errors.New("sandbox scratch requires owner and session")
	}
	if input.WriteScope == "" {
		input.WriteScope = agentclient.SandboxWriteScopeWorkspaceWrite
	}
	if input.WriteScope != agentclient.SandboxWriteScopeReadOnly && input.WriteScope != agentclient.SandboxWriteScopeWorkspaceWrite {
		return nil, fmt.Errorf("unsupported sandbox write scope %q", input.WriteScope)
	}

	root := strings.TrimSpace(input.Root)
	if root == "" {
		root = ownerRuntimeRoot(owner)
	}
	root, err := absoluteCleanDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("invalid sandbox runtime root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("validate sandbox runtime root: %w", err)
	}
	root = filepath.Clean(resolvedRoot)
	// Keep the runtime root open while creating the scratch parent and leaf.
	// A path-only check followed by os.Mkdir would allow an owner-controlled
	// directory replacement to redirect the lease between validation and use.
	runtimeRootFS, err := openSandboxDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("open sandbox runtime root: %w", err)
	}
	defer runtimeRootFS.Close()
	if err := runtimeRootFS.MkdirAll(scratchDirName, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox scratch parent: %w", err)
	}
	baseFS, err := openSandboxChildDirectory(runtimeRootFS, scratchDirName)
	if err != nil {
		return nil, fmt.Errorf("open sandbox scratch parent: %w", err)
	}
	defer baseFS.Close()
	if err := chmodSandboxDirectory(baseFS); err != nil {
		return nil, fmt.Errorf("lock sandbox scratch parent: %w", err)
	}
	base := filepath.Join(root, scratchDirName)
	confinedRuntimeRoot, err := confinedfs.Open(root)
	if err != nil {
		return nil, fmt.Errorf("open confined sandbox runtime root: %w", err)
	}
	defer confinedRuntimeRoot.Close()
	confinedBase, err := confinedRuntimeRoot.OpenOrCreateRootNoSymlink(scratchDirName, 0o700)
	if err != nil {
		return nil, fmt.Errorf("open confined sandbox scratch parent: %w", err)
	}
	retainConfinedBase := false
	defer func() {
		if !retainConfinedBase {
			_ = confinedBase.Close()
		}
	}()
	if err := confinedBase.ChmodRoot(0o700); err != nil {
		return nil, fmt.Errorf("lock confined sandbox scratch parent: %w", err)
	}
	baseIdentity, err := confinedBase.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("stat confined sandbox scratch parent: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Keep the scope in the host registry for audit/debugging without putting
	// user-controlled identifiers in a filesystem path. One active process
	// session keeps one path so routine round reconfiguration does not force a
	// scratch replacement.
	scopeKey := root + "\x00" + owner + "\x00" + session
	registryMu.Lock()
	existing := byScope[scopeKey]
	registryMu.Unlock()
	if existing != nil {
		existing.mu.Lock()
		active := !existing.closed
		existingScope := existing.policy.WriteScope
		sameWriteScope := existingScope == input.WriteScope
		cleanupErr := existing.cleanupErr
		if active && cleanupErr != nil {
			existing.mu.Unlock()
			return nil, fmt.Errorf("sandbox session cleanup is pending: %w", cleanupErr)
		}
		if active && sameWriteScope {
			handle := &Lease{resource: existing, roundID: roundID}
			existing.refs++
			if existing.handles == nil {
				existing.handles = make(map[*Lease]struct{})
			}
			existing.handles[handle] = struct{}{}
			existing.mu.Unlock()
			return handle, nil
		}
		existing.mu.Unlock()
		if active {
			if !sameWriteScope {
				return nil, fmt.Errorf("sandbox session already owns a %s lease; cannot replace it with %s", existingScope, input.WriteScope)
			}
		}
	}

	digest := sha256.Sum256([]byte(scopeKey))
	name := ".scratch-" + hex.EncodeToString(digest[:])[:16]
	if err := checkSandboxScratchAdmission(ctx, confinedBase, root, owner, session, name); err != nil {
		return nil, err
	}
	if err := baseFS.Mkdir(name, 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create sandbox scratch: %w", err)
		}
		// A directory left by a crashed host is never silently adopted. Keep a
		// unique replacement and let recovery tooling decide when to sweep it.
		staleName, tempErr := mkdirSandboxTemp(baseFS, name+"-stale-")
		if tempErr != nil {
			return nil, fmt.Errorf("create sandbox scratch replacement: %w", tempErr)
		}
		name = filepath.FromSlash(staleName)
	}
	path := filepath.Join(base, name)
	leafFS, err := openSandboxChildDirectory(baseFS, name)
	if err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, fmt.Errorf("validate sandbox scratch: %w", err)
	}
	defer leafFS.Close()
	if err := chmodSandboxDirectory(leafFS); err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, fmt.Errorf("lock sandbox scratch: %w", err)
	}
	info, err := leafFS.Stat(".")
	if err != nil || !info.IsDir() {
		_ = removeOwnedScratch(base, path)
		if err == nil {
			err = errors.New("scratch leaf is not a directory")
		}
		return nil, fmt.Errorf("validate sandbox scratch: %w", err)
	}
	policy := agentclient.SandboxResourcePolicy{
		Version:     policyVersion,
		WriteScope:  input.WriteScope,
		ScratchRoot: path,
	}
	if err := policy.Validate(); err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, fmt.Errorf("validate sandbox resource policy: %w", err)
	}
	marker, err := newSandboxLeaseMarker(owner, session, strings.TrimSpace(input.RoundID), root)
	if err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, fmt.Errorf("create sandbox lease marker: %w", err)
	}
	if err := writeSandboxLeaseMarkerInRoot(leafFS, marker); err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, fmt.Errorf("persist sandbox lease marker: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = removeOwnedScratch(base, path)
		return nil, err
	}
	resource := &sandboxResource{
		path:         path,
		root:         base,
		scopeKey:     scopeKey,
		policy:       policy,
		marker:       marker,
		base:         confinedBase,
		baseIdentity: baseIdentity,
		leafIdentity: info,
		refs:         1,
		handles:      make(map[*Lease]struct{}),
	}
	handle := &Lease{resource: resource, roundID: roundID}
	resource.handles[handle] = struct{}{}
	registryMu.Lock()
	if existing := byScope[scopeKey]; existing != nil {
		registryMu.Unlock()
		_ = removeOwnedScratch(base, path)
		existing.mu.Lock()
		if existing.cleanupErr != nil {
			cleanupErr := existing.cleanupErr
			existing.mu.Unlock()
			return nil, fmt.Errorf("sandbox session cleanup is pending: %w", cleanupErr)
		}
		existingScope := existing.policy.WriteScope
		sameWriteScope := existingScope == input.WriteScope
		if sameWriteScope && !existing.closed {
			handle := &Lease{resource: existing, roundID: roundID}
			existing.refs++
			if existing.handles == nil {
				existing.handles = make(map[*Lease]struct{})
			}
			existing.handles[handle] = struct{}{}
			existing.mu.Unlock()
			return handle, nil
		}
		existing.mu.Unlock()
		if !sameWriteScope {
			return nil, fmt.Errorf("sandbox session already owns a %s lease; cannot replace it with %s", existingScope, input.WriteScope)
		}
		return nil, errors.New("sandbox session resource was closed concurrently")
	}
	registry[path] = resource
	byScope[scopeKey] = resource
	retainConfinedBase = true
	registryMu.Unlock()
	return handle, nil
}

func absoluteCleanDirectory(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("path is empty")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(absolute)
	if clean == "." || !filepath.IsAbs(clean) || filepath.Dir(clean) == clean {
		return "", errors.New("path must be an absolute non-root directory")
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(clean, `\\`) {
		return "", errors.New("UNC runtime roots are not supported")
	}
	return clean, nil
}

// openSandboxDirectory fixes a real directory handle and verifies that the
// handle still refers to the directory observed before opening it.
func openSandboxDirectory(name string) (*confinedfs.Root, error) {
	return confinedfs.Open(name)
}

// openSandboxChildDirectory opens one real child beneath a fixed root and
// rejects a symlink or an inode replacement between Lstat and OpenRoot.
func openSandboxChildDirectory(parent *confinedfs.Root, name string) (*confinedfs.Root, error) {
	if parent == nil || strings.TrimSpace(name) == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return nil, errors.New("invalid sandbox child directory")
	}
	return parent.OpenRootNoSymlink(name)
}

func chmodSandboxDirectory(root *confinedfs.Root) error {
	if root == nil {
		return errors.New("sandbox root is closed")
	}
	return root.ChmodRoot(0o700)
}

func openSandboxRegularFile(root *confinedfs.Root, name string) (*os.File, error) {
	if root == nil || strings.TrimSpace(name) == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return nil, errors.New("invalid sandbox file")
	}
	return root.OpenFileNoSymlink(name, os.O_RDONLY, 0)
}

func mkdirSandboxTemp(parent *confinedfs.Root, prefix string) (string, error) {
	if parent == nil || strings.ContainsAny(prefix, `/\\`+"\x00") {
		return "", errors.New("invalid sandbox temporary directory prefix")
	}
	return parent.MkdirTemp(".", prefix, 0o700)
}

// Resources returns an independent policy copy for SDK options.
func (l *Lease) Resources() *agentclient.SandboxResourcePolicy {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return nil
	}
	resource := l.resource
	l.mu.Unlock()
	if resource == nil {
		return nil
	}
	resource.mu.Lock()
	defer resource.mu.Unlock()
	policy := resource.policy
	return &policy
}

// Marker returns a copy of the durable lease identity for runtime diagnostics.
// It never grants another caller ownership of the lease.
func (l *Lease) Marker() *SandboxLeaseMarker {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return nil
	}
	resource := l.resource
	l.mu.Unlock()
	if resource == nil {
		return nil
	}
	resource.mu.Lock()
	defer resource.mu.Unlock()
	marker := resource.marker
	return &marker
}

// RoundID returns the round identity carried by this exact handle. A shared
// owner/session resource retains one durable marker, while each Acquire call
// may represent a different active round.
func (l *Lease) RoundID() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return ""
	}
	return l.roundID
}

// Path returns the host-owned scratch path.
func (l *Lease) Path() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return ""
	}
	resource := l.resource
	l.mu.Unlock()
	if resource == nil {
		return ""
	}
	resource.mu.Lock()
	defer resource.mu.Unlock()
	return resource.path
}

// MarkCleanupUncertain fences a resource whose owning process did not close
// cleanly. New acquisitions for the scope are rejected until the exact handle
// can be reconciled and released successfully. A non-nil return means the
// durable marker state could not be updated; the in-memory fence remains set.
func (l *Lease) MarkCleanupUncertain(err error) error {
	if l == nil || err == nil {
		return nil
	}
	l.mu.Lock()
	if l.released {
		l.mu.Unlock()
		return nil
	}
	resource := l.resource
	l.mu.Unlock()
	if resource == nil {
		return nil
	}
	var persistErr error
	resource.mu.Lock()
	if !resource.closed {
		resource.cleanupErr = err
		resource.uncertainLease = l
		resource.marker.CleanupState = cleanupStateUnknown
		resource.marker.CleanupError = sandboxCleanupErrorSummary(err)
		resource.marker.CleanupUpdatedAt = time.Now().UTC()
		persistErr = persistSandboxLeaseMarkerStateLocked(resource)
	}
	resource.mu.Unlock()
	return persistErr
}

// Release removes the scratch directory only after the caller's runtime has
// been disconnected. A cleanup error leaves the lease registered and returns
// the error so the session close fence can retain it for recovery.
func (l *Lease) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return nil
	}
	resource := l.resource
	if resource == nil {
		l.released = true
		return nil
	}
	err := releaseSandboxResource(resource, l)
	if err != nil {
		return err
	}
	l.released = true
	return nil
}

func releaseSandboxResource(resource *sandboxResource, handle *Lease) error {
	resource.mu.Lock()
	defer resource.mu.Unlock()
	if resource.closed {
		return nil
	}
	if resource.refs <= 0 {
		return errors.New("sandbox resource reference count is invalid")
	}
	if resource.handles != nil {
		if _, ok := resource.handles[handle]; !ok {
			return errors.New("sandbox lease handle is not registered")
		}
	}
	if resource.uncertainLease != nil && handle != resource.uncertainLease {
		if resource.refs > 1 {
			if resource.handles != nil {
				delete(resource.handles, handle)
			}
			resource.refs--
			return nil
		}
		return resource.cleanupErr
	}
	if resource.refs > 1 {
		if resource.handles != nil {
			delete(resource.handles, handle)
			if handle == resource.uncertainLease {
				// The uncertain owner may release its own reference before sibling
				// handles. Transfer the cleanup fence to one still-live exact handle;
				// otherwise the final sibling would be unable to reconcile the resource
				// after the original owner handle became permanently idempotent.
				resource.uncertainLease = nil
				for remaining := range resource.handles {
					resource.uncertainLease = remaining
					break
				}
			}
		}
		resource.refs--
		return nil
	}
	cleanup := removeSandboxResource
	if resource.supervisedCleanup != nil {
		cleanup = resource.supervisedCleanup
	}
	if err := cleanup(resource); err != nil {
		resource.cleanupErr = err
		resource.marker.CleanupState = cleanupStateUnknown
		resource.marker.CleanupError = sandboxCleanupErrorSummary(err)
		resource.marker.CleanupUpdatedAt = time.Now().UTC()
		if markerErr := persistSandboxLeaseMarkerStateLocked(resource); markerErr != nil {
			resource.cleanupErr = errors.Join(err, markerErr)
		}
		return resource.cleanupErr
	}
	resource.refs = 0
	if resource.handles != nil {
		delete(resource.handles, handle)
	}
	resource.closed = true
	resource.cleanupErr = nil
	resource.marker.CleanupState = cleanupStateActive
	resource.marker.CleanupError = ""
	resource.marker.CleanupUpdatedAt = time.Time{}
	resource.uncertainLease = nil
	if resource.base != nil {
		_ = resource.base.Close()
		resource.base = nil
	}
	registryMu.Lock()
	if registry[resource.path] == resource {
		delete(registry, resource.path)
	}
	if byScope[resource.scopeKey] == resource {
		delete(byScope, resource.scopeKey)
	}
	registryMu.Unlock()
	return nil
}

func removeSandboxResource(resource *sandboxResource) error {
	if resource == nil || resource.base == nil {
		return errors.New("sandbox resource directory handle is unavailable")
	}
	basePath := filepath.Clean(resource.root)
	currentBase, err := confinedfs.Open(basePath)
	if err != nil {
		return fmt.Errorf("open sandbox scratch parent for cleanup: %w", err)
	}
	currentInfo, statErr := currentBase.Stat(".")
	closeErr := currentBase.Close()
	if statErr != nil {
		return fmt.Errorf("stat sandbox scratch parent for cleanup: %w", statErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close sandbox scratch parent check: %w", closeErr)
	}
	if resource.baseIdentity == nil || !os.SameFile(resource.baseIdentity, currentInfo) {
		return errors.New("sandbox scratch parent changed while cleaning")
	}
	name := filepath.Base(resource.path)
	if name == "" || name == "." || filepath.Dir(resource.path) != basePath || !strings.HasPrefix(name, ".scratch-") {
		return errors.New("sandbox scratch path is outside host lease")
	}
	observed, err := resource.base.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if observed.Mode()&os.ModeSymlink != 0 || !observed.IsDir() {
		return errors.New("sandbox scratch leaf is not a directory")
	}
	if resource.leafIdentity != nil && !os.SameFile(resource.leafIdentity, observed) {
		return errors.New("sandbox scratch leaf changed while cleaning")
	}
	return resource.base.RemoveAll(name)
}

// AcquireSandboxResource creates or reuses the active owner/session scratch lease.
func AcquireSandboxResource(ctx context.Context, input SandboxResourceInput) (*SandboxResourceLease, error) {
	return Acquire(ctx, input)
}

func newSandboxLeaseMarker(owner, session, roundID, runtimeRoot string) (SandboxLeaseMarker, error) {
	var rawID [16]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return SandboxLeaseMarker{}, err
	}
	processStartTime, err := currentProcessStartTimeUnixNano()
	if err != nil {
		// Marker creation must remain available when a platform cannot expose
		// process creation time. The zero value deliberately makes recovery
		// conservative rather than weakening it.
		processStartTime = 0
	}
	return SandboxLeaseMarker{
		Version:                  leaseMarkerVersion,
		LeaseID:                  hex.EncodeToString(rawID[:]),
		OwnerUserID:              owner,
		SessionKey:               session,
		RoundID:                  roundID,
		RuntimeRoot:              filepath.Clean(runtimeRoot),
		ProcessID:                os.Getpid(),
		ProcessStartTimeUnixNano: processStartTime,
		CreatedAt:                time.Now().UTC(),
		CleanupState:             cleanupStateActive,
	}, nil
}

func writeSandboxLeaseMarkerInRoot(root *confinedfs.Root, marker SandboxLeaseMarker) error {
	payload, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	file, err := root.OpenFile(leaseMarkerName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	writeErr := error(nil)
	if _, writeErr = file.Write(payload); writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = root.Remove(leaseMarkerName)
		return writeErr
	}
	return nil
}

// persistSandboxLeaseMarkerStateLocked replaces the marker through the fixed
// parent/leaf directory handles retained by the active lease. It is used only
// while resource.mu is held; an inability to persist the state never clears
// the in-memory cleanup fence.
func persistSandboxLeaseMarkerStateLocked(resource *sandboxResource) error {
	if resource == nil || resource.base == nil {
		return errors.New("sandbox lease marker root is unavailable")
	}
	name := filepath.Base(resource.path)
	if name == "" || name == "." || filepath.Dir(resource.path) != filepath.Clean(resource.root) || !strings.HasPrefix(name, ".scratch-") {
		return errors.New("sandbox lease marker path is outside host lease")
	}
	observed, err := resource.base.Lstat(name)
	if err != nil {
		return err
	}
	if observed.Mode()&os.ModeSymlink != 0 || !observed.IsDir() ||
		resource.leafIdentity == nil || !os.SameFile(resource.leafIdentity, observed) {
		return errors.New("sandbox lease marker leaf changed while persisting")
	}
	leaf, err := resource.base.OpenRootNoSymlink(name)
	if err != nil {
		return err
	}
	defer leaf.Close()
	payload, err := json.Marshal(resource.marker)
	if err != nil {
		return err
	}
	return leaf.WriteFileAtomic(leaseMarkerName, payload, 0o600)
}

func sandboxCleanupErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 512 {
		message = message[:512]
	}
	return message
}

func readSandboxLeaseMarker(path, runtimeRoot, owner string) (SandboxLeaseMarker, error) {
	leaseRoot, err := openSandboxDirectory(path)
	if err != nil {
		return SandboxLeaseMarker{}, err
	}
	defer leaseRoot.Close()
	marker, err := readSandboxLeaseMarkerInRoot(leaseRoot)
	if err != nil {
		return SandboxLeaseMarker{}, err
	}
	return validateSandboxLeaseMarker(marker, runtimeRoot, owner)
}

// validateSandboxLeaseMarker 同时服务恢复扫描与启动准入，避免两条路径对旧 marker 的解释分叉。
func validateSandboxLeaseMarker(marker SandboxLeaseMarker, runtimeRoot, owner string) (SandboxLeaseMarker, error) {
	if marker.Version != leaseMarkerVersion || strings.TrimSpace(marker.LeaseID) == "" ||
		strings.TrimSpace(marker.OwnerUserID) == "" || marker.OwnerUserID != owner ||
		marker.SessionKey == "" || marker.RuntimeRoot != runtimeRoot ||
		marker.ProcessID <= 0 || marker.ProcessStartTimeUnixNano < 0 || marker.CreatedAt.IsZero() {
		return SandboxLeaseMarker{}, errors.New("sandbox lease marker identity is invalid")
	}
	if marker.CleanupState == "" {
		marker.CleanupState = cleanupStateActive
	}
	if marker.CleanupState != cleanupStateActive && marker.CleanupState != cleanupStateUnknown {
		return SandboxLeaseMarker{}, errors.New("sandbox lease marker cleanup state is invalid")
	}
	if marker.CleanupState == cleanupStateUnknown {
		if marker.CleanupUpdatedAt.IsZero() || strings.TrimSpace(marker.CleanupError) == "" {
			return SandboxLeaseMarker{}, errors.New("sandbox lease marker cleanup state is incomplete")
		}
	} else if strings.TrimSpace(marker.CleanupError) != "" || !marker.CleanupUpdatedAt.IsZero() {
		return SandboxLeaseMarker{}, errors.New("sandbox lease marker active state has cleanup details")
	}
	return marker, nil
}

func readSandboxLeaseMarkerInRoot(leaseRoot *confinedfs.Root) (SandboxLeaseMarker, error) {
	if leaseRoot == nil {
		return SandboxLeaseMarker{}, errors.New("sandbox lease root is closed")
	}
	file, err := openSandboxRegularFile(leaseRoot, leaseMarkerName)
	if err != nil {
		return SandboxLeaseMarker{}, err
	}
	payload, readErr := io.ReadAll(io.LimitReader(file, 64<<10))
	closeErr := file.Close()
	if readErr != nil {
		return SandboxLeaseMarker{}, readErr
	}
	if closeErr != nil {
		return SandboxLeaseMarker{}, closeErr
	}
	var marker SandboxLeaseMarker
	if err := json.Unmarshal(payload, &marker); err != nil {
		return SandboxLeaseMarker{}, err
	}
	return marker, nil
}

// DiscoverSandboxResources lists valid durable markers for one owner. It is
// read-only and deliberately ignores malformed/untrusted marker directories.
func DiscoverSandboxResources(ctx context.Context, input SandboxResourceSweepInput) ([]SandboxResourceRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner := input.OwnerUserID
	if owner == "" {
		return nil, errors.New("sandbox resource discovery requires owner")
	}
	runtimeRoot, base, err := sandboxResourceRoots(owner, input.Root)
	if err != nil {
		return nil, err
	}
	return scanSandboxResources(ctx, owner, runtimeRoot, base, input.Now)
}

// SweepStaleSandboxResources performs an explicit owner-scoped stale cleanup.
// A dry run (Apply=false) returns eligible candidates without deleting them.
// Even with Apply=true, active in-process leases, live/unknown processes,
// malformed markers and younger leases are always retained.
func SweepStaleSandboxResources(ctx context.Context, input SandboxResourceSweepInput) (SandboxResourceSweepResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if input.OlderThan <= 0 {
		return SandboxResourceSweepResult{}, errors.New("sandbox resource sweep requires a positive age")
	}
	owner := input.OwnerUserID
	if owner == "" {
		return SandboxResourceSweepResult{}, errors.New("sandbox resource sweep requires owner")
	}
	runtimeRoot, base, err := sandboxResourceRoots(owner, input.Root)
	if err != nil {
		return SandboxResourceSweepResult{}, err
	}
	records, err := scanSandboxResources(ctx, owner, runtimeRoot, base, input.Now)
	if err != nil {
		return SandboxResourceSweepResult{}, err
	}
	result := SandboxResourceSweepResult{}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if record.Age < input.OlderThan || record.ProcessActive {
			result.Skipped = append(result.Skipped, record)
			continue
		}
		result.Candidates = append(result.Candidates, record)
	}
	if !input.Apply {
		return result, nil
	}
	for _, record := range result.Candidates {
		// Re-check the in-memory registry immediately before deletion. A host
		// restart cannot silently adopt a marker, and a concurrent local lease
		// must win over an explicit sweep.
		if sandboxResourceIsActive(record.Path) {
			result.Skipped = append(result.Skipped, record)
			continue
		}
		if err := removeStaleSandboxScratch(base, record.Path, record.leafIdentity, record.Marker.LeaseID); err != nil {
			return result, fmt.Errorf("remove stale sandbox resource %q: %w", record.Path, err)
		}
		result.Removed = append(result.Removed, record)
	}
	return result, nil
}

func sandboxResourceRoots(owner, requestedRoot string) (string, string, error) {
	runtimeRoot := strings.TrimSpace(requestedRoot)
	if runtimeRoot == "" {
		runtimeRoot = ownerRuntimeRoot(owner)
	}
	runtimeRoot, err := absoluteCleanDirectory(runtimeRoot)
	if err != nil {
		return "", "", fmt.Errorf("invalid sandbox runtime root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(runtimeRoot)
	if err != nil {
		return "", "", fmt.Errorf("validate sandbox runtime root: %w", err)
	}
	runtimeRoot = filepath.Clean(resolvedRoot)
	base := filepath.Join(runtimeRoot, scratchDirName)
	baseFS, err := openSandboxDirectory(base)
	if errors.Is(err, os.ErrNotExist) {
		return runtimeRoot, base, nil
	}
	if err != nil {
		return "", "", err
	}
	if err := baseFS.Close(); err != nil {
		return "", "", fmt.Errorf("close sandbox scratch parent: %w", err)
	}
	return runtimeRoot, base, nil
}

func scanSandboxResources(ctx context.Context, owner, runtimeRoot, base string, now time.Time) ([]SandboxResourceRecord, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	baseFS, err := openSandboxDirectory(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan sandbox scratch parent: %w", err)
	}
	defer baseFS.Close()
	entries, err := fs.ReadDir(baseFS.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("scan sandbox scratch parent: %w", err)
	}
	var records []SandboxResourceRecord
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return records, err
		}
		if !strings.HasPrefix(entry.Name(), ".scratch-") {
			continue
		}
		path := filepath.Join(base, entry.Name())
		info, err := baseFS.Lstat(entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return records, fmt.Errorf("inspect sandbox resource %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		marker, err := readSandboxLeaseMarker(path, runtimeRoot, owner)
		if err != nil {
			continue
		}
		active := sandboxResourceIsActive(path)
		if !active {
			// A cleanup_unknown marker is a durable statement that the previous
			// generation could not prove its close. A dead PID is insufficient
			// evidence because descendants, handles or helper processes may have
			// escaped the parent lifecycle. Keep it for explicit reconciliation.
			active = marker.CleanupState == cleanupStateUnknown
		}
		if !active {
			active = sandboxProcessMarkerIsActive(marker)
		}
		age := now.Sub(marker.CreatedAt)
		if age < 0 {
			age = 0
		}
		records = append(records, SandboxResourceRecord{Path: path, Marker: marker, Age: age, ProcessActive: active, leafIdentity: info})
	}
	return records, nil
}

func sandboxResourceIsActive(path string) bool {
	registryMu.Lock()
	resource := registry[path]
	registryMu.Unlock()
	if resource == nil {
		return false
	}
	resource.mu.Lock()
	active := !resource.closed || resource.cleanupErr != nil || resource.refs > 0
	resource.mu.Unlock()
	return active
}

func removeStaleSandboxScratch(base, path string, expectedLeaf os.FileInfo, expectedLeaseID string) error {
	base = filepath.Clean(base)
	path = filepath.Clean(path)
	if base == "." || path == "." || filepath.Dir(path) != base || !strings.HasPrefix(filepath.Base(path), ".scratch-") {
		return errors.New("sandbox stale path is outside host lease")
	}
	parent, err := openSandboxDirectory(base)
	if err != nil {
		return fmt.Errorf("open sandbox scratch parent for stale cleanup: %w", err)
	}
	defer parent.Close()
	name := filepath.Base(path)
	observed, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if observed.Mode()&os.ModeSymlink != 0 || !observed.IsDir() {
		return errors.New("sandbox stale leaf is not a directory")
	}
	if expectedLeaf == nil || !os.SameFile(expectedLeaf, observed) {
		return errors.New("sandbox stale leaf changed while cleaning")
	}
	leaf, err := openSandboxChildDirectory(parent, name)
	if err != nil {
		return err
	}
	defer leaf.Close()
	marker, err := readSandboxLeaseMarkerInRoot(leaf)
	if err != nil {
		return fmt.Errorf("verify stale sandbox marker: %w", err)
	}
	if strings.TrimSpace(expectedLeaseID) == "" || marker.LeaseID != expectedLeaseID {
		return errors.New("sandbox stale marker changed while cleaning")
	}
	return parent.RemoveAll(name)
}

func removeOwnedScratch(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == "." || path == "." || filepath.Dir(path) != root || !strings.HasPrefix(filepath.Base(path), ".scratch-") {
		return errors.New("sandbox scratch path is outside host lease")
	}
	parent, err := openSandboxDirectory(root)
	if err != nil {
		return fmt.Errorf("open sandbox scratch parent for cleanup: %w", err)
	}
	defer parent.Close()
	info, err := parent.Lstat(filepath.Base(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("sandbox scratch leaf is not a directory")
	}
	return parent.RemoveAll(filepath.Base(path))
}
