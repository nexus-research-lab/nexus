// INPUT: canonical history 中不可信的图片 MIME 声明。
// OUTPUT: detail 响应只保留安全 raster MIME，其他类型降级为 octet-stream。
// POS: 大图片 HTTP 响应的内容类型安全回归测试。
package agent

import "testing"

func TestSafeMessageDetailImageMediaType(t *testing.T) {
	if got := safeMessageDetailImageMediaType(" IMAGE/PNG "); got != "image/png" {
		t.Fatalf("png MIME = %q", got)
	}
	for _, value := range []string{"image/svg+xml", "text/html", "image/png\r\nX-Test: bad"} {
		if got := safeMessageDetailImageMediaType(value); got != "application/octet-stream" {
			t.Fatalf("unsafe MIME %q = %q", value, got)
		}
	}
}
