// INPUT: App固定机器映像/Host日志根、可信端点与Manager原始会话资源策略。
// OUTPUT: Bridge Windows独立factory及跨重启pending栅栏，不复用Darwin监督类型。
// POS: 显式产品装配端口；默认nil，不自行开启Windows sandbox capability。
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"sync"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	bridgepermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/windowssandbox"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type WindowsSandboxSupervisor struct {
	Root                    *confinedfs.Root
	HelperPath              string
	HelperSHA256            string
	NetworkEndpoints        []WindowsSandboxNetworkEndpoint
	ResolveNetworkEndpoints func(context.Context, []string) ([]WindowsSandboxNetworkEndpoint, error)
}

// SetWindowsSandboxSupervisor 仅在启动前接收App拥有的私有目录/固定映像，不读任务环境或隐式选择路径。
func (m *Manager) SetWindowsSandboxSupervisor(config WindowsSandboxSupervisor) error {
	if config.ResolveNetworkEndpoints != nil && config.NetworkEndpoints != nil {
		return errors.New("Windows supervisor cannot mix static and resolved network endpoints")
	}
	digest, err := hex.DecodeString(config.HelperSHA256)
	if err != nil || len(digest) != 32 || strings.Trim(config.HelperSHA256, "0") == "" || hex.EncodeToString(digest) != config.HelperSHA256 || config.Root == nil || !filepath.IsAbs(config.Root.Name()) || !filepath.IsAbs(config.HelperPath) {
		return errors.New("invalid installed Windows supervisor configuration")
	}
	endpoints, err := json.Marshal(config.NetworkEndpoints)
	if err != nil {
		return err
	}
	config.NetworkEndpoints = nil
	if err := json.Unmarshal(endpoints, &config.NetworkEndpoints); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeStartups != 0 || len(m.sessions) != 0 || m.shutdownDone != nil || m.sandboxSupervisor != nil || m.windowsSandboxSupervisor != nil {
		return errors.New("Windows supervisor must be exclusively configured before startup")
	}
	if _, ok := m.sandboxReceiptStore.(WindowsSandboxStore); !ok {
		return errors.New("Windows supervisor requires its durable repository")
	}
	if _, ok := m.sandboxReceiptStore.(windowsRuntimeProcessReader); !ok {
		return errors.New("Windows supervisor requires exact original policy binding reader")
	}
	m.windowsSandboxSupervisor = &config
	return nil
}

// windowsSupervisedProcessOptions 固定原始owner/session代次，不用probe后缀绕过旧unknown。
func (m *Manager) windowsSupervisedProcessOptions(options bridge.Options, owner, session string, floor uint64, lease *SandboxResourceLease) (bridge.Options, error) {
	m.mu.RLock()
	config := m.windowsSandboxSupervisor
	store, ok := m.sandboxReceiptStore.(WindowsSandboxStore)
	m.mu.RUnlock()
	if config == nil {
		return options, nil
	}
	if options.Runtime.PermissionMode == bridgepermission.ModeBypassPermissions {
		if options.WindowsSandbox != nil || options.ProcessSupervision != nil || (options.Sandbox != nil && (options.Sandbox.RequireSandbox || options.Sandbox.Resources != nil)) {
			return bridge.Options{}, errors.New("Full Access cannot carry restricted Windows resources or an injected supervisor")
		}
		return options, nil
	}
	if !ok || floor >= math.MaxInt64-1 || owner == "" || session == "" || options.ProcessSupervision != nil || options.WindowsSandbox != nil {
		return bridge.Options{}, errors.New("invalid Windows supervised scope or replacement factory")
	}
	projection, err := projectWindowsSandboxConfig(options)
	if err != nil {
		return bridge.Options{}, err
	}
	leaseID, err := supervisedLeaseIdentity(options, owner, session, lease)
	if err != nil {
		return bridge.Options{}, err
	}
	baseDigest, err := sandboxPolicyDigest(options)
	if err != nil {
		return bridge.Options{}, err
	}
	body, err := json.Marshal(struct {
		Base      string
		Config    json.RawMessage
		Endpoints []WindowsSandboxNetworkEndpoint
	}{baseDigest, projection, config.NetworkEndpoints})
	if err != nil {
		return bridge.Options{}, err
	}
	binding := windowsSandboxBinding{Owner: owner, Session: session, Generation: floor + 1, LeaseID: leaseID, HelperSHA256: config.HelperSHA256, PolicyDigest: sha256.Sum256(body)}
	if lease != nil {
		binding.RoundID = lease.RoundID()
		resources := lease.Resources()
		if resources == nil {
			return bridge.Options{}, errors.New("Windows lease closed during preparation")
		}
		binding.ScratchRoot = resources.ScratchRoot
	}
	if err := configureWindowsSandboxLeaseCleanup(lease, config, store, binding); err != nil {
		return bridge.Options{}, err
	}
	source := options
	check := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.RLock()
		closed := m.shutdownDone != nil
		m.mu.RUnlock()
		if closed {
			return errors.New("Windows supervisor is shutting down")
		}
		actual, err := supervisedLeaseIdentity(source, owner, session, lease)
		if err != nil {
			return err
		}
		if actual != leaseID {
			return errors.New("Windows resource lease changed")
		}
		current, err := sandboxPolicyDigest(source)
		if err != nil {
			return err
		}
		if current != baseDigest {
			return errors.New("Windows policy changed before launch")
		}
		return nil
	}
	var prepareNetwork sync.Once
	var prepareNetworkErr error
	fixedEndpoints := config.NetworkEndpoints
	var approvedDomains []string
	if options.Sandbox.Network != nil {
		approvedDomains = append([]string(nil), options.Sandbox.Network.AllowedDomains...)
	}
	freezeNetwork := func(ctx context.Context) error {
		prepareNetwork.Do(func() {
			if config.ResolveNetworkEndpoints == nil {
				return
			}
			resolved, err := config.ResolveNetworkEndpoints(ctx, append([]string(nil), approvedDomains...))
			if err != nil {
				prepareNetworkErr = err
				return
			}
			// Detach callback-owned slices before hashing or exposing to either launch.
			encoded, err := json.Marshal(resolved)
			if err != nil {
				prepareNetworkErr = err
				return
			}
			if err = json.Unmarshal(encoded, &fixedEndpoints); err != nil {
				prepareNetworkErr = err
				return
			}
			body, err := json.Marshal(struct {
				Base      string
				Config    json.RawMessage
				Endpoints []WindowsSandboxNetworkEndpoint
			}{baseDigest, projection, fixedEndpoints})
			if err != nil {
				prepareNetworkErr = err
				return
			}
			binding.PolicyDigest = sha256.Sum256(body)
		})
		return prepareNetworkErr
	}
	options.WindowsSandbox = func(ctx context.Context, purpose supervision.Purpose) (windowssandbox.Config, error) {
		if err := check(ctx); err != nil {
			return windowssandbox.Config{}, err
		}
		purposeText := string(purpose)
		if purpose != supervision.Runtime && purpose != supervision.VersionProbe {
			return windowssandbox.Config{}, errors.New("Windows nxs supervisor refuses non-nxs probe purpose")
		}
		if err := freezeNetwork(ctx); err != nil {
			return windowssandbox.Config{}, err
		}
		if err := check(ctx); err != nil {
			return windowssandbox.Config{}, err
		}
		next := binding
		next.Purpose = purposeText
		host := &windowsSandboxHost{store: store, root: config.Root, binding: next, config: append(json.RawMessage(nil), projection...), endpoints: fixedEndpoints, check: check}
		return windowssandbox.Config{HelperPath: config.HelperPath, HelperSHA256: config.HelperSHA256, Host: host}, nil
	}
	return options, nil
}

// windowsSandboxStartupGeneration 在任何factory前读取原scope；unknown即使切Full Access或关配置也不能跳过。
func windowsSandboxStartupGeneration(ctx context.Context, store SandboxPolicyReceiptStore, owner, session string) (uint64, error) {
	reader, ok := store.(WindowsSandboxStore)
	if !ok {
		return 0, nil
	}
	pending, err := reader.PendingWindowsSandboxResourceRecords(ctx, owner, session)
	if err != nil {
		return 0, err
	}
	for _, record := range pending {
		// Only an existing original in-process owner may retain pending scratch
		// between warm launches. Cold startup never reconstructs it from a path.
		registryMu.Lock()
		resource := registry[record.ScratchRoot]
		registryMu.Unlock()
		if resource == nil {
			return 0, ErrSandboxCleanupPending
		}
		resource.mu.Lock()
		live := !resource.closed && resource.refs > 0 && resource.cleanupErr == nil && resource.marker.CleanupState != cleanupStateUnknown && resource.marker.OwnerUserID == owner && resource.marker.SessionKey == session && resource.marker.LeaseID == record.LeaseID && resource.windowsCleanupSupervisor != nil
		resource.mu.Unlock()
		if !live {
			return 0, ErrSandboxCleanupPending
		}
	}
	snapshot, found, err := reader.LatestWindowsSandbox(ctx, owner, session)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	key := snapshot.Intent.Key
	if key.OwnerUserID != owner || key.SessionKey != session || key.Generation == 0 || key.Generation >= math.MaxInt64 || strings.TrimSpace(key.LaunchID) == "" {
		return 0, errors.New("previous Windows sandbox scope is invalid")
	}
	if snapshot.Phase != "cleaned" || snapshot.Outcome == nil || !snapshot.Outcome.Cleaned {
		return 0, ErrSandboxCleanupPending
	}
	return key.Generation, nil
}
