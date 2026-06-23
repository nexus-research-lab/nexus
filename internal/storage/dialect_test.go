package storage

import "testing"

func TestSQLDialectBindList(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		firstBind  string
		threeBinds string
		timestamp  string
		jsonText   string
		jsonValue  string
		insert     string
		suffix     string
	}{
		{
			name:       "postgres",
			driver:     "postgres",
			firstBind:  "$1",
			threeBinds: "$1,$2,$3",
			timestamp:  "now()",
			jsonText:   "payload::text",
			jsonValue:  "$2::json",
			insert:     "INSERT INTO sessions",
			suffix:     "\nON CONFLICT DO NOTHING",
		},
		{
			name:       "sqlite",
			driver:     "sqlite",
			firstBind:  "?",
			threeBinds: "?,?,?",
			timestamp:  "CURRENT_TIMESTAMP",
			jsonText:   "payload",
			jsonValue:  "json(?)",
			insert:     "INSERT OR IGNORE INTO sessions",
			suffix:     "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialect := NewSQLDialect(test.driver)
			if got := dialect.Bind(1); got != test.firstBind {
				t.Fatalf("Bind(1) = %q, want %q", got, test.firstBind)
			}
			if got := dialect.BindList(3); got != test.threeBinds {
				t.Fatalf("BindList(3) = %q, want %q", got, test.threeBinds)
			}
			if got := dialect.CurrentTimestamp(); got != test.timestamp {
				t.Fatalf("CurrentTimestamp() = %q, want %q", got, test.timestamp)
			}
			if got := dialect.JSONText("payload"); got != test.jsonText {
				t.Fatalf("JSONText() = %q, want %q", got, test.jsonText)
			}
			if got := dialect.JSONValue(2); got != test.jsonValue {
				t.Fatalf("JSONValue() = %q, want %q", got, test.jsonValue)
			}
			if got := dialect.InsertIgnoreInto("sessions"); got != test.insert {
				t.Fatalf("InsertIgnoreInto() = %q, want %q", got, test.insert)
			}
			if got := dialect.InsertIgnoreSuffix(); got != test.suffix {
				t.Fatalf("InsertIgnoreSuffix() = %q, want %q", got, test.suffix)
			}
		})
	}
}
