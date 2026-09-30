// INPUT: 冻结的产品 Windows 策略、原始 scope/lease、Bridge 最终命令与持久仓储。
// OUTPUT: Reserve→Prepared→单次Start→cleaned/unknown callbacks，不记录环境秘密。
// POS: Nexus 私有授权适配，不向 Bridge 添加 SDK 依赖，不用 Darwin UID/launchd 字段。
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/windowssandbox"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type WindowsSandboxStore interface {
	ReserveWindowsSandbox(context.Context, protocol.WindowsSandboxIntent) error
	RecordWindowsSandboxPrepared(context.Context, protocol.WindowsSandboxIntent, protocol.WindowsSandboxPrepared) error
	ClaimWindowsSandboxStart(context.Context, protocol.WindowsSandboxIntent, protocol.WindowsSandboxPrepared) error
	FinishWindowsSandbox(context.Context, protocol.WindowsSandboxIntent, protocol.WindowsSandboxOutcome) error
	WindowsSandbox(context.Context, protocol.WindowsSandboxKey) (protocol.WindowsSandboxSnapshot, bool, error)
	LatestWindowsSandbox(context.Context, string, string) (protocol.WindowsSandboxSnapshot, bool, error)
	WindowsSandboxResources(context.Context, string, string, string) (protocol.WindowsSandboxResources, bool, error)
	CompleteWindowsSandboxResources(context.Context, protocol.WindowsSandboxKey) error
	PendingWindowsSandboxResourceRecords(context.Context, string, string) ([]protocol.WindowsSandboxResources, error)
}

type windowsSandboxBinding struct {
	Owner, Session, LeaseID, RoundID, Purpose, HelperSHA256 string
	ScratchRoot                                             string
	Generation                                              uint64
	PolicyDigest                                            [32]byte
}

type windowsSandboxHost struct {
	mu           sync.Mutex
	store        WindowsSandboxStore
	root         *confinedfs.Root
	binding      windowsSandboxBinding
	config       json.RawMessage
	endpoints    []WindowsSandboxNetworkEndpoint
	check        func(context.Context) error
	intent       *protocol.WindowsSandboxIntent
	bridgeIntent windowssandbox.Intent
}

var _ windowssandbox.Host = (*windowsSandboxHost)(nil)

// Reserve 先持久保存精确命令/授权摘要，再创建本次私有日志目录并返回正文；失败 owner 仍绑定原意图。
func (h *windowsSandboxHost) Reserve(ctx context.Context, i windowssandbox.Intent, command windowssandbox.Command) (json.RawMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.intent != nil || i.Version != 1 || !validSandboxLaunchID(i.LaunchID) || i.HelperSHA256 != h.binding.HelperSHA256 {
		return nil, errors.New("Windows Host intent cannot be reused or rebound")
	}
	if err := h.check(ctx); err != nil {
		return nil, err
	}
	commandBody, err := json.Marshal(command)
	if err != nil || sha256.Sum256(commandBody) != i.CommandDigest {
		return nil, errors.New("Bridge command differs from original Windows intent")
	}
	if !filepath.IsAbs(command.Program) || !filepath.IsAbs(command.Directory) {
		return nil, errors.New("Windows runtime command requires explicit absolute paths")
	}
	directory := filepath.Join(h.root.Name(), "windows", i.LaunchID)
	round := h.binding.RoundID
	if round == "" {
		round = "startup:" + i.LaunchID
	}
	request := struct {
		Owner      string                 `json:"owner_id"`
		Session    string                 `json:"session_id"`
		Round      string                 `json:"round_id"`
		Tool       string                 `json:"tool_name"`
		Use        string                 `json:"tool_use_id"`
		Generation uint64                 `json:"policy_generation"`
		Policy     [32]byte               `json:"policy_digest"`
		Command    windowssandbox.Command `json:"command"`
	}{h.binding.Owner, h.binding.Session, round, h.binding.Purpose, i.LaunchID, h.binding.Generation, h.binding.PolicyDigest, command}
	body, err := json.Marshal(struct {
		Request          any
		Config           json.RawMessage
		JournalRoot      string
		NetworkEndpoints []WindowsSandboxNetworkEndpoint
	}{request, h.config, directory, h.endpoints})
	if err != nil || len(body) > 900<<10 {
		return nil, errors.New("Windows authorization projection exceeds control frame budget")
	}
	intent := protocol.WindowsSandboxIntent{Key: protocol.WindowsSandboxKey{OwnerUserID: h.binding.Owner, SessionKey: h.binding.Session, Generation: h.binding.Generation, LaunchID: i.LaunchID}, Version: 1, Purpose: h.binding.Purpose, LeaseID: h.binding.LeaseID, ScratchRoot: h.binding.ScratchRoot, PolicyDigest: h.binding.PolicyDigest, OptionsDigest: sha256.Sum256(body), CommandDigest: i.CommandDigest, HelperSHA256: i.HelperSHA256}
	h.intent, h.bridgeIntent = &intent, i
	if err := h.store.ReserveWindowsSandbox(ctx, intent); err != nil {
		return nil, err
	}
	dir, err := h.root.OpenOrCreateRootNoSymlink(filepath.Join("windows", i.LaunchID), 0700)
	if err != nil {
		return nil, err
	}
	if err := dir.Close(); err != nil {
		return nil, err
	}
	return body, nil
}

func (h *windowsSandboxHost) exact(i windowssandbox.Intent, p windowssandbox.Prepared) error {
	if h.intent == nil || i != h.bridgeIntent || p.LaunchID != i.LaunchID {
		return errors.New("Windows callback differs from reserved product intent")
	}
	return nil
}

func windowsProductPrepared(p windowssandbox.Prepared) protocol.WindowsSandboxPrepared {
	return protocol.WindowsSandboxPrepared{ExecutionID: p.ExecutionID, PrepareDigest: p.PrepareDigest, ManifestDigest: p.ManifestDigest}
}

// RecordPrepared 原执行绑定独立持久保存，未知写入失败阻止随后 start。
func (h *windowsSandboxHost) RecordPrepared(ctx context.Context, i windowssandbox.Intent, p windowssandbox.Prepared) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.exact(i, p); err != nil {
		return err
	}
	if err := h.check(ctx); err != nil {
		return err
	}
	return h.store.RecordWindowsSandboxPrepared(ctx, *h.intent, windowsProductPrepared(p))
}

// ClaimStart 重新检查原 lease/策略再单次 CAS；持久响应丢失不得重放启动。
func (h *windowsSandboxHost) ClaimStart(ctx context.Context, i windowssandbox.Intent, p windowssandbox.Prepared) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.exact(i, p); err != nil {
		return err
	}
	if err := h.check(ctx); err != nil {
		return err
	}
	return h.store.ClaimWindowsSandboxStart(ctx, *h.intent, windowsProductPrepared(p))
}

// Finish 清理不要求旧授权仍活跃；只核原绑定与Bridge完整终态，unknown永远不释放scope槽。
func (h *windowsSandboxHost) Finish(ctx context.Context, i windowssandbox.Intent, outcome windowssandbox.Outcome) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.exact(i, outcome.Prepared); err != nil {
		return err
	}
	proof := protocol.WindowsSandboxOutcome{Prepared: windowsProductPrepared(outcome.Prepared), Cleaned: outcome.Cleaned, ExitCode: outcome.ExitCode, Reason: outcome.Reason}
	if !proof.Cleaned {
		proof.Reason = "windows_execution_cleanup_unknown"
	}
	return h.store.FinishWindowsSandbox(ctx, *h.intent, proof)
}
