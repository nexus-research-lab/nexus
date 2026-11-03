// INPUT: Explicitly selected JSON logs containing nexus.command.call start/finish events.
// OUTPUT: Offline operation/outcome counts, repeated request observations and host-duration distributions.
// POS: Local measurement tool; incomplete calls stay unknown and no business operation is replayed.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

type diagnosticRecord struct {
	Event          string   `json:"event"`
	SchemaVersion  int      `json:"schema_version"`
	Phase          string   `json:"phase"`
	CallID         string   `json:"call_id"`
	ScopeHash      string   `json:"scope_hash"`
	RoundHash      string   `json:"round_hash"`
	Domain         string   `json:"domain"`
	Action         string   `json:"action"`
	Operation      string   `json:"operation"`
	RequestIDState string   `json:"request_id_state"`
	RequestHash    string   `json:"request_hash"`
	Status         string   `json:"status"`
	Outcome        string   `json:"outcome"`
	ReasonCode     string   `json:"reason_code"`
	HostDurationMS *float64 `json:"host_duration_ms"`
	Time           string   `json:"time"`
}

type observedCall struct {
	start, finish *diagnosticRecord
	conflict      bool
}

type collector struct {
	calls              map[string]*observedCall
	malformedLines     int
	unsupportedRecords int
	duplicateRecords   int
}

func (c *collector) read(input io.Reader) error {
	if c.calls == nil {
		c.calls = map[string]*observedCall{}
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if strings.TrimSpace(string(line)) == "" {
			continue
		}
		var record diagnosticRecord
		if err := json.Unmarshal(line, &record); err != nil {
			c.malformedLines++
			continue
		}
		if record.Event != "nexus.command.call" {
			continue
		}
		if record.SchemaVersion != 1 || record.CallID == "" || record.Phase != "start" && record.Phase != "finish" {
			c.unsupportedRecords++
			continue
		}
		call := c.calls[record.CallID]
		if call == nil {
			call = &observedCall{}
			c.calls[record.CallID] = call
		}
		slot := &call.start
		if record.Phase == "finish" {
			slot = &call.finish
		}
		if *slot == nil {
			*slot = &record
		} else {
			previous, _ := json.Marshal(*slot)
			current, _ := json.Marshal(record)
			if string(previous) == string(current) {
				c.duplicateRecords++
			} else {
				call.conflict = true
			}
		}
	}
	return scanner.Err()
}

type groupKey struct {
	Domain    string `json:"domain"`
	Action    string `json:"action"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason_code"`
}

type durationSummary struct {
	Samples int      `json:"samples"`
	Mean    *float64 `json:"mean"`
	P50     *float64 `json:"p50"`
	P95     *float64 `json:"p95"`
}

type groupReport struct {
	groupKey
	Calls       int             `json:"calls"`
	HostMS      durationSummary `json:"host_duration_ms"`
	durationsMS []float64
}

type report struct {
	Label               string         `json:"label"`
	Environment         string         `json:"source_environment"`
	GeneratedAt         string         `json:"generated_at"`
	Calls               int            `json:"observed_calls"`
	Incomplete          int            `json:"incomplete_calls"`
	MissingStart        int            `json:"calls_missing_start"`
	Conflicting         int            `json:"conflicting_calls"`
	RepeatedRequests    int            `json:"repeated_request_observations"`
	MalformedLines      int            `json:"malformed_lines"`
	UnsupportedRecords  int            `json:"unsupported_records"`
	DuplicateLogRecords int            `json:"duplicate_log_records"`
	Unknown             map[string]int `json:"missing_or_unknown"`
	Groups              []groupReport  `json:"groups"`
	PerToolTokens       *int64         `json:"per_tool_tokens"`
	Limitations         []string       `json:"limitations"`
}

func (c *collector) report(label, environment string) report {
	r := report{
		Label: label, Environment: environment, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Calls: len(c.calls),
		MalformedLines: c.malformedLines, UnsupportedRecords: c.unsupportedRecords, DuplicateLogRecords: c.duplicateRecords,
		Unknown: map[string]int{}, Groups: []groupReport{},
		Limitations: []string{
			"Host durations include command validation, dispatch and serialization; they are not model or end-to-end latency.",
			"Repeated request observations count calls with the same scoped request hash, not repeated side effects.",
			"Missing finishes and conflicting records remain unknown. Calls with neither event cannot be counted; diagnostics are best effort.",
			"Per-tool tokens are unavailable. Collect actual provider/round usage separately; do not estimate tokens from these records.",
			"Registered Goal/Execution operations are named; other domains and calls rejected before registry lookup may have unknown operation labels.",
		},
	}
	groups := map[groupKey]*groupReport{}
	requests := map[string]int{}
	for _, call := range c.calls {
		record := call.finish
		if record == nil {
			record = call.start
			r.Incomplete++
		}
		if call.start == nil {
			r.MissingStart++
		}
		conflicting := call.conflict
		if call.start != nil && call.finish != nil {
			a, b := call.start, call.finish
			conflicting = conflicting || a.ScopeHash != b.ScopeHash || a.RoundHash != b.RoundHash || a.Domain != b.Domain || a.Action != b.Action || a.RequestHash != b.RequestHash || a.RequestIDState != b.RequestIDState
		}
		key := groupKey{
			Domain: known(record.Domain), Action: known(record.Action), Operation: known(record.Operation),
			Status: known(record.Status), Outcome: known(record.Outcome), Reason: known(record.ReasonCode),
		}
		if call.finish == nil {
			key.Status, key.Outcome, key.Reason = "incomplete", "unknown", "missing_finish"
		} else if key.Status == "incomplete" {
			r.Incomplete++
			key.Outcome = "unknown"
		} else if key.Status != "ok" {
			key.Outcome = "unknown"
		}
		if conflicting {
			r.Conflicting++
			key.Status, key.Outcome, key.Reason = "unknown", "unknown", "conflicting_records"
		}
		for field, value := range map[string]string{"domain": key.Domain, "action": key.Action, "operation": key.Operation, "status": key.Status, "outcome": key.Outcome, "reason_code": key.Reason} {
			if value == "unknown" || value == "not_reported" || value == "other" {
				r.Unknown[field]++
			}
		}
		if record.RequestIDState == "valid" && record.RequestHash != "" && !conflicting {
			requests[record.RequestHash]++
		} else {
			r.Unknown["request_hash"]++
		}
		group := groups[key]
		if group == nil {
			group = &groupReport{groupKey: key}
			groups[key] = group
		}
		group.Calls++
		if duration := record.HostDurationMS; call.finish != nil && !conflicting && duration != nil && *duration >= 0 && !math.IsInf(*duration, 0) && !math.IsNaN(*duration) {
			group.durationsMS = append(group.durationsMS, *duration)
		} else {
			r.Unknown["host_duration_ms"]++
		}
	}
	for _, count := range requests {
		r.RepeatedRequests += count - 1
	}
	for _, group := range groups {
		group.HostMS = summarizeDurations(group.durationsMS)
		r.Groups = append(r.Groups, *group)
	}
	sort.Slice(r.Groups, func(i, j int) bool {
		a, _ := json.Marshal(r.Groups[i].groupKey)
		b, _ := json.Marshal(r.Groups[j].groupKey)
		return string(a) < string(b)
	})
	return r
}

func known(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func summarizeDurations(values []float64) durationSummary {
	result := durationSummary{Samples: len(values)}
	if len(values) == 0 {
		return result
	}
	sort.Float64s(values)
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	// Nearest-rank quantiles remain defined for small samples; sample count is
	// always reported so a one-call p95 is not mistaken for a stable estimate.
	p50, p95 := values[int(math.Ceil(float64(len(values))*0.50))-1], values[int(math.Ceil(float64(len(values))*0.95))-1]
	result.Mean, result.P50, result.P95 = &mean, &p50, &p95
	return result
}

func run(args []string, output, stderr io.Writer) error {
	flags := flag.NewFlagSet("workgraph-command-report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	label := flags.String("label", "", "sample or experiment label; identify synthetic samples explicitly")
	environment := flags.String("environment", "", "source host/database/provider/model and scenario, as applicable")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *label == "" || *environment == "" || flags.NArg() == 0 {
		return fmt.Errorf("usage: workgraph-command-report --label NAME --environment DESCRIPTION LOG.jsonl [LOG.jsonl ...]")
	}
	var c collector
	for _, path := range flags.Args() {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		err = c.read(file)
		closeErr := file.Close()
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c.report(*label, *environment))
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
