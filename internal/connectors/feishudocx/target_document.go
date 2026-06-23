package feishudocx

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseDocumentTarget(raw string) (DocumentTarget, error) {
	value := strings.TrimSpace(raw)
	target := DocumentTarget{Raw: value}
	if value == "" {
		return target, fmt.Errorf("飞书文档 URL 或 document_id 不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if strings.Contains(value, "/") {
			return target, fmt.Errorf("飞书文档 URL 格式不正确")
		}
		target.DocumentID = value
		target.SourceType = "docx"
		return target, nil
	}
	segments := splitPath(parsed.Path)
	for index, segment := range segments {
		if index+1 >= len(segments) {
			continue
		}
		switch segment {
		case "docx":
			target.DocumentID = segments[index+1]
			target.SourceType = "docx"
			return target, nil
		case "wiki":
			target.WikiToken = segments[index+1]
			target.SourceType = "wiki"
			return target, nil
		}
	}
	return target, fmt.Errorf("暂只支持飞书 docx/wiki 链接")
}
