// INPUT: Handler 内部固定的资源前缀、资源名称和强 If-Match 请求头。
// OUTPUT: no-store 强 ETag，或通过严格校验的正版本号。
// POS: Handler 共享的 HTTP 条件写原语；不解释领域冲突或写入结果。
package shared

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// WriteStrongETag 写入只用于条件写的强 ETag，并禁止中间缓存。
func WriteStrongETag(writer http.ResponseWriter, prefix string, version int64) {
	if writer == nil || version < 1 {
		return
	}
	writer.Header().Set("ETag", fmt.Sprintf(`"%s%d"`, prefix, version))
	writer.Header().Set("Cache-Control", "no-store")
}

// ParseStrongIfMatch 只接受属于指定资源的单个强 ETag。
func ParseStrongIfMatch(
	value string,
	prefix string,
	resource string,
	scope string,
) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, ",") || value == "*" || strings.HasPrefix(value, "W/") {
		return nil, fmt.Errorf("%s If-Match 必须是单个强 ETag", resource)
	}
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return nil, fmt.Errorf("%s If-Match 缺少强 ETag 引号", resource)
	}
	opaque := strings.TrimSpace(value[1 : len(value)-1])
	if !strings.HasPrefix(opaque, prefix) {
		return nil, fmt.Errorf("%s If-Match 不属于%s", resource, scope)
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(opaque, prefix), 10, 64)
	if err != nil || version < 1 {
		return nil, errors.New(resource + " If-Match version 无效")
	}
	return &version, nil
}
