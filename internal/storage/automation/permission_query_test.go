package automation

import (
	"reflect"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func TestPermissionRequestListQueryBoundsActionableCandidatesBeforeExactJoin(t *testing.T) {
	repository := &Repository{}
	query, args := repository.permissionRequestListQuery("owner-1", "actionable", "job-1")

	statusFilter := "status IN (?, ?)"
	statusIndex := strings.Index(query, statusFilter)
	exactJoinIndex := strings.Index(query, "EXISTS (")
	if statusIndex < 0 || exactJoinIndex < 0 || statusIndex > exactJoinIndex {
		t.Fatalf("actionable 查询必须先按状态缩小候选集: %s", query)
	}
	wantArgs := []any{
		"owner-1",
		automationdomain.PermissionRequestStatusPending,
		automationdomain.PermissionRequestStatusApproved,
		"job-1",
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("actionable 查询参数顺序错误: got=%#v want=%#v", args, wantArgs)
	}
}
