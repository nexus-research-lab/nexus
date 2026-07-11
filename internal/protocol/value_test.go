package protocol

import (
	"encoding/json"
	"math"
	"testing"
)

func TestInt64FromAny(t *testing.T) {
	type count uint32
	tests := []struct {
		name  string
		value any
		want  int64
	}{
		{name: "signed", value: int16(-12), want: -12},
		{name: "unsigned alias", value: count(23), want: 23},
		{name: "float", value: 34.9, want: 34},
		{name: "json integer", value: json.Number("45"), want: 45},
		{name: "json decimal", value: json.Number("56.7"), want: 56},
		{name: "string", value: " 67 ", want: 67},
		{name: "unsigned overflow", value: uint64(math.MaxUint64), want: 0},
		{name: "infinite", value: math.Inf(1), want: 0},
		{name: "invalid", value: "not-a-number", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Int64FromAny(test.value); got != test.want {
				t.Fatalf("Int64FromAny(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}
