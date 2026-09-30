package visualize

import (
	"context"
	"strings"
	"testing"
)

func TestShowWidgetAcceptsBoundedFragment(t *testing.T) {
	result, err := showWidget(context.Background(), map[string]any{
		"title":       "照片",
		"widget_code": `<section><img src="data:image/jpeg;base64,AAAA" loading="eager"></section>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("bounded widget rejected: %+v", result)
	}
}

func TestShowWidgetRejectsOversizedFragment(t *testing.T) {
	result, err := showWidget(context.Background(), map[string]any{
		"title":       "照片",
		"widget_code": strings.Repeat("x", MaxWidgetCodeBytes+1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("oversized widget was accepted")
	}
}

func TestShowWidgetRejectsOversizedInlineImages(t *testing.T) {
	widget := `<section><img src="data:image/jpeg;base64,` +
		strings.Repeat("A", MaxInlineImageBytes) + `"></section>`
	result, err := showWidget(context.Background(), map[string]any{
		"title":       "照片",
		"widget_code": widget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("oversized inline image payload was accepted")
	}
}
