// INPUT: canonical overlay/transcript、round 分页游标与可取消请求上下文。
// OUTPUT: 短请求冷建、有界 exact-scope admission 与与 canonical 等价的 round 分页投影。
// POS: workspace DM/Room canonical 历史与 SQLite 派生读模型之间的唯一编排层。
package workspace

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	historyPageIndexVersion      = 4
	historyPageEdgeHashBytes     = 4 * 1024
	historyPageBuildAttempts     = 2
	historyPageRebuildTimeout    = 15 * time.Minute
	historyPageForegroundWait    = 150 * time.Millisecond
	historyPageIndexRetryAfterMS = 250
	historyPageRebuildWorkers    = 2
	historyPageSourceMaxBytes    = 64 * 1024 * 1024
)

const (
	historyPageSourceDMOverlay          = "dm_overlay"
	historyPageSourceRoomLedger         = "room_ledger"
	historyPageSourceRoomPrivateOverlay = "room_private_overlay"
	historyPageSourceTranscript         = "transcript"
)

var (
	errHistoryPageIndexInvalid       = errors.New("history page index is invalid")
	errHistoryPageIndexResourceLimit = errors.New("history page index resource limit exceeded")
	historyPageRebuilds              = newHistoryPageRebuildManager(historyPageRebuildWorkers)
	historyPageTimeType              = reflect.TypeOf(time.Time{})
	historyPageJSONMarshalerType     = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	historyPageTextMarshalerType     = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

const historyPageIndexDisabledResourceLimit = "resource_limit"

const (
	historyPageDisabledCoverageAll        = "all"
	historyPageDisabledCoverageRoomLedger = "room_ledger"
)

type historyPageIndexDisabledError struct {
	sourceDigest string
	coverage     string
}

func (e *historyPageIndexDisabledError) Error() string {
	return "history page index is disabled for the current canonical sources"
}

type historyPageRebuildFuture struct {
	ready    chan struct{}
	complete chan struct{}
	built    historyPageIndexBuild
	err      error
}

type historyPageRebuildManager struct {
	mu        sync.Mutex
	futures   map[string]*historyPageRebuildFuture
	admission chan struct{}
	changed   chan struct{}
}

func newHistoryPageRebuildManager(workerCount int) *historyPageRebuildManager {
	if workerCount < 1 {
		workerCount = 1
	}
	return &historyPageRebuildManager{
		futures:   make(map[string]*historyPageRebuildFuture),
		admission: make(chan struct{}, workerCount),
		changed:   make(chan struct{}),
	}
}

type historyPageSourceSnapshot struct {
	Kind            string `json:"kind"`
	WorkspacePath   string `json:"workspace_path,omitempty"`
	SessionKey      string `json:"session_key,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	ResolvedPath    string `json:"resolved_path,omitempty"`
	Exists          bool   `json:"exists"`
	Size            int64  `json:"size,omitempty"`
	ModifiedUnixNS  int64  `json:"modified_unix_ns,omitempty"`
	EdgeFingerprint string `json:"edge_fingerprint,omitempty"`
	FileIdentity    string `json:"file_identity,omitempty"`
}

type historyPageIndexGroup struct {
	CursorRoundID        string `json:"cursor_round_id"`
	CursorRoundTimestamp int64  `json:"cursor_round_timestamp"`
	GroupKey             string `json:"group_key"`
	// FirstNonSyntheticTimestamp 指向该 physical group 中首条不受 active
	// 状态影响的 row；nil 表示该 group 完全由 synthetic interrupt 组成。
	FirstNonSyntheticTimestamp *int64                          `json:"first_non_synthetic_timestamp,omitempty"`
	SyntheticInterrupts        []historyPageSyntheticInterrupt `json:"synthetic_interrupts,omitempty"`
}

type historyPageSyntheticInterrupt struct {
	RoundID   string `json:"round_id"`
	Timestamp int64  `json:"timestamp"`
}

type historyPageIndexedGroup struct {
	CursorRoundID        string                    `json:"cursor_round_id"`
	CursorRoundTimestamp int64                     `json:"cursor_round_timestamp"`
	GroupKey             string                    `json:"group_key"`
	Items                []protocol.Message        `json:"items"`
	DeliveryReceipts     []ExternalDeliveryReceipt `json:"delivery_receipts,omitempty"`
}

type historyPageIndexBuild struct {
	Groups     []historyPageIndexedGroup
	RoundIndex []protocol.SessionRoundIndexItem
	Sources    []historyPageSourceSnapshot
	Cacheable  bool
	Disabled   bool
	// DisabledCoverage 只在 Build 于 canonical read 前触发 source budget 时使用；
	// Room ledger 自身超限时不能为了计算 marker 再解析 ledger 发现 dependencies。
	DisabledCoverage string
}

type historyPageIndexRequest struct {
	Limit                int
	BeforeRoundID        string
	BeforeRoundTimestamp int64
	AroundRoundID        string
	AroundLimit          int
	CollapseRoomRounds   bool
	ActiveRoundIDs       []string
	DeferIndex           bool
}

type historyPageIndexSelection struct {
	Indexes                  []int
	HasMore                  bool
	NextBeforeRoundID        *string
	NextBeforeRoundTimestamp *int64
}

type historyPageIndexAccess struct {
	Scope           string
	ReadModel       *historyReadModel
	OpenRoot        func(create bool) (*confinedfs.Root, error)
	ValidateSources func(context.Context, []historyPageSourceSnapshot) (bool, error)
	Build           func(context.Context) (historyPageIndexBuild, error)
	BuildCanonical  func(context.Context) (historyPageIndexBuild, error)
	Persist         func(context.Context, historyPageIndexBuild) error
}

func readHistoryPageWithIndex(
	ctx context.Context,
	access historyPageIndexAccess,
	request historyPageIndexRequest,
) (protocol.MessagePage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.MessagePage{}, err
	}

	if page, ok, err := loadHistoryPageIndex(ctx, access, request); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return protocol.MessagePage{}, err
		}
		var disabled *historyPageIndexDisabledError
		if errors.As(err, &disabled) {
			page, built, buildErr := buildCanonicalHistoryPage(ctx, access, request)
			if buildErr != nil {
				return protocol.MessagePage{}, buildErr
			}
			// resource-limit marker 与 canonical source 一致时保持 exact
			// fallback，不再制造 detached rebuild loop。source 变化后只在当前
			// 有空闲 admission 时启动一次新尝试，首屏不为恢复索引等待。
			if built.Cacheable &&
				historyPageSourceDigestForCoverage(built.Sources, disabled.coverage) != disabled.sourceDigest {
				_, _ = historyPageRebuilds.tryStart(ctx, access)
			}
			return page, nil
		}
		if errors.Is(err, errHistoryPageIndexResourceLimit) {
			page, _, buildErr := buildCanonicalHistoryPage(ctx, access, request)
			return page, buildErr
		}
	} else if ok {
		return page, nil
	}
	built, indexing, err := awaitHistoryPageIndexBuild(ctx, access, request.DeferIndex)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	if indexing {
		return historyPageIndexingPage(), nil
	}
	if built.Disabled {
		page, _, buildErr := buildCanonicalHistoryPage(ctx, access, request)
		return page, buildErr
	}
	if historyPageBuildHasLargeDetails(built) {
		// 大内容 generation 在 ready 前同步发布；优先从读模型返回 detail
		// 引用。落盘失败时仍回退完整 canonical 页，不牺牲历史可读性。
		if page, ok, loadErr := loadHistoryPageIndex(ctx, access, request); loadErr == nil && ok {
			return page, nil
		} else if errors.Is(loadErr, context.Canceled) || errors.Is(loadErr, context.DeadlineExceeded) {
			return protocol.MessagePage{}, loadErr
		}
	}
	return paginateHistoryPageIndexedGroups(built.Groups, request), nil
}

func historyPageIndexingPage() protocol.MessagePage {
	return protocol.MessagePage{
		Items:        []protocol.Message{},
		Indexing:     true,
		RetryAfterMS: historyPageIndexRetryAfterMS,
	}
}

func awaitHistoryPageIndexBuild(
	ctx context.Context,
	access historyPageIndexAccess,
	deferIndex bool,
) (historyPageIndexBuild, bool, error) {
	for {
		var future *historyPageRebuildFuture
		var err error
		if deferIndex {
			future, err = historyPageRebuilds.tryStart(ctx, access)
			if errors.Is(err, errHistoryPageIndexResourceLimit) {
				return historyPageIndexBuild{}, true, nil
			}
		} else {
			future, err = historyPageRebuilds.start(ctx, access)
		}
		if err != nil {
			return historyPageIndexBuild{}, false, err
		}
		var timer *time.Timer
		var timeout <-chan time.Time
		if deferIndex {
			timer = time.NewTimer(historyPageForegroundWait)
			timeout = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return historyPageIndexBuild{}, false, ctx.Err()
		case <-timeout:
			return historyPageIndexBuild{}, true, nil
		case <-future.ready:
			if timer != nil {
				timer.Stop()
			}
			if future.err != nil {
				return historyPageIndexBuild{}, false, future.err
			}
			valid, validateErr := access.ValidateSources(ctx, future.built.Sources)
			if validateErr != nil {
				return historyPageIndexBuild{}, false, validateErr
			}
			if valid {
				return future.built, false, nil
			}
			// canonical source 在 build/persist 窗口继续推进；不能复用已经 ready
			// 的旧 generation，等待其原子落盘退出后再启动下一代。
			select {
			case <-ctx.Done():
				return historyPageIndexBuild{}, false, ctx.Err()
			case <-future.complete:
			}
		}
	}
}

func buildCanonicalHistoryPage(
	ctx context.Context,
	access historyPageIndexAccess,
	request historyPageIndexRequest,
) (protocol.MessagePage, historyPageIndexBuild, error) {
	build := access.BuildCanonical
	if build == nil {
		build = access.Build
	}
	built, err := build(ctx)
	if err != nil {
		return protocol.MessagePage{}, historyPageIndexBuild{}, err
	}
	if built.Groups == nil {
		built.Groups = []historyPageIndexedGroup{}
	}
	if built.Sources == nil {
		built.Sources = []historyPageSourceSnapshot{}
	}
	return paginateHistoryPageIndexedGroups(built.Groups, request), built, nil
}

func (m *historyPageRebuildManager) start(
	requestCtx context.Context,
	access historyPageIndexAccess,
) (*historyPageRebuildFuture, error) {
	return m.startWithAdmission(requestCtx, access, true)
}

func (m *historyPageRebuildManager) tryStart(
	requestCtx context.Context,
	access historyPageIndexAccess,
) (*historyPageRebuildFuture, error) {
	return m.startWithAdmission(requestCtx, access, false)
}

func (m *historyPageRebuildManager) startWithAdmission(
	requestCtx context.Context,
	access historyPageIndexAccess,
	wait bool,
) (*historyPageRebuildFuture, error) {
	for {
		m.mu.Lock()
		if current := m.futures[access.Scope]; current != nil {
			m.mu.Unlock()
			return current, nil
		}
		changed := m.changed
		m.mu.Unlock()

		if wait {
			select {
			case m.admission <- struct{}{}:
			case <-changed:
				// 某个 scope 的 future 创建/完成后重新检查 exact scope；
				// 同 scope waiter 不必等 worker slot 才能 attach。
				continue
			case <-requestCtx.Done():
				return nil, requestCtx.Err()
			}
		} else {
			select {
			case m.admission <- struct{}{}:
			case <-requestCtx.Done():
				return nil, requestCtx.Err()
			default:
				return nil, errHistoryPageIndexResourceLimit
			}
		}

		// 两个 unique waiter 可能同时等到 admission；拿到 slot 后必须再次
		// 检查 exact scope，避免为同一会话启动重复 generation。
		m.mu.Lock()
		if current := m.futures[access.Scope]; current != nil {
			m.mu.Unlock()
			<-m.admission
			return current, nil
		}
		future := &historyPageRebuildFuture{
			ready:    make(chan struct{}),
			complete: make(chan struct{}),
		}
		m.futures[access.Scope] = future
		m.notifyLocked()
		m.mu.Unlock()

		go m.run(requestCtx, access, future)
		return future, nil
	}
}

func (m *historyPageRebuildManager) notifyLocked() {
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *historyPageRebuildManager) run(
	requestCtx context.Context,
	access historyPageIndexAccess,
	future *historyPageRebuildFuture,
) {
	base := context.WithoutCancel(requestCtx)
	buildCtx, cancel := context.WithTimeout(base, historyPageRebuildTimeout)
	defer cancel()

	future.built, future.err = access.Build(buildCtx)
	if future.built.Groups == nil {
		future.built.Groups = []historyPageIndexedGroup{}
	}
	if future.built.Sources == nil {
		future.built.Sources = []historyPageSourceSnapshot{}
	}
	shouldPersist := future.err == nil && future.built.Cacheable &&
		shouldPersistHistoryPageIndex(future.built)
	persistBeforeReady := shouldPersist && historyPageBuildHasLargeDetails(future.built)
	if persistBeforeReady {
		persistHistoryPageIndex(buildCtx, access, future.built)
	}
	// 普通 generation 仍让 waiter 只等待 canonical 投影；包含大型 detail
	// 的 generation 必须先原子发布引用目标，避免首个页面重新内联大内容。
	close(future.ready)
	if shouldPersist && !persistBeforeReady {
		persistHistoryPageIndex(buildCtx, access, future.built)
	}
	m.finish(access.Scope, future)
}

func persistHistoryPageIndex(
	ctx context.Context,
	access historyPageIndexAccess,
	built historyPageIndexBuild,
) {
	if access.Persist != nil {
		_ = access.Persist(ctx, built)
	} else if access.ReadModel != nil {
		_ = access.ReadModel.persist(ctx, access, built)
	}
}

func (m *historyPageRebuildManager) finish(scope string, future *historyPageRebuildFuture) {
	m.mu.Lock()
	if m.futures[scope] == future {
		delete(m.futures, scope)
		m.notifyLocked()
	}
	m.mu.Unlock()
	close(future.complete)
	<-m.admission
}

func shouldPersistHistoryPageIndex(built historyPageIndexBuild) bool {
	if built.Disabled {
		return true
	}
	if len(built.Groups) > 0 {
		return true
	}
	for _, source := range built.Sources {
		if source.Exists {
			return true
		}
	}
	return false
}

func loadHistoryPageIndex(
	ctx context.Context,
	access historyPageIndexAccess,
	request historyPageIndexRequest,
) (protocol.MessagePage, bool, error) {
	if access.ReadModel == nil {
		return protocol.MessagePage{}, false, errors.New("history read model is not configured")
	}
	return access.ReadModel.load(ctx, access, request)
}

func historyPageIndexMetadataForGroup(group historyPageIndexedGroup) historyPageIndexGroup {
	metadata := historyPageIndexGroup{
		CursorRoundID:        group.CursorRoundID,
		CursorRoundTimestamp: group.CursorRoundTimestamp,
		GroupKey:             group.GroupKey,
	}
	for _, row := range group.Items {
		if roundID := indexedSyntheticInterruptRoundID(row); roundID != "" {
			metadata.SyntheticInterrupts = append(
				metadata.SyntheticInterrupts,
				historyPageSyntheticInterrupt{
					RoundID:   roundID,
					Timestamp: messageTimestamp(row),
				},
			)
			continue
		}
		if metadata.FirstNonSyntheticTimestamp == nil {
			timestamp := messageTimestamp(row)
			metadata.FirstNonSyntheticTimestamp = &timestamp
		}
	}
	return metadata
}

func historyPageGroupVisibilityMatches(
	metadata historyPageIndexGroup,
	group historyPageIndexedGroup,
) bool {
	actual := historyPageIndexMetadataForGroup(group)
	if actual.CursorRoundID != metadata.CursorRoundID ||
		actual.CursorRoundTimestamp != metadata.CursorRoundTimestamp ||
		actual.GroupKey != metadata.GroupKey ||
		!slices.Equal(actual.SyntheticInterrupts, metadata.SyntheticInterrupts) {
		return false
	}
	if actual.FirstNonSyntheticTimestamp == nil || metadata.FirstNonSyntheticTimestamp == nil {
		return actual.FirstNonSyntheticTimestamp == nil && metadata.FirstNonSyntheticTimestamp == nil
	}
	return *actual.FirstNonSyntheticTimestamp == *metadata.FirstNonSyntheticTimestamp
}

func historyPageSourceDigest(sources []historyPageSourceSnapshot) string {
	hasher := sha256.New()
	writeString := func(value string) {
		_, _ = io.WriteString(hasher, strconv.Itoa(len(value)))
		_, _ = io.WriteString(hasher, ":")
		_, _ = io.WriteString(hasher, value)
		_, _ = io.WriteString(hasher, "|")
	}
	writeString(strconv.Itoa(len(sources)))
	for _, source := range sources {
		writeString(source.Kind)
		writeString(source.WorkspacePath)
		writeString(source.SessionKey)
		writeString(source.SessionID)
		writeString(source.ResolvedPath)
		writeString(strconv.FormatBool(source.Exists))
		writeString(strconv.FormatInt(source.Size, 10))
		writeString(strconv.FormatInt(source.ModifiedUnixNS, 10))
		writeString(source.EdgeFingerprint)
		writeString(source.FileIdentity)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func historyPageSourceDigestForCoverage(
	sources []historyPageSourceSnapshot,
	coverage string,
) string {
	if coverage != historyPageDisabledCoverageRoomLedger {
		return historyPageSourceDigest(sources)
	}
	for _, source := range sources {
		if source.Kind == historyPageSourceRoomLedger {
			return historyPageSourceDigest([]historyPageSourceSnapshot{source})
		}
	}
	return historyPageSourceDigest(nil)
}

func historyPageSourcesExceedLimit(sources []historyPageSourceSnapshot, limit int64) bool {
	if limit <= 0 {
		return false
	}
	total := int64(0)
	for _, source := range sources {
		if !source.Exists {
			continue
		}
		if source.Size < 0 || source.Size > limit-total {
			return true
		}
		total += source.Size
	}
	return false
}

func marshalHistoryPageJSONBounded(value any, limit int64) ([]byte, error) {
	if limit <= 0 || !historyPageJSONWithinLimit(reflect.ValueOf(value), limit, 0) {
		return nil, errHistoryPageIndexResourceLimit
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, errHistoryPageIndexResourceLimit
	}
	return payload, nil
}

// historyPageJSONWithinLimit 先以 JSON escaping 的保守上界审计值树，避免
// json.Marshal 在遇到单个 GB 级 Block 时先分配同量内存再发现超限。
func historyPageJSONWithinLimit(value reflect.Value, remaining int64, depth int) bool {
	if remaining < 0 || depth > 128 {
		return false
	}
	consume := func(amount int64) bool {
		if amount < 0 || amount > remaining {
			return false
		}
		remaining -= amount
		return true
	}
	var visit func(reflect.Value, int) bool
	visit = func(current reflect.Value, currentDepth int) bool {
		if currentDepth > 128 || !current.IsValid() {
			return consume(4)
		}
		for current.Kind() == reflect.Interface {
			if current.IsNil() {
				return consume(4)
			}
			current = current.Elem()
			currentDepth++
			if currentDepth > 128 {
				return false
			}
		}
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return consume(4)
			}
			if current.Type().Implements(historyPageJSONMarshalerType) ||
				current.Type().Implements(historyPageTextMarshalerType) {
				if current.Elem().Type() != historyPageTimeType {
					return false
				}
			}
			current = current.Elem()
			currentDepth++
			if currentDepth > 128 {
				return false
			}
		}
		if current.Type() == historyPageTimeType {
			return consume(64)
		}
		if current.CanInterface() {
			if raw, ok := current.Interface().(json.RawMessage); ok {
				length := int64(len(raw))
				if length > (remaining-2)/6 {
					return false
				}
				return consume(2 + length*6)
			}
			if _, ok := current.Interface().(json.Marshaler); ok {
				return false
			}
			if _, ok := current.Interface().(encoding.TextMarshaler); ok {
				return false
			}
		}
		switch current.Kind() {
		case reflect.Bool:
			return consume(5)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Uintptr, reflect.Float32, reflect.Float64:
			return consume(32)
		case reflect.String:
			length := int64(current.Len())
			if length > (remaining-2)/6 {
				return false
			}
			return consume(2 + length*6)
		case reflect.Slice, reflect.Array:
			if current.Kind() == reflect.Slice && current.IsNil() {
				return consume(4)
			}
			if !consume(2) {
				return false
			}
			for index := 0; index < current.Len(); index++ {
				if index > 0 && !consume(1) {
					return false
				}
				if !visit(current.Index(index), currentDepth+1) {
					return false
				}
			}
			return true
		case reflect.Map:
			if current.IsNil() {
				return consume(4)
			}
			if current.Type().Key().Kind() != reflect.String || !consume(2) {
				return false
			}
			iterator := current.MapRange()
			index := 0
			for iterator.Next() {
				if index > 0 && !consume(1) {
					return false
				}
				keyLength := int64(iterator.Key().Len())
				if keyLength > (remaining-3)/6 || !consume(3+keyLength*6) {
					return false
				}
				if !visit(iterator.Value(), currentDepth+1) {
					return false
				}
				index++
			}
			return true
		case reflect.Struct:
			if !consume(2) {
				return false
			}
			included := 0
			typeValue := current.Type()
			for index := 0; index < current.NumField(); index++ {
				field := typeValue.Field(index)
				if field.PkgPath != "" {
					continue
				}
				name := strings.Split(field.Tag.Get("json"), ",")[0]
				if name == "-" {
					continue
				}
				if name == "" {
					name = field.Name
				}
				if included > 0 && !consume(1) {
					return false
				}
				nameLength := int64(len(name))
				if nameLength > (remaining-3)/6 || !consume(3+nameLength*6) {
					return false
				}
				if !visit(current.Field(index), currentDepth+1) {
					return false
				}
				included++
			}
			return true
		default:
			return false
		}
	}
	return visit(value, depth)
}

func selectHistoryPageIndexGroups(
	groups []historyPageIndexGroup,
	request historyPageIndexRequest,
) historyPageIndexSelection {
	logicalGroups := buildVisibleHistoryPageLogicalGroups(
		groups,
		normalizeActiveRoundIDs(request.ActiveRoundIDs),
	)
	if len(logicalGroups) == 0 {
		return historyPageIndexSelection{Indexes: []int{}}
	}
	aroundRoundID := strings.TrimSpace(request.AroundRoundID)
	if aroundRoundID != "" {
		targetIndex := -1
		for index, group := range logicalGroups {
			if group.CursorRoundID == aroundRoundID {
				targetIndex = index
				break
			}
		}
		if targetIndex < 0 {
			return historyPageIndexSelection{Indexes: []int{}, HasMore: true}
		}
		radius := normalizeRoundAroundLimit(request.AroundLimit)
		start := max(0, targetIndex-radius)
		end := min(len(logicalGroups), targetIndex+radius+1)
		selection := historyPageIndexSelection{
			Indexes: flattenHistoryPageLogicalGroupIndexes(logicalGroups[start:end]),
			HasMore: start > 0 || end < len(logicalGroups),
		}
		if start > 0 {
			selection.NextBeforeRoundID = stringPointer(logicalGroups[start].CursorRoundID)
			timestamp := logicalGroups[start].CursorRoundTimestamp
			selection.NextBeforeRoundTimestamp = &timestamp
		}
		return selection
	}

	metadataGroups := make([]historyPageGroup, 0, len(logicalGroups))
	for _, group := range logicalGroups {
		metadataGroups = append(metadataGroups, historyPageGroup{
			CursorRoundID:        group.CursorRoundID,
			CursorRoundTimestamp: group.CursorRoundTimestamp,
		})
	}
	end := findHistoryPageEndGroupIndex(
		metadataGroups,
		strings.TrimSpace(request.BeforeRoundID),
		request.BeforeRoundTimestamp,
	)
	if end <= 0 {
		return historyPageIndexSelection{Indexes: []int{}}
	}
	start := max(0, end-normalizeRoundPageLimit(request.Limit))
	selection := historyPageIndexSelection{
		Indexes: flattenHistoryPageLogicalGroupIndexes(logicalGroups[start:end]),
		HasMore: start > 0,
	}
	if selection.HasMore {
		selection.NextBeforeRoundID = stringPointer(logicalGroups[start].CursorRoundID)
		timestamp := logicalGroups[start].CursorRoundTimestamp
		selection.NextBeforeRoundTimestamp = &timestamp
	}
	return selection
}

type historyPageLogicalGroup struct {
	GroupKey             string
	CursorRoundID        string
	CursorRoundTimestamp int64
	PhysicalIndexes      []int
}

func buildVisibleHistoryPageLogicalGroups(
	groups []historyPageIndexGroup,
	active map[string]struct{},
) []historyPageLogicalGroup {
	logical := make([]historyPageLogicalGroup, 0, len(groups))
	for index, group := range groups {
		firstTimestamp, visible := historyPageIndexGroupFirstVisibleTimestamp(group, active)
		if !visible {
			continue
		}
		if len(logical) > 0 && logical[len(logical)-1].GroupKey == group.GroupKey {
			logical[len(logical)-1].PhysicalIndexes = append(
				logical[len(logical)-1].PhysicalIndexes,
				index,
			)
			continue
		}
		logical = append(logical, historyPageLogicalGroup{
			GroupKey:             group.GroupKey,
			CursorRoundID:        group.CursorRoundID,
			CursorRoundTimestamp: firstTimestamp,
			PhysicalIndexes:      []int{index},
		})
	}
	return logical
}

func historyPageIndexGroupFirstVisibleTimestamp(
	group historyPageIndexGroup,
	active map[string]struct{},
) (int64, bool) {
	first := int64(0)
	visible := false
	if group.FirstNonSyntheticTimestamp != nil {
		first = *group.FirstNonSyntheticTimestamp
		visible = true
	}
	for _, synthetic := range group.SyntheticInterrupts {
		if _, hidden := active[synthetic.RoundID]; hidden {
			continue
		}
		if !visible || synthetic.Timestamp < first {
			first = synthetic.Timestamp
		}
		visible = true
	}
	return first, visible
}

func flattenHistoryPageLogicalGroupIndexes(groups []historyPageLogicalGroup) []int {
	count := 0
	for _, group := range groups {
		count += len(group.PhysicalIndexes)
	}
	result := make([]int, 0, count)
	for _, group := range groups {
		result = append(result, group.PhysicalIndexes...)
	}
	return result
}

func paginateHistoryPageIndexedGroups(
	groups []historyPageIndexedGroup,
	request historyPageIndexRequest,
) protocol.MessagePage {
	metadata := make([]historyPageIndexGroup, 0, len(groups))
	for _, group := range groups {
		metadata = append(metadata, historyPageIndexMetadataForGroup(group))
	}
	selection := selectHistoryPageIndexGroups(metadata, request)
	selected := make([]historyPageIndexedGroup, 0, len(selection.Indexes))
	for _, index := range selection.Indexes {
		selected = append(selected, groups[index])
	}
	return materializeHistoryPageSelection(selected, selection, request)
}

func materializeHistoryPageSelection(
	groups []historyPageIndexedGroup,
	selection historyPageIndexSelection,
	request historyPageIndexRequest,
) protocol.MessagePage {
	active := normalizeActiveRoundIDs(request.ActiveRoundIDs)
	items := make([]protocol.Message, 0)
	for _, group := range groups {
		for _, row := range group.Items {
			if indexedSyntheticInterruptIsActive(row, active) {
				continue
			}
			items = append(items, normalizeHistoryPageRow(row, request.CollapseRoomRounds))
		}
	}
	return protocol.MessagePage{
		Items:                    items,
		HasMore:                  selection.HasMore,
		NextBeforeRoundID:        selection.NextBeforeRoundID,
		NextBeforeRoundTimestamp: selection.NextBeforeRoundTimestamp,
	}
}

func indexedSyntheticInterruptIsActive(
	row protocol.Message,
	active map[string]struct{},
) bool {
	if len(active) == 0 {
		return false
	}
	roundID := indexedSyntheticInterruptRoundID(row)
	if roundID == "" {
		return false
	}
	_, ok := active[roundID]
	return ok
}

func indexedSyntheticInterruptRoundID(row protocol.Message) string {
	roundID := stringFromAny(row["round_id"])
	if roundID == "" || stringFromAny(row["message_id"]) != "assistant_interrupt_"+roundID {
		return ""
	}
	return roundID
}

func buildHistoryPageIndexedGroups(
	ctx context.Context,
	rows []protocol.Message,
	collapseRoomRounds bool,
) ([]historyPageIndexedGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 索引缓存 inactive 基准投影，而不是 materialize 前的半成品。分页 group
	// 必须与 legacy oracle 在全局 normalize 后完全同构；active 的唯一差异是
	// 是否移除可识别的 synthetic interrupt，可在 selected groups 内确定性完成。
	normalized := normalizeHistoryRows(rows, nil)

	groups := make([]historyPageIndexedGroup, 0)
	currentKey := ""
	for _, row := range normalized {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := historyPageGroupKey(row, collapseRoomRounds)
		if key == "" {
			continue
		}
		if key != currentKey {
			currentKey = key
			groups = append(groups, historyPageIndexedGroup{
				CursorRoundID:        historyPageCursorRoundID(row, collapseRoomRounds),
				CursorRoundTimestamp: messageTimestamp(row),
				GroupKey:             key,
				Items:                make([]protocol.Message, 0, 1),
			})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, row)
	}
	return groups, nil
}

func historyPageScope(parts ...string) string {
	hasher := sha256.New()
	for _, part := range parts {
		_, _ = io.WriteString(hasher, fmt.Sprintf("%d:%s|", len(part), part))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func snapshotsEqual(left []historyPageSourceSnapshot, right []historyPageSourceSnapshot) bool {
	return slices.Equal(left, right)
}

func snapshotFileAtRoot(
	ctx context.Context,
	root *confinedfs.Root,
	relative string,
	base historyPageSourceSnapshot,
) (historyPageSourceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return historyPageSourceSnapshot{}, err
	}
	info, err := root.Lstat(relative)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return historyPageSourceSnapshot{}, errors.New("history source is not a regular file")
	}
	file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer file.Close()
	return snapshotOpenedFile(ctx, file, info, base)
}

func snapshotOpenedFile(
	ctx context.Context,
	file *os.File,
	info os.FileInfo,
	base historyPageSourceSnapshot,
) (historyPageSourceSnapshot, error) {
	fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, info.Size())
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	base.Exists = true
	base.Size = info.Size()
	base.ModifiedUnixNS = info.ModTime().UnixNano()
	base.EdgeFingerprint = fingerprint
	base.FileIdentity = historyPageFileIdentity(file, info)
	return base, nil
}

func historyPageFileEdgeFingerprint(ctx context.Context, file *os.File, size int64) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, _ = io.WriteString(hasher, fmt.Sprintf("%d|", size))
	readRange := func(offset int64, length int64) error {
		if length <= 0 {
			return nil
		}
		buffer := make([]byte, int(length))
		read, err := file.ReadAt(buffer, offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		_, _ = hasher.Write(buffer[:read])
		return ctx.Err()
	}
	if size <= historyPageEdgeHashBytes*2 {
		if err := readRange(0, size); err != nil {
			return "", err
		}
	} else {
		if err := readRange(0, historyPageEdgeHashBytes); err != nil {
			return "", err
		}
		if err := readRange(size-historyPageEdgeHashBytes, historyPageEdgeHashBytes); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func sortHistoryPageSources(sources []historyPageSourceSnapshot) {
	sort.Slice(sources, func(left int, right int) bool {
		leftKey := historyPageSourceKey(sources[left])
		rightKey := historyPageSourceKey(sources[right])
		return leftKey < rightKey
	})
}

func historyPageSourceKey(source historyPageSourceSnapshot) string {
	return strings.Join([]string{
		source.Kind,
		source.WorkspacePath,
		source.SessionKey,
		source.SessionID,
		source.ResolvedPath,
	}, "\x00")
}

func readWorkspaceJSONLAtContext(
	ctx context.Context,
	store *SessionFileStore,
	workspacePath string,
	target string,
) ([]map[string]any, error) {
	if ownerUserID := strings.TrimSpace(store.ownerUserID); ownerUserID != "" {
		parent, name, err := store.openOwnerWorkspaceFileParent(
			ownerUserID,
			workspacePath,
			target,
			false,
		)
		if err != nil {
			return nil, err
		}
		defer parent.Close()
		file, err := parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return readJSONLFileWithContext(ctx, file)
	}
	root, relative, err := relativeStorePathWithCreate(workspacePath, target, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readJSONLFileWithContext(ctx, file)
}

func readRoomJSONLWithContext(
	ctx context.Context,
	store *SessionFileStore,
	ownerUserID string,
	target string,
) ([]map[string]any, error) {
	parent, name, err := store.openRoomFileParent(ownerUserID, target, false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	file, err := parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readJSONLFileWithContext(ctx, file)
}

func readJSONLFileWithContext(ctx context.Context, file *os.File) ([]map[string]any, error) {
	return readJSONLReaderWithContext(ctx, file)
}

func readJSONLReaderWithContext(ctx context.Context, source io.Reader) ([]map[string]any, error) {
	reader := bufio.NewScanner(source)
	reader.Buffer(make([]byte, 0, 64*1024), transcriptScannerBufferBytes)
	rows := make([]map[string]any, 0)
	for reader.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		normalized, ok := normalizeDecodedJSONValue(item).(map[string]any)
		if ok {
			rows = append(rows, normalized)
		}
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	return rows, ctx.Err()
}
