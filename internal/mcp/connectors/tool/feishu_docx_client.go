package tool

import (
	"context"
	"errors"
	"strings"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	feishudocxapi "github.com/nexus-research-lab/nexus/internal/connectors/feishudocx"
	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
)

func loadFeishuDocxClient(ctx context.Context, svc contract.Service, sctx contract.ServerContext) (*feishudocxapi.Client, error) {
	snapshot, err := svc.LoadActiveConnection(ctx, sctx.OwnerUserID, "feishu-docx")
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("飞书云文档连接器未连接")
	}
	return feishuDocxClientFromSnapshot(snapshot), nil
}

func feishuDocxClientFromSnapshot(snapshot *connectordomain.ConnectionSnapshot) *feishudocxapi.Client {
	return feishudocxapi.NewClient(snapshot.APIBaseURL, snapshot.AccessToken, connectorCallHTTPClient)
}

func filterDriveFilesByType(files []map[string]any, fileType string) []map[string]any {
	fileType = strings.TrimSpace(fileType)
	if fileType == "" {
		return files
	}
	result := make([]map[string]any, 0, len(files))
	for _, item := range files {
		if strings.TrimSpace(stringValue(item["type"])) == fileType {
			result = append(result, item)
		}
	}
	return result
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, strings.TrimSpace(text))
			}
		}
		return result
	default:
		return nil
	}
}
