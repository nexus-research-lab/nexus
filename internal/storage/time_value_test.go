package storage

import (
	"testing"
	"time"
)

func TestNullableTimeSupportsDriverAndSQLiteAggregateValues(t *testing.T) {
	want := time.Date(2026, 8, 17, 12, 34, 56, 123456789, time.UTC)
	values := []any{
		want,
		want.Format(time.RFC3339Nano),
		[]byte(want.Format("2006-01-02 15:04:05.999999999-07:00")),
	}
	for _, value := range values {
		got, err := NullableTime(value)
		if err != nil {
			t.Fatalf("NullableTime(%T): %v", value, err)
		}
		if got == nil || !got.Equal(want) {
			t.Fatalf("NullableTime(%T) = %v, want %v", value, got, want)
		}
	}
	got, err := NullableTime(nil)
	if err != nil || got != nil {
		t.Fatalf("NullableTime(nil) = %v, %v", got, err)
	}
}
