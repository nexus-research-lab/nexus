package main

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestReportCountsCorruptionAndDoesNotTrustConflictingRecords(t *testing.T) {
	base := diagnosticRecord{Event: "nexus.command.call", SchemaVersion: 1, CallID: "call-1", Phase: "finish", Domain: "execution", Action: "invoke", Operation: "submit_work", Status: "ok", Outcome: "applied"}
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, record := range []diagnosticRecord{base, base} {
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	base.Outcome = "rejected"
	_ = encoder.Encode(base)
	base.CallID, base.SchemaVersion = "future-call", 2
	_ = encoder.Encode(base)
	input.WriteString("not json\n{\"event\":\"unrelated\"}\n")
	var c collector
	if err := c.read(&input); err != nil {
		t.Fatal(err)
	}
	r := c.report("synthetic-corruption", "fixture")
	if r.Calls != 1 || r.MissingStart != 1 || r.Conflicting != 1 || r.MalformedLines != 1 || r.UnsupportedRecords != 1 || r.DuplicateLogRecords != 1 {
		t.Fatalf("corrupt report = %+v", r)
	}
	if len(r.Groups) != 1 || r.Groups[0].Outcome != "unknown" || r.Groups[0].Status != "unknown" || r.Groups[0].HostMS.Mean != nil {
		t.Fatalf("conflict reported as success: %+v", r.Groups)
	}
}

func TestReportRequiresExperimentMetadataAndPreservesEmptyDurations(t *testing.T) {
	if err := run(nil, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted an unlabeled measurement")
	}
	if empty := summarizeDurations(nil); empty.Samples != 0 || empty.Mean != nil || empty.P50 != nil || empty.P95 != nil {
		t.Fatalf("empty durations = %+v", empty)
	}
	values := summarizeDurations([]float64{4, 1, 3, 2})
	if values.Samples != 4 || *values.Mean != 2.5 || *values.P50 != 2 || *values.P95 != 4 {
		t.Fatalf("duration summary = %+v", values)
	}
}
