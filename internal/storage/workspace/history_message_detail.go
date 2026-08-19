// INPUT: SQLite history generation 中的大型 Tool result / 内联图片引用。
// OUTPUT: 消息页中的有界预览与 owner/session 校验后的按需完整 detail。
// POS: canonical 历史不变的派生大内容读取层；引用只在当前 source generation 内有效。
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	historyMessageDetailThresholdBytes = 256 * 1024
	historyMessageDetailPreviewBytes   = 16 * 1024
	historyMessageDetailMaxBytes       = 64 * 1024 * 1024
	historyMessageDetailKindToolResult = "tool_result"
	historyMessageDetailKindImage      = "image"
)

// ErrHistoryMessageDetailUnavailable 表示引用已过期、不属于当前 Session 或内容损坏。
var ErrHistoryMessageDetailUnavailable = errors.New("history message detail is unavailable")

// HistoryMessageDetail 是 handler 按 JSON 或图片字节流返回的完整派生内容。
type HistoryMessageDetail struct {
	Ref       string
	Kind      string
	MediaType string
	ByteSize  int64
	Payload   []byte
}

type historyReadModelDetail struct {
	Ref       string
	Kind      string
	MediaType string
	Payload   []byte
	Digest    string
}

type historyDetailProjectionContext struct {
	scope      string
	generation string
	sequence   int
	messageID  string
	path       string
}

func projectHistoryReadModelGroup(
	scope string,
	generation string,
	sequence int,
	group historyPageIndexedGroup,
) (historyPageIndexedGroup, []historyReadModelDetail, error) {
	projected := group
	projected.Items = make([]protocol.Message, 0, len(group.Items))
	details := make([]historyReadModelDetail, 0)
	for rowIndex, row := range group.Items {
		cloned := protocol.Clone(row)
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		content, extracted, err := projectHistoryDetailValue(
			row["content"],
			historyDetailProjectionContext{
				scope:      scope,
				generation: generation,
				sequence:   sequence,
				messageID:  messageID,
				path:       fmt.Sprintf("items/%d/content", rowIndex),
			},
		)
		if err != nil {
			return historyPageIndexedGroup{}, nil, err
		}
		cloned["content"] = content
		projected.Items = append(projected.Items, cloned)
		details = append(details, extracted...)
	}
	return projected, details, nil
}

func projectHistoryDetailValue(
	value any,
	ctx historyDetailProjectionContext,
) (any, []historyReadModelDetail, error) {
	switch typed := value.(type) {
	case []any:
		result := make([]any, len(typed))
		details := make([]historyReadModelDetail, 0)
		for index, item := range typed {
			projected, extracted, err := projectHistoryDetailValue(
				item,
				withHistoryDetailPath(ctx, fmt.Sprintf("%s/%d", ctx.path, index)),
			)
			if err != nil {
				return nil, nil, err
			}
			result[index] = projected
			details = append(details, extracted...)
		}
		return result, details, nil
	case []map[string]any:
		items := make([]any, len(typed))
		for index := range typed {
			items[index] = typed[index]
		}
		return projectHistoryDetailValue(items, ctx)
	case map[string]any:
		return projectHistoryDetailMap(typed, ctx)
	default:
		return value, nil, nil
	}
}

func projectHistoryDetailMap(
	value map[string]any,
	ctx historyDetailProjectionContext,
) (map[string]any, []historyReadModelDetail, error) {
	blockType := strings.TrimSpace(stringFromAny(value["type"]))
	if blockType == "tool_result" {
		payload, err := json.Marshal(value["content"])
		if err != nil {
			return nil, nil, err
		}
		if len(payload) >= historyMessageDetailThresholdBytes {
			detail, err := newHistoryReadModelDetail(
				ctx,
				historyMessageDetailKindToolResult,
				"application/json",
				payload,
			)
			if err != nil {
				return nil, nil, err
			}
			projected := cloneHistoryDetailMap(value)
			projected["content"] = historyToolResultPreview(value["content"])
			attachHistoryDetailReference(projected, detail)
			return projected, []historyReadModelDetail{detail}, nil
		}
	}
	if blockType == "image" {
		payload, mediaType, ok := decodeHistoryInlineImage(value)
		if ok && len(payload) >= historyMessageDetailThresholdBytes {
			detail, err := newHistoryReadModelDetail(
				ctx,
				historyMessageDetailKindImage,
				mediaType,
				payload,
			)
			if err != nil {
				return nil, nil, err
			}
			projected := cloneHistoryDetailMap(value)
			delete(projected, "data")
			if source, sourceOK := projected["source"].(map[string]any); sourceOK {
				source = cloneHistoryDetailMap(source)
				delete(source, "data")
				projected["source"] = source
			}
			attachHistoryDetailReference(projected, detail)
			return projected, []historyReadModelDetail{detail}, nil
		}
	}

	projected := cloneHistoryDetailMap(value)
	details := make([]historyReadModelDetail, 0)
	for key, item := range value {
		if key != "content" && key != "source" {
			continue
		}
		nested, extracted, err := projectHistoryDetailValue(
			item,
			withHistoryDetailPath(ctx, ctx.path+"/"+key),
		)
		if err != nil {
			return nil, nil, err
		}
		projected[key] = nested
		details = append(details, extracted...)
	}
	return projected, details, nil
}

func newHistoryReadModelDetail(
	ctx historyDetailProjectionContext,
	kind string,
	mediaType string,
	payload []byte,
) (historyReadModelDetail, error) {
	if len(payload) == 0 || len(payload) > historyMessageDetailMaxBytes {
		return historyReadModelDetail{}, ErrHistoryMessageDetailUnavailable
	}
	payloadDigest := sha256.Sum256(payload)
	identity := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%s\x00%s\x00%s",
		ctx.scope,
		ctx.generation,
		ctx.sequence,
		ctx.messageID,
		ctx.path,
		hex.EncodeToString(payloadDigest[:]),
	)
	refDigest := sha256.Sum256([]byte(identity))
	return historyReadModelDetail{
		Ref:       hex.EncodeToString(refDigest[:16]),
		Kind:      kind,
		MediaType: mediaType,
		Payload:   payload,
		Digest:    hex.EncodeToString(payloadDigest[:]),
	}, nil
}

func attachHistoryDetailReference(
	block map[string]any,
	detail historyReadModelDetail,
) {
	block["detail_ref"] = detail.Ref
	block["detail_kind"] = detail.Kind
	block["detail_size"] = len(detail.Payload)
	block["detail_truncated"] = true
}

func historyToolResultPreview(value any) any {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	if len(text) <= historyMessageDetailPreviewBytes {
		return text
	}
	end := historyMessageDetailPreviewBytes
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	return text[:end]
}

func decodeHistoryInlineImage(block map[string]any) ([]byte, string, bool) {
	data := strings.TrimSpace(stringFromAny(block["data"]))
	mediaType := strings.TrimSpace(stringFromAny(block["mime_type"]))
	if source, ok := block["source"].(map[string]any); ok {
		if data == "" {
			data = strings.TrimSpace(stringFromAny(source["data"]))
		}
		if mediaType == "" {
			mediaType = strings.TrimSpace(stringFromAny(source["mime_type"]))
		}
		if mediaType == "" {
			mediaType = strings.TrimSpace(stringFromAny(source["media_type"]))
		}
	}
	if strings.HasPrefix(strings.ToLower(data), "data:") {
		header, encoded, found := strings.Cut(data, ",")
		if !found || !strings.Contains(strings.ToLower(header), ";base64") {
			return nil, "", false
		}
		if mediaType == "" {
			mediaType = strings.TrimSpace(strings.TrimSuffix(header[5:], ";base64"))
		}
		data = encoded
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		if unescaped, unescapeErr := url.QueryUnescape(data); unescapeErr == nil {
			decoded, err = base64.StdEncoding.DecodeString(unescaped)
		}
	}
	if err != nil || len(decoded) == 0 {
		return nil, "", false
	}
	if mediaType == "" {
		mediaType = "image/png"
	}
	return decoded, mediaType, true
}

func cloneHistoryDetailMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func withHistoryDetailPath(
	ctx historyDetailProjectionContext,
	path string,
) historyDetailProjectionContext {
	ctx.path = path
	return ctx
}

func historyPageBuildHasLargeDetails(built historyPageIndexBuild) bool {
	for _, group := range built.Groups {
		for _, row := range group.Items {
			found, err := historyValueHasLargeDetail(row["content"])
			if err == nil && found {
				return true
			}
		}
	}
	return false
}

func historyValueHasLargeDetail(value any) (bool, error) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if found, err := historyValueHasLargeDetail(item); err != nil || found {
				return found, err
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if found, err := historyValueHasLargeDetail(item); err != nil || found {
				return found, err
			}
		}
	case map[string]any:
		blockType := strings.TrimSpace(stringFromAny(typed["type"]))
		if blockType == "tool_result" {
			payload, err := json.Marshal(typed["content"])
			if err != nil || len(payload) >= historyMessageDetailThresholdBytes {
				return len(payload) >= historyMessageDetailThresholdBytes, err
			}
		}
		if blockType == "image" {
			payload, _, ok := decodeHistoryInlineImage(typed)
			if ok && len(payload) >= historyMessageDetailThresholdBytes {
				return true, nil
			}
		}
		for _, key := range []string{"content", "source"} {
			if found, err := historyValueHasLargeDetail(typed[key]); err != nil || found {
				return found, err
			}
		}
	}
	return false, nil
}

func (m *historyReadModel) loadDetail(
	ctx context.Context,
	access historyPageIndexAccess,
	ref string,
) (HistoryMessageDetail, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	db, err := m.database(ctx)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	defer tx.Rollback()
	scope, ok, err := readHistoryReadModelScope(ctx, tx, access.Scope)
	if err != nil || !ok || scope.DisabledReason != "" {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	valid, err := access.ValidateSources(ctx, scope.Sources)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	if !valid {
		_ = tx.Rollback()
		_ = m.deleteScope(ctx, access.Scope)
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	var detail HistoryMessageDetail
	var digest string
	err = tx.QueryRowContext(
		ctx,
		`SELECT kind, media_type, byte_size, payload, payload_digest
		 FROM history_read_details
		 WHERE scope = ? AND generation = ? AND detail_ref = ?`,
		access.Scope,
		scope.Generation,
		ref,
	).Scan(&detail.Kind, &detail.MediaType, &detail.ByteSize, &detail.Payload, &digest)
	if err != nil {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	actual := sha256.Sum256(detail.Payload)
	if detail.ByteSize != int64(len(detail.Payload)) ||
		detail.ByteSize <= 0 || detail.ByteSize > historyMessageDetailMaxBytes ||
		hex.EncodeToString(actual[:]) != digest {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	detail.Ref = ref
	if err = tx.Commit(); err != nil {
		return HistoryMessageDetail{}, err
	}
	return detail, nil
}
