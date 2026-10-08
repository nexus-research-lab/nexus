package textutil

import "testing"

func TestFirstNonEmptyReturnsFirstTrimmedValue(t *testing.T) {
	if got := FirstNonEmpty("", "  ", " b ", "c"); got != "b" {
		t.Fatalf("FirstNonEmpty = %q", got)
	}
	if got := FirstNonEmpty(" ", ""); got != "" {
		t.Fatalf("FirstNonEmpty(blank) = %q", got)
	}
}

func TestPointerValueAndAnyStringTrim(t *testing.T) {
	value := " x "
	if PointerValue(nil) != "" || PointerValue(&value) != "x" {
		t.Fatal("PointerValue did not trim or handle nil")
	}
	if AnyString(" y ") != "y" || AnyString(3) != "" || AnyString(nil) != "" {
		t.Fatal("AnyString did not trim or reject non-string")
	}
}
