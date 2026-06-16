package config

import "testing"

func TestParseIntEnv(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback int
		want     int
	}{
		{"valid int", "42", 0, 42},
		{"valid zero", "0", 99, 0},
		{"valid negative", "-1", 0, -1},
		{"invalid string returns fallback", "abc", 10, 10},
		{"empty string returns fallback", "", 24, 24},
		{"float string returns fallback", "3.14", 7, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIntEnv(tt.raw, tt.fallback)
			if got != tt.want {
				t.Errorf("parseIntEnv(%q, %d) = %d, want %d", tt.raw, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestMustBool(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"true", "true", true},
		{"false", "false", false},
		{"1", "1", true},
		{"0", "0", false},
		{"invalid returns false", "yes", false},
		{"empty returns false", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustBool(tt.raw)
			if got != tt.want {
				t.Errorf("mustBool(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMustFloat(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want float64
	}{
		{"valid float", "3.14", 3.14},
		{"valid int as float", "42", 42.0},
		{"zero", "0", 0},
		{"invalid returns zero", "abc", 0},
		{"empty returns zero", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustFloat(tt.raw)
			if got != tt.want {
				t.Errorf("mustFloat(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMustStringList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"single", "a", []string{"a"}},
		{"multiple", "a, b, c", []string{"a", "b", "c"}},
		{"with empty parts", "a, , b", []string{"a", "b"}},
		{"empty string", "", []string{}},
		{"only spaces", "  ,  ", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustStringList(tt.raw)
			if len(got) != len(tt.want) {
				t.Errorf("mustStringList(%q) = %v, want %v", tt.raw, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mustStringList(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}
