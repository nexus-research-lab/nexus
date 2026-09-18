// INPUT: Host-owned runtime identity and a session/round scope.
// OUTPUT: A private, versioned scratch directory and an idempotent release lease.
// POS: The desktop host owns scratch creation and cleanup; the SDK/Bridge only
// receives the resulting SandboxResourcePolicy.
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const (
	policyVersion  = 1
	scratchDirName = "sandbox"
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
	released bool
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
	base := filepath.Join(root, scratchDirName)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox scratch parent: %w", err)
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
	scopeKey := owner + "\x00" + session
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
	lease := &Lease{path: path, root: base, scopeKey: scopeKey, policy: policy}
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
