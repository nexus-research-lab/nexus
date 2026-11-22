package connectors

import (
	"reflect"
	"testing"
	"unicode"
)

func hasHan(text string) bool {
	for _, char := range text {
		if unicode.Is(unicode.Han, char) {
			return true
		}
	}
	return false
}

func catalogDetail(entry CatalogEntry) Detail {
	return Detail{
		Info:           Info{ConnectorID: entry.ConnectorID, Kind: ConnectorKindCatalog, Title: entry.Title, Description: entry.Description},
		Scopes:         entry.Scopes,
		Features:       entry.Features,
		FeatureDetails: connectorFeatureDetailsFor(entry),
	}
}

func TestLocalizeDetailCoversEveryCatalogText(t *testing.T) {
	for _, entry := range connectorCatalog {
		source := catalogDetail(entry)
		detail := LocalizeDetail(source, "en")
		texts := map[string]string{"title": detail.Title, "description": detail.Description}
		for _, feature := range detail.Features {
			texts["feature "+feature] = feature
		}
		for _, feature := range detail.FeatureDetails {
			texts[feature.Name+" description"] = feature.Description
			for _, item := range feature.Items {
				texts[feature.Name+" item "+item] = item
			}
		}
		for name, text := range texts {
			if text == "" || hasHan(text) {
				t.Errorf("%s %s 缺少英文文案: %q", entry.ConnectorID, name, text)
			}
		}
		if len(detail.FeatureDetails) != len(source.FeatureDetails) {
			t.Fatalf("%s 能力数量变化: %d != %d", entry.ConnectorID, len(detail.FeatureDetails), len(source.FeatureDetails))
		}
		for index, feature := range detail.FeatureDetails {
			if feature.Name != detail.Features[index] {
				t.Errorf("%s 能力名与详情不一致: %q != %q", entry.ConnectorID, detail.Features[index], feature.Name)
			}
			if len(feature.Items) != len(source.FeatureDetails[index].Items) {
				t.Errorf("%s 能力 %s 的要点数量变化", entry.ConnectorID, feature.Name)
			}
			if !reflect.DeepEqual(feature.Scopes, source.FeatureDetails[index].Scopes) {
				t.Errorf("%s 能力 %s 的授权范围被改写", entry.ConnectorID, feature.Name)
			}
		}
		if info := LocalizeInfo(source.Info, "en"); !reflect.DeepEqual(info, detail.Info) {
			t.Errorf("%s 目录项与详情文案不一致", entry.ConnectorID)
		}
	}
}

func TestLocalizeKeepsChineseCatalogAndCustomMCP(t *testing.T) {
	for _, locale := range []string{"", "zh", "zh-CN"} {
		for _, entry := range connectorCatalog {
			source := catalogDetail(entry)
			if got := LocalizeDetail(source, locale); !reflect.DeepEqual(got, source) {
				t.Errorf("locale %q 改写了 %s 的中文目录", locale, entry.ConnectorID)
			}
		}
	}
	custom := Info{ConnectorID: "github", Kind: ConnectorKindCustomMCP, Title: "我的工具", Description: "自定义说明"}
	if got := LocalizeInfo(custom, "en"); !reflect.DeepEqual(got, custom) {
		t.Errorf("自定义 MCP 文案被改写: %+v", got)
	}
}
