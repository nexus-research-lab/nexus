package management

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

func TestLocalizeChannelConfigViewCoversEveryCatalogText(t *testing.T) {
	for _, item := range ChannelCatalog() {
		view := LocalizeChannelConfigView(ChannelConfigView{ChannelCatalogItem: item}, "en")
		texts := map[string]string{
			"title":        view.Title,
			"bot_label":    view.BotLabel,
			"description":  view.Description,
			"runtime_note": view.RuntimeNote,
		}
		for _, field := range view.CredentialFields {
			texts[field.Key+" label"] = field.Label
			texts[field.Key+" placeholder"] = field.Placeholder
		}
		for name, text := range texts {
			if text == "" || hasHan(text) {
				t.Errorf("%s %s 缺少英文文案: %q", item.ChannelType, name, text)
			}
		}
		if len(view.CredentialFields) != len(item.CredentialFields) {
			t.Fatalf("%s 凭据字段数量变化: %d != %d", item.ChannelType, len(view.CredentialFields), len(item.CredentialFields))
		}
		for index, field := range view.CredentialFields {
			want := item.CredentialFields[index]
			want.Label, want.Placeholder = field.Label, field.Placeholder
			if field != want {
				t.Errorf("%s 字段 %s 的非文案属性被改写: %+v", item.ChannelType, field.Key, field)
			}
		}
	}
}

func TestLocalizeChannelConfigViewKeepsChineseCatalogByDefault(t *testing.T) {
	for _, locale := range []string{"", "zh", "zh-CN", "enigma"} {
		for _, item := range ChannelCatalog() {
			view := ChannelConfigView{ChannelCatalogItem: item}
			if got := LocalizeChannelConfigView(view, locale); !reflect.DeepEqual(got, view) {
				t.Errorf("locale %q 改写了 %s 的中文目录", locale, item.ChannelType)
			}
		}
	}
}
