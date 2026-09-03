// INPUT: Connector 服务端静态目录及已移除的占位产品标识。
// OUTPUT: 证明运行时只发布真实接入产品，未上线产品没有详情入口。
// POS: Connector Catalog 上线边界的服务端回归合同。
package connectors

import (
	"slices"
	"testing"
)

func TestCatalogContainsOnlyImplementedConnectors(t *testing.T) {
	wantIDs := []string{
		"amap",
		"didi",
		"dingtalk-ai-table",
		"feishu-docx",
		"github",
		"richmail",
		"tencent-docs",
		"yuque",
	}
	gotIDs := make([]string, 0, len(connectorCatalog))
	for _, entry := range connectorCatalog {
		if entry.Status != "available" {
			t.Fatalf("未实现 Connector 不得进入运行时目录: id=%q status=%q", entry.ConnectorID, entry.Status)
		}
		gotIDs = append(gotIDs, entry.ConnectorID)
	}
	slices.Sort(gotIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("运行时目录应只包含已接入 Connector: got=%v want=%v", gotIDs, wantIDs)
	}
}

func TestRemovedPlaceholderConnectorsHaveNoDetail(t *testing.T) {
	for _, connectorID := range []string{
		"gmail",
		"google-calendar",
		"google-drive",
		"outlook",
		"x-twitter",
	} {
		if _, ok := getConnector(connectorID); ok {
			t.Fatalf("未上线 Connector 不应保留详情入口: %q", connectorID)
		}
	}
}
