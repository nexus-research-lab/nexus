// INPUT: SQLite history generation 中的大型 Tool result / 内联图片引用。
// OUTPUT: 消息页中的有界预览与 owner/session 校验后的按需完整 detail。
// POS: canonical 历史不变的派生大内容读取层；引用只在当前 source generation 内有效。
package workspace

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
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

// 同一组中的多个中等输出共用预览预算，索引冷建和热读采用同一判定。
type historyToolResultBudget int

func (b *historyToolResultBudget) reserve(size int) (inline bool, previewBytes int) {
	if size < historyMessageDetailThresholdBytes && size <= int(*b) {
		*b -= historyToolResultBudget(size)
		return true, 0
	}
	previewBytes = min(int(*b), historyMessageDetailPreviewBytes)
	*b -= historyToolResultBudget(previewBytes)
	return false, previewBytes
}

type historyDetailProjectionContext struct {
	toolBudget *historyToolResultBudget
	scope      string
	generation string
	sequence   int
	messageID  string
	path       string
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
		if inline, previewBytes := ctx.toolBudget.reserve(len(payload)); !inline {
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
			projected["content"] = historyToolResultPreview(value["content"], previewBytes)
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
	for _, key := range []string{"content", "source"} {
		item, exists := value[key]
		if !exists {
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

func historyToolResultPreview(value any, limit int) any {
	text, ok := value.(string)
	if !ok || limit == 0 {
		return nil
	}
	if len(text) <= limit {
		return text
	}
	end := limit
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
