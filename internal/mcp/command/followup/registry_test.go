package followup

import (
	"context"
	"testing"
	"time"

	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

type stubService struct{ last dmsvc.FollowUpRequest }

func (s *stubService) ScheduleFollowUp(_ context.Context, _ string, _ string, request dmsvc.FollowUpRequest) (dmsvc.FollowUp, error) {
	s.last = request
	return dmsvc.FollowUp{ID: "f1", Note: request.Note}, nil
}

func (s *stubService) ListFollowUps(context.Context, string, string) ([]dmsvc.FollowUp, error) {
	return nil, nil
}

func (s *stubService) CancelFollowUp(context.Context, string, string, string) error { return nil }

func TestScheduleMapsInputAndRequestID(t *testing.T) {
	stub := &stubService{}
	operation, _ := command.FindOperation(BuildAll(stub, "s", "nexus"), "schedule_follow_up")
	result, err := operation.Invoke(context.Background(), map[string]any{
		"at": "2026-11-26T15:30:00+08:00", "note": "看 CI",
	}, &command.CallContext{RequestID: "req-12345678"})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := time.Date(2026, 11, 26, 7, 30, 0, 0, time.UTC)
	if !stub.last.At.Equal(want) || stub.last.RequestKey != "req-12345678" || stub.last.Note != "看 CI" {
		t.Fatalf("request = %+v", stub.last)
	}
	if result.StructuredContent["outcome"] != "applied" {
		t.Fatalf("outcome = %v", result.StructuredContent["outcome"])
	}
}

func TestScheduleRejectsBadTime(t *testing.T) {
	operation, _ := command.FindOperation(BuildAll(&stubService{}, "s", "nexus"), "schedule_follow_up")
	result, _ := operation.Invoke(context.Background(), map[string]any{"at": "明天下午", "note": "x"}, nil)
	if !result.IsError {
		t.Fatal("非 RFC3339 的 at 必须拒绝")
	}
}
