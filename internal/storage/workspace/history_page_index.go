// INPUT: 规范化后的历史行、round 分页游标与来源文件。
// OUTPUT: 按根轮次分组与分页选择（读模型与 canonical 分页共用），以及来源文件的身份快照。
// POS: workspace 历史分页的纯计算层，不读写任何派生库。
package workspace

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	historyPageEdgeHashBytes = 4 * 1024
)

const (
	historyPageSourceRoomLedger = "room_ledger"
)

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

type historyPageIndexRequest struct {
	Limit                int
	BeforeRoundID        string
	BeforeRoundTimestamp int64
	AroundRoundID        string
	AroundLimit          int
	CollapseRoomRounds   bool
	ActiveRoundIDs       []string
}

type historyPageIndexSelection struct {
	Indexes                  []int
	HasMore                  bool
	NextBeforeRoundID        *string
	NextBeforeRoundTimestamp *int64
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

// openWorkspaceFileAt 以只读方式打开 workspace 内的受限文件；调用方负责关闭。
func openWorkspaceFileAt(store *SessionFileStore, workspacePath string, target string) (*os.File, error) {
	if ownerUserID := store.ownerUserID; ownerUserID != "" {
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
		return parent.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	}
	root, relative, err := relativeStorePathWithCreate(workspacePath, target, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
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
