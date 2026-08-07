package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const runtimeEmotionStateRelativePath = ".agents/emotion.json"

const (
	defaultRuntimeEmotionContextID = "default"
	runtimeEmotionBaseTTL          = 6 * time.Hour
	runtimeEmotionContextTTL       = 2 * time.Hour
)

var (
	// ErrRuntimeEmotionVersionConflict 表示情绪状态已被另一轮更新。
	ErrRuntimeEmotionVersionConflict = errors.New("runtime emotion version conflict")
	runtimeEmotionMutationLocks      sync.Map
)

// RuntimeEmotionBase 是 agent 的基础情绪锚点。
type RuntimeEmotionBase struct {
	Mood        string    `json:"mood"`
	Energy      int       `json:"energy"`
	Valence     int       `json:"valence"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RuntimeEmotionContext 是单个会话/房间上下文里的临时情绪。
type RuntimeEmotionContext struct {
	Mood      string    `json:"mood"`
	Valence   int       `json:"valence"`
	Trigger   string    `json:"trigger"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuntimeFatigueState 是轻量疲劳状态。
type RuntimeFatigueState struct {
	Status    string    `json:"status"`
	Level     int       `json:"level"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuntimeEmotionState 是 .agents/emotion.json 的持久化结构。
type RuntimeEmotionState struct {
	Version  int64                            `json:"version"`
	Base     RuntimeEmotionBase               `json:"base"`
	Contexts map[string]RuntimeEmotionContext `json:"contexts,omitempty"`
	Fatigue  RuntimeFatigueState              `json:"fatigue"`
}

// RuntimeEmotionComposite 是本轮最终用于表达的合成情绪。
type RuntimeEmotionComposite struct {
	Mood        string `json:"mood"`
	Energy      int    `json:"energy"`
	Valence     int    `json:"valence"`
	Description string `json:"description"`
}

// RuntimeEmotionView 是 prompt 和 CLI 展示使用的归一化视图。
type RuntimeEmotionView struct {
	Version   int64                   `json:"version"`
	ContextID string                  `json:"context_id"`
	Base      RuntimeEmotionBase      `json:"base"`
	Context   *RuntimeEmotionContext  `json:"context,omitempty"`
	Composite RuntimeEmotionComposite `json:"composite"`
	Fatigue   RuntimeFatigueState     `json:"fatigue"`
	StatePath string                  `json:"state_path,omitempty"`
}

// SafeRuntimeEmotionView 移除只供本机 CLI 诊断使用的绝对路径。
func SafeRuntimeEmotionView(view RuntimeEmotionView) RuntimeEmotionView {
	view.StatePath = ""
	return view
}

// RuntimeEmotionBaseUpdate 是 reset 命令的输入。
type RuntimeEmotionBaseUpdate struct {
	Mood        string
	Energy      int
	Valence     int
	Description string
	Timestamp   time.Time
}

// RuntimeEmotionContextUpdate 是 note 命令的输入。
type RuntimeEmotionContextUpdate struct {
	ContextID string
	Mood      string
	Valence   int
	Trigger   string
	Timestamp time.Time
}

// LoadRuntimeEmotionView 读取指定 workspace 的当前情绪视图。
func LoadRuntimeEmotionView(workspacePath string, contextID string, now time.Time) RuntimeEmotionView {
	if now.IsZero() {
		now = time.Now()
	}
	state := loadRuntimeEmotionState(workspacePath, now)
	return buildRuntimeEmotionView(workspacePath, state, contextID, now)
}

// EnsureRuntimeEmotionState 保证 agent workspace 内存在情绪状态文件。
func EnsureRuntimeEmotionState(workspacePath string) error {
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return ensureRuntimeEmotionStateAt(root)
}

func ensureRuntimeEmotionStateAt(root *confinedfs.Root) error {
	file, err := root.OpenFileNoSymlink(
		runtimeEmotionStateRelativePath,
		os.O_RDONLY,
		0,
	)
	if err == nil {
		return file.Close()
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(runtimeEmotionStateRelativePath), agentWorkspaceDirectoryMode()); err != nil {
		return err
	}
	file, err = root.OpenFileNoSymlink(
		runtimeEmotionStateRelativePath,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		agentWorkspaceFileMode(0o644),
	)
	if err != nil {
		if os.IsExist(err) {
			existing, openErr := root.OpenFileNoSymlink(
				runtimeEmotionStateRelativePath,
				os.O_RDONLY,
				0,
			)
			if openErr != nil {
				return openErr
			}
			return existing.Close()
		}
		return err
	}
	return file.Close()
}

// EnsureRuntimeEmotionStateAt 在已验证的 workspace 根中初始化情绪状态。
func EnsureRuntimeEmotionStateAt(root *confinedfs.Root) error {
	return ensureRuntimeEmotionStateAt(root)
}

// SetRuntimeEmotionBase 更新基础情绪。
func SetRuntimeEmotionBase(workspacePath string, update RuntimeEmotionBaseUpdate) (RuntimeEmotionView, error) {
	return setRuntimeEmotionBaseAtVersion(workspacePath, update, nil)
}

// SetRuntimeEmotionBaseAtVersion 仅在 version 匹配时更新基础情绪。
func SetRuntimeEmotionBaseAtVersion(
	workspacePath string,
	update RuntimeEmotionBaseUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionBaseAtVersion(workspacePath, update, &expectedVersion)
}

func setRuntimeEmotionBaseAtVersion(
	workspacePath string,
	update RuntimeEmotionBaseUpdate,
	expectedVersion *int64,
) (RuntimeEmotionView, error) {
	now := update.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	return mutateRuntimeEmotionState(
		workspacePath,
		defaultRuntimeEmotionContextID,
		now,
		expectedVersion,
		func(state *RuntimeEmotionState) {
			state.Base = normalizeRuntimeEmotionBase(RuntimeEmotionBase{
				Mood:        update.Mood,
				Energy:      update.Energy,
				Valence:     update.Valence,
				Description: update.Description,
				UpdatedAt:   now,
			}, now)
		},
	)
}

// SetRuntimeEmotionContext 更新当前会话/房间上下文情绪。
func SetRuntimeEmotionContext(workspacePath string, update RuntimeEmotionContextUpdate) (RuntimeEmotionView, error) {
	return setRuntimeEmotionContextAtVersion(workspacePath, update, nil)
}

// SetRuntimeEmotionContextAtVersion 仅在 version 匹配时更新指定上下文情绪。
func SetRuntimeEmotionContextAtVersion(
	workspacePath string,
	update RuntimeEmotionContextUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionContextAtVersion(workspacePath, update, &expectedVersion)
}

func setRuntimeEmotionContextAtVersion(
	workspacePath string,
	update RuntimeEmotionContextUpdate,
	expectedVersion *int64,
) (RuntimeEmotionView, error) {
	now := update.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	contextID := normalizeRuntimeEmotionContextID(update.ContextID)
	return mutateRuntimeEmotionState(
		workspacePath,
		contextID,
		now,
		expectedVersion,
		func(state *RuntimeEmotionState) {
			if state.Contexts == nil {
				state.Contexts = map[string]RuntimeEmotionContext{}
			}
			state.Contexts[contextID] = normalizeRuntimeEmotionContext(RuntimeEmotionContext{
				Mood:      update.Mood,
				Valence:   update.Valence,
				Trigger:   update.Trigger,
				UpdatedAt: now,
			}, now)
		},
	)
}

// ClearRuntimeEmotionContext 清除指定上下文情绪。
func ClearRuntimeEmotionContext(workspacePath string, contextID string) (RuntimeEmotionView, error) {
	return clearRuntimeEmotionContextAtVersion(workspacePath, contextID, nil)
}

// ClearRuntimeEmotionContextAtVersion 仅在 version 匹配时清除指定上下文情绪。
func ClearRuntimeEmotionContextAtVersion(
	workspacePath string,
	contextID string,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return clearRuntimeEmotionContextAtVersion(workspacePath, contextID, &expectedVersion)
}

func clearRuntimeEmotionContextAtVersion(
	workspacePath string,
	contextID string,
	expectedVersion *int64,
) (RuntimeEmotionView, error) {
	now := time.Now()
	normalizedContextID := normalizeRuntimeEmotionContextID(contextID)
	return mutateRuntimeEmotionState(
		workspacePath,
		normalizedContextID,
		now,
		expectedVersion,
		func(state *RuntimeEmotionState) {
			delete(state.Contexts, normalizedContextID)
		},
	)
}

func mutateRuntimeEmotionState(
	workspacePath string,
	contextID string,
	now time.Time,
	expectedVersion *int64,
	mutate func(*RuntimeEmotionState),
) (RuntimeEmotionView, error) {
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return RuntimeEmotionView{}, errors.New("runtime emotion workspace 不能为空")
	}
	unlock := lockRuntimeEmotionMutation(workspacePath)
	defer unlock()

	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return RuntimeEmotionView{}, err
	}
	defer root.Close()
	return mutateRuntimeEmotionStateAt(
		root,
		workspacePath,
		contextID,
		now,
		expectedVersion,
		mutate,
	)
}

func mutateRuntimeEmotionStateAt(
	root *confinedfs.Root,
	workspacePath string,
	contextID string,
	now time.Time,
	expectedVersion *int64,
	mutate func(*RuntimeEmotionState),
) (RuntimeEmotionView, error) {
	state := loadRuntimeEmotionStateAt(root, now)
	if expectedVersion != nil && state.Version != *expectedVersion {
		return RuntimeEmotionView{}, fmt.Errorf(
			"%w: expected=%d actual=%d",
			ErrRuntimeEmotionVersionConflict,
			*expectedVersion,
			state.Version,
		)
	}
	if mutate != nil {
		mutate(&state)
	}
	state.Version++
	state = normalizeRuntimeEmotionState(state, now)
	if err := writeRuntimeEmotionStateAt(root, state); err != nil {
		return RuntimeEmotionView{}, err
	}
	return buildRuntimeEmotionView(workspacePath, state, contextID, now), nil
}

func lockRuntimeEmotionMutation(workspacePath string) func() {
	key := filepath.Clean(strings.TrimSpace(workspacePath))
	value, _ := runtimeEmotionMutationLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

// GetAgentRuntimeEmotionView 在 owner 校验后的 Agent workspace 中读取情绪状态。
func (s *Service) GetAgentRuntimeEmotionView(
	ctx context.Context,
	agentID string,
	contextID string,
	now time.Time,
) (RuntimeEmotionView, error) {
	agentValue, err := s.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return RuntimeEmotionView{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	root, err := s.openAgentWorkspace(*agentValue, false)
	if err != nil {
		return RuntimeEmotionView{}, err
	}
	defer root.Close()
	state := loadRuntimeEmotionStateAt(root, now)
	return buildRuntimeEmotionView(agentValue.WorkspacePath, state, contextID, now), nil
}

// SetAgentRuntimeEmotionBaseAtVersion 在 owner 校验后的 Agent workspace 中以 CAS 更新基础情绪。
func (s *Service) SetAgentRuntimeEmotionBaseAtVersion(
	ctx context.Context,
	agentID string,
	update RuntimeEmotionBaseUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return s.mutateAgentRuntimeEmotion(
		ctx,
		agentID,
		defaultRuntimeEmotionContextID,
		update.Timestamp,
		expectedVersion,
		func(state *RuntimeEmotionState, now time.Time) {
			state.Base = normalizeRuntimeEmotionBase(RuntimeEmotionBase{
				Mood:        update.Mood,
				Energy:      update.Energy,
				Valence:     update.Valence,
				Description: update.Description,
				UpdatedAt:   now,
			}, now)
		},
	)
}

// SetAgentRuntimeEmotionContextAtVersion 以 CAS 更新当前可信 DM/Room 上下文情绪。
func (s *Service) SetAgentRuntimeEmotionContextAtVersion(
	ctx context.Context,
	agentID string,
	update RuntimeEmotionContextUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	contextID := normalizeRuntimeEmotionContextID(update.ContextID)
	return s.mutateAgentRuntimeEmotion(
		ctx,
		agentID,
		contextID,
		update.Timestamp,
		expectedVersion,
		func(state *RuntimeEmotionState, now time.Time) {
			if state.Contexts == nil {
				state.Contexts = map[string]RuntimeEmotionContext{}
			}
			state.Contexts[contextID] = normalizeRuntimeEmotionContext(RuntimeEmotionContext{
				Mood:      update.Mood,
				Valence:   update.Valence,
				Trigger:   update.Trigger,
				UpdatedAt: now,
			}, now)
		},
	)
}

// ClearAgentRuntimeEmotionContextAtVersion 以 CAS 清除当前可信 DM/Room 上下文情绪。
func (s *Service) ClearAgentRuntimeEmotionContextAtVersion(
	ctx context.Context,
	agentID string,
	contextID string,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	contextID = normalizeRuntimeEmotionContextID(contextID)
	return s.mutateAgentRuntimeEmotion(
		ctx,
		agentID,
		contextID,
		time.Time{},
		expectedVersion,
		func(state *RuntimeEmotionState, _ time.Time) {
			delete(state.Contexts, contextID)
		},
	)
}

func (s *Service) mutateAgentRuntimeEmotion(
	ctx context.Context,
	agentID string,
	contextID string,
	now time.Time,
	expectedVersion int64,
	mutate func(*RuntimeEmotionState, time.Time),
) (RuntimeEmotionView, error) {
	agentValue, err := s.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return RuntimeEmotionView{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	unlock := lockRuntimeEmotionMutation(agentValue.WorkspacePath)
	defer unlock()
	root, err := s.openAgentWorkspace(*agentValue, false)
	if err != nil {
		return RuntimeEmotionView{}, err
	}
	defer root.Close()
	return mutateRuntimeEmotionStateAt(
		root,
		agentValue.WorkspacePath,
		contextID,
		now,
		&expectedVersion,
		func(state *RuntimeEmotionState) {
			mutate(state, now)
		},
	)
}

func loadRuntimeEmotionState(workspacePath string, now time.Time) RuntimeEmotionState {
	state := defaultRuntimeEmotionState(now)
	if strings.TrimSpace(workspacePath) == "" {
		return state
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return state
	}
	defer root.Close()
	return loadRuntimeEmotionStateAt(root, now)
}

func loadRuntimeEmotionStateAt(root *confinedfs.Root, now time.Time) RuntimeEmotionState {
	state := defaultRuntimeEmotionState(now)
	if root == nil {
		return state
	}
	payload, err := root.ReadFile(runtimeEmotionStateRelativePath)
	if err != nil {
		return state
	}
	var fileState RuntimeEmotionState
	if err = json.Unmarshal(payload, &fileState); err == nil && strings.TrimSpace(fileState.Base.Mood) != "" {
		return normalizeRuntimeEmotionState(fileState, now)
	}
	return state
}

func writeRuntimeEmotionState(workspacePath string, state RuntimeEmotionState) error {
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeRuntimeEmotionStateAt(root, state)
}

func writeRuntimeEmotionStateAt(root *confinedfs.Root, state RuntimeEmotionState) error {
	if err := root.MkdirAll(filepath.Dir(runtimeEmotionStateRelativePath), agentWorkspaceDirectoryMode()); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFileAtomic(
		runtimeEmotionStateRelativePath,
		append(payload, '\n'),
		agentWorkspaceFileMode(0o600),
	)
}

func runtimeEmotionStatePath(workspacePath string) string {
	root := strings.TrimSpace(workspacePath)
	if root == "" {
		return ""
	}
	return filepath.Join(root, runtimeEmotionStateRelativePath)
}

func buildRuntimeEmotionView(
	workspacePath string,
	state RuntimeEmotionState,
	contextID string,
	now time.Time,
) RuntimeEmotionView {
	contextID = normalizeRuntimeEmotionContextID(contextID)
	state = normalizeRuntimeEmotionState(state, now)
	context, hasContext := state.Contexts[contextID]
	if hasContext && isRuntimeEmotionExpired(context.UpdatedAt, now, runtimeEmotionContextTTL) {
		hasContext = false
	}
	var contextPtr *RuntimeEmotionContext
	if hasContext {
		contextCopy := context
		contextPtr = &contextCopy
	}
	return RuntimeEmotionView{
		Version:   state.Version,
		ContextID: contextID,
		Base:      state.Base,
		Context:   contextPtr,
		Composite: composeRuntimeEmotion(state.Base, contextPtr),
		Fatigue:   state.Fatigue,
		StatePath: runtimeEmotionStatePath(workspacePath),
	}
}

func composeRuntimeEmotion(base RuntimeEmotionBase, contextValue *RuntimeEmotionContext) RuntimeEmotionComposite {
	composite := RuntimeEmotionComposite{
		Mood:        base.Mood,
		Energy:      base.Energy,
		Valence:     base.Valence,
		Description: base.Description,
	}
	if contextValue == nil {
		return composite
	}
	composite.Mood = contextValue.Mood
	composite.Valence = clampRuntimeEmotionScore((base.Valence + contextValue.Valence) / 2)
	composite.Description = strings.TrimSpace(contextValue.Trigger)
	if composite.Description == "" {
		composite.Description = base.Description
	}
	return composite
}

func defaultRuntimeEmotionState(now time.Time) RuntimeEmotionState {
	return RuntimeEmotionState{
		Version:  1,
		Base:     defaultRuntimeEmotionBase(now),
		Contexts: map[string]RuntimeEmotionContext{},
		Fatigue: RuntimeFatigueState{
			Status:    "awake",
			Level:     0,
			UpdatedAt: now,
		},
	}
}

func defaultRuntimeEmotionBase(now time.Time) RuntimeEmotionBase {
	return RuntimeEmotionBase{
		Mood:        "focused",
		Energy:      6,
		Valence:     6,
		Description: "clear, proactive, concise",
		UpdatedAt:   now,
	}
}

func normalizeRuntimeEmotionState(state RuntimeEmotionState, now time.Time) RuntimeEmotionState {
	if state.Version <= 0 {
		state.Version = 1
	}
	if strings.TrimSpace(state.Base.Mood) == "" || isRuntimeEmotionExpired(state.Base.UpdatedAt, now, runtimeEmotionBaseTTL) {
		state.Base = defaultRuntimeEmotionBase(now)
	} else {
		state.Base = normalizeRuntimeEmotionBase(state.Base, now)
	}
	if state.Contexts == nil {
		state.Contexts = map[string]RuntimeEmotionContext{}
	}
	for key, contextValue := range state.Contexts {
		normalizedKey := normalizeRuntimeEmotionContextID(key)
		if isRuntimeEmotionExpired(contextValue.UpdatedAt, now, runtimeEmotionContextTTL) {
			delete(state.Contexts, key)
			continue
		}
		if normalizedKey != key {
			delete(state.Contexts, key)
		}
		state.Contexts[normalizedKey] = normalizeRuntimeEmotionContext(contextValue, now)
	}
	if strings.TrimSpace(state.Fatigue.Status) == "" {
		state.Fatigue.Status = "awake"
	}
	state.Fatigue.Status = strings.TrimSpace(state.Fatigue.Status)
	state.Fatigue.Level = clampFatigueScore(state.Fatigue.Level)
	if state.Fatigue.UpdatedAt.IsZero() {
		state.Fatigue.UpdatedAt = now
	}
	return state
}

func normalizeRuntimeEmotionBase(base RuntimeEmotionBase, now time.Time) RuntimeEmotionBase {
	base.Mood = strings.TrimSpace(base.Mood)
	if base.Mood == "" {
		base.Mood = "focused"
	}
	base.Energy = clampRuntimeEmotionScore(base.Energy)
	base.Valence = clampRuntimeEmotionScore(base.Valence)
	base.Description = strings.TrimSpace(base.Description)
	if base.Description == "" {
		base.Description = "clear, proactive, concise"
	}
	if base.UpdatedAt.IsZero() {
		base.UpdatedAt = now
	}
	return base
}

func normalizeRuntimeEmotionContext(contextValue RuntimeEmotionContext, now time.Time) RuntimeEmotionContext {
	contextValue.Mood = strings.TrimSpace(contextValue.Mood)
	if contextValue.Mood == "" {
		contextValue.Mood = "focused"
	}
	contextValue.Valence = clampRuntimeEmotionScore(contextValue.Valence)
	contextValue.Trigger = strings.TrimSpace(contextValue.Trigger)
	if contextValue.UpdatedAt.IsZero() {
		contextValue.UpdatedAt = now
	}
	return contextValue
}

func normalizeRuntimeEmotionContextID(contextID string) string {
	return cmp.Or(strings.TrimSpace(contextID), defaultRuntimeEmotionContextID)
}

func isRuntimeEmotionExpired(updatedAt time.Time, now time.Time, ttl time.Duration) bool {
	return !updatedAt.IsZero() && now.Sub(updatedAt) > ttl
}

func clampRuntimeEmotionScore(value int) int {
	return min(max(value, 0), 10)
}

func clampFatigueScore(value int) int {
	return min(max(value, 0), 100)
}
