package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const runtimeEmotionStateRelativePath = ".agents/emotion.json"

const (
	defaultRuntimeEmotionContextID = "default"
	runtimeEmotionBaseTTL          = 6 * time.Hour
	runtimeEmotionContextTTL       = 2 * time.Hour
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
	ContextID string                  `json:"context_id"`
	Base      RuntimeEmotionBase      `json:"base"`
	Context   *RuntimeEmotionContext  `json:"context,omitempty"`
	Composite RuntimeEmotionComposite `json:"composite"`
	Fatigue   RuntimeFatigueState     `json:"fatigue"`
	StatePath string                  `json:"state_path"`
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

type runtimeEmotionLegacyState struct {
	Mood      string     `json:"mood"`
	Energy    *int       `json:"energy"`
	Valence   *int       `json:"valence"`
	Summary   string     `json:"summary"`
	UpdatedAt *time.Time `json:"updated_at"`
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
	path := runtimeEmotionStatePath(workspacePath)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return file.Close()
}

// SetRuntimeEmotionBase 更新基础情绪。
func SetRuntimeEmotionBase(workspacePath string, update RuntimeEmotionBaseUpdate) (RuntimeEmotionView, error) {
	now := update.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	state := loadRuntimeEmotionState(workspacePath, now)
	state.Base = normalizeRuntimeEmotionBase(RuntimeEmotionBase{
		Mood:        update.Mood,
		Energy:      update.Energy,
		Valence:     update.Valence,
		Description: update.Description,
		UpdatedAt:   now,
	}, now)
	state = normalizeRuntimeEmotionState(state, now)
	if err := writeRuntimeEmotionState(workspacePath, state); err != nil {
		return RuntimeEmotionView{}, err
	}
	return buildRuntimeEmotionView(workspacePath, state, defaultRuntimeEmotionContextID, now), nil
}

// SetRuntimeEmotionContext 更新当前会话/房间上下文情绪。
func SetRuntimeEmotionContext(workspacePath string, update RuntimeEmotionContextUpdate) (RuntimeEmotionView, error) {
	now := update.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	contextID := normalizeRuntimeEmotionContextID(update.ContextID)
	state := loadRuntimeEmotionState(workspacePath, now)
	if state.Contexts == nil {
		state.Contexts = map[string]RuntimeEmotionContext{}
	}
	state.Contexts[contextID] = normalizeRuntimeEmotionContext(RuntimeEmotionContext{
		Mood:      update.Mood,
		Valence:   update.Valence,
		Trigger:   update.Trigger,
		UpdatedAt: now,
	}, now)
	state = normalizeRuntimeEmotionState(state, now)
	if err := writeRuntimeEmotionState(workspacePath, state); err != nil {
		return RuntimeEmotionView{}, err
	}
	return buildRuntimeEmotionView(workspacePath, state, contextID, now), nil
}

// ClearRuntimeEmotionContext 清除指定上下文情绪。
func ClearRuntimeEmotionContext(workspacePath string, contextID string) (RuntimeEmotionView, error) {
	now := time.Now()
	normalizedContextID := normalizeRuntimeEmotionContextID(contextID)
	state := loadRuntimeEmotionState(workspacePath, now)
	delete(state.Contexts, normalizedContextID)
	state = normalizeRuntimeEmotionState(state, now)
	if err := writeRuntimeEmotionState(workspacePath, state); err != nil {
		return RuntimeEmotionView{}, err
	}
	return buildRuntimeEmotionView(workspacePath, state, normalizedContextID, now), nil
}

func loadRuntimeEmotionState(workspacePath string, now time.Time) RuntimeEmotionState {
	state := defaultRuntimeEmotionState(now)
	path := runtimeEmotionStatePath(workspacePath)
	if path == "" {
		return state
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	var fileState RuntimeEmotionState
	if err = json.Unmarshal(payload, &fileState); err == nil && strings.TrimSpace(fileState.Base.Mood) != "" {
		return normalizeRuntimeEmotionState(fileState, now)
	}
	var legacy runtimeEmotionLegacyState
	if err = json.Unmarshal(payload, &legacy); err == nil && strings.TrimSpace(legacy.Mood) != "" {
		base := defaultRuntimeEmotionBase(now)
		base.Mood = strings.TrimSpace(legacy.Mood)
		if legacy.Energy != nil {
			base.Energy = *legacy.Energy
		}
		if legacy.Valence != nil {
			base.Valence = *legacy.Valence
		}
		if strings.TrimSpace(legacy.Summary) != "" {
			base.Description = strings.TrimSpace(legacy.Summary)
		}
		if legacy.UpdatedAt != nil {
			base.UpdatedAt = *legacy.UpdatedAt
		}
		state.Base = base
		return normalizeRuntimeEmotionState(state, now)
	}
	return state
}

func writeRuntimeEmotionState(workspacePath string, state RuntimeEmotionState) error {
	path := runtimeEmotionStatePath(workspacePath)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(normalizeRuntimeEmotionState(state, time.Now()), "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".emotion-*.json")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if _, err = file.Write(append(payload, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
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
	contextID = strings.TrimSpace(contextID)
	if contextID == "" {
		return defaultRuntimeEmotionContextID
	}
	return contextID
}

func isRuntimeEmotionExpired(updatedAt time.Time, now time.Time, ttl time.Duration) bool {
	return !updatedAt.IsZero() && now.Sub(updatedAt) > ttl
}

func clampRuntimeEmotionScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10 {
		return 10
	}
	return value
}

func clampFatigueScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
