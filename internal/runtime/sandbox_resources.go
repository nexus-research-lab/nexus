// INPUT: Host-owned runtime identity and a session/round scope.
// OUTPUT: A private, versioned scratch directory and an idempotent release lease.
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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	policyVersion      = 1
	scratchDirName     = "sandbox"
	leaseMarkerName    = ".nexus-sandbox-lease.json"
	leaseMarkerVersion = 1
)

var (
	registryMu sync.Mutex
	registry   = map[string]*Lease{}
	byScope    = map[string]*Lease{}
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

// Lease owns one scratch directory. Release is safe to call more than once;
// a failed cleanup keeps the lease registered so the host can retry it.
type Lease struct {
	mu       sync.Mutex
	path     string
	root     string
	scopeKey string
	policy   agentclient.SandboxResourcePolicy
	marker   SandboxLeaseMarker
	released bool
}

// SandboxLeaseMarker is the durable identity left beside a scratch lease.
// It intentionally records only scope and process metadata; it is not an
// execution receipt and does not authorize another process to adopt the lease.
type SandboxLeaseMarker struct {
	Version     int       `json:"version"`
	LeaseID     string    `json:"lease_id"`
	OwnerUserID string    `json:"owner_user_id"`
	SessionKey  string    `json:"session_key"`
	RoundID     string    `json:"round_id,omitempty"`
	RuntimeRoot string    `json:"runtime_root"`
	ProcessID   int       `json:"process_id"`
	CreatedAt   time.Time `json:"created_at"`
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

// Acquire creates a private scratch directory before SDK initialize. It does
// not trust caller-provided absolute paths and never follows an existing
// symlink at the created leaf.
func Acquire(ctx context.Context, input Input) (*Lease, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner := strings.TrimSpace(input.OwnerUserID)
	session := strings.TrimSpace(input.SessionKey)
	_ = strings.TrimSpace(input.RoundID) // retained in Input for round-scoped audit callers
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
	base := filepath.Join(root, scratchDirName)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox scratch parent: %w", err)
	}
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil || filepath.Clean(resolvedBase) != base {
		if err == nil {
			err = errors.New("sandbox scratch parent resolves through a symlink")
		}
		return nil, fmt.Errorf("validate sandbox scratch parent: %w", err)
	}
	if err := os.Chmod(base, 0o700); err != nil {
		return nil, fmt.Errorf("lock sandbox scratch parent: %w", err)
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
		active := !existing.released
		existing.mu.Unlock()
		if active {
			return existing, nil
		}
	}

	digest := sha256.Sum256([]byte(scopeKey))
	name := ".scratch-" + hex.EncodeToString(digest[:])[:16]
	path := filepath.Join(base, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create sandbox scratch: %w", err)
		}
		// A directory left by a crashed host is never silently adopted. Keep a
		// unique replacement and let recovery tooling decide when to sweep it.
		path, err = os.MkdirTemp(base, name+"-stale-")
		if err != nil {
			return nil, fmt.Errorf("create sandbox scratch replacement: %w", err)
		}
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.RemoveAll(path)
		return nil, fmt.Errorf("lock sandbox scratch: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		_ = os.RemoveAll(path)
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
		_ = os.RemoveAll(path)
		return nil, fmt.Errorf("validate sandbox resource policy: %w", err)
	}
	marker, err := newSandboxLeaseMarker(owner, session, strings.TrimSpace(input.RoundID), root)
	if err != nil {
		_ = os.RemoveAll(path)
		return nil, fmt.Errorf("create sandbox lease marker: %w", err)
	}
	if err := writeSandboxLeaseMarker(path, marker); err != nil {
		_ = os.RemoveAll(path)
		return nil, fmt.Errorf("persist sandbox lease marker: %w", err)
	}
	lease := &Lease{path: path, root: base, scopeKey: scopeKey, policy: policy, marker: marker}
	registryMu.Lock()
	if existing := byScope[scopeKey]; existing != nil {
		registryMu.Unlock()
		_ = removeOwnedScratch(base, path)
		return existing, nil
	}
	registry[path] = lease
	byScope[scopeKey] = lease
	registryMu.Unlock()
	return lease, nil
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

// Resources returns an independent policy copy for SDK options.
func (l *Lease) Resources() *agentclient.SandboxResourcePolicy {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	policy := l.policy
	return &policy
}

// Path returns the host-owned scratch path.
func (l *Lease) Path() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
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
	if err := removeOwnedScratch(l.root, l.path); err != nil {
		return err
	}
	l.released = true
	registryMu.Lock()
	if registry[l.path] == l {
		delete(registry, l.path)
	}
	if byScope[l.scopeKey] == l {
		delete(byScope, l.scopeKey)
	}
	registryMu.Unlock()
	return nil
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
	return SandboxLeaseMarker{
		Version:     leaseMarkerVersion,
		LeaseID:     hex.EncodeToString(rawID[:]),
		OwnerUserID: owner,
		SessionKey:  session,
		RoundID:     roundID,
		RuntimeRoot: filepath.Clean(runtimeRoot),
		ProcessID:   os.Getpid(),
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func writeSandboxLeaseMarker(path string, marker SandboxLeaseMarker) error {
	payload, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	markerPath := filepath.Join(path, leaseMarkerName)
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
		_ = os.Remove(markerPath)
		return writeErr
	}
	return nil
}

func readSandboxLeaseMarker(path, runtimeRoot, owner string) (SandboxLeaseMarker, error) {
	markerPath := filepath.Join(path, leaseMarkerName)
	info, err := os.Lstat(markerPath)
	if err != nil {
		return SandboxLeaseMarker{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return SandboxLeaseMarker{}, errors.New("sandbox lease marker is not a regular file")
	}
	file, err := os.Open(markerPath)
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
	if marker.Version != leaseMarkerVersion || strings.TrimSpace(marker.LeaseID) == "" ||
		strings.TrimSpace(marker.OwnerUserID) == "" || marker.OwnerUserID != owner ||
		marker.SessionKey == "" || marker.RuntimeRoot != runtimeRoot ||
		marker.ProcessID <= 0 || marker.CreatedAt.IsZero() {
		return SandboxLeaseMarker{}, errors.New("sandbox lease marker identity is invalid")
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
	owner := strings.TrimSpace(input.OwnerUserID)
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
	owner := strings.TrimSpace(input.OwnerUserID)
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
		if err := removeOwnedScratch(base, record.Path); err != nil {
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
	info, err := os.Lstat(base)
	if errors.Is(err, os.ErrNotExist) {
		return runtimeRoot, base, nil
	}
	if err != nil {
		return "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", "", errors.New("sandbox scratch parent is not a directory")
	}
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil || filepath.Clean(resolvedBase) != base {
		if err == nil {
			err = errors.New("sandbox scratch parent resolves through a symlink")
		}
		return "", "", fmt.Errorf("validate sandbox scratch parent: %w", err)
	}
	return runtimeRoot, base, nil
}

func scanSandboxResources(ctx context.Context, owner, runtimeRoot, base string, now time.Time) ([]SandboxResourceRecord, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	entries, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
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
		info, err := os.Lstat(path)
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
			alive, known := sandboxProcessAlive(marker.ProcessID)
			active = alive || !known
		}
		age := now.Sub(marker.CreatedAt)
		if age < 0 {
			age = 0
		}
		records = append(records, SandboxResourceRecord{Path: path, Marker: marker, Age: age, ProcessActive: active})
	}
	return records, nil
}

func sandboxResourceIsActive(path string) bool {
	registryMu.Lock()
	lease := registry[path]
	registryMu.Unlock()
	if lease == nil {
		return false
	}
	lease.mu.Lock()
	active := !lease.released
	lease.mu.Unlock()
	return active
}

// ReleasePath is used by the runtime client cleanup path. It only releases
// paths previously created by Acquire; an arbitrary bridge option cannot make
// the host delete a user-selected directory.
func ReleasePath(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return nil
	}
	registryMu.Lock()
	lease := registry[path]
	registryMu.Unlock()
	if lease == nil {
		return nil
	}
	return lease.Release()
}

// ReleaseSandboxPath releases only a path previously registered by the host.
func ReleaseSandboxPath(path string) error {
	return ReleasePath(path)
}

func removeOwnedScratch(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == "." || path == "." || filepath.Dir(path) != root || !strings.HasPrefix(filepath.Base(path), ".scratch-") {
		return errors.New("sandbox scratch path is outside host lease")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("sandbox scratch leaf is not a directory")
	}
	return os.RemoveAll(path)
}
