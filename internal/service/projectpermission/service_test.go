package projectpermission

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeOwnerRuntimeCloser struct {
	owners      []string
	contextErrs []error
	err         error
}

func (f *fakeOwnerRuntimeCloser) CloseOwnerSessions(
	ctx context.Context,
	ownerUserID string,
) (int, error) {
	f.owners = append(f.owners, ownerUserID)
	f.contextErrs = append(f.contextErrs, ctx.Err())
	return 2, f.err
}

func TestEnsureResultDecodesLauncherShape(t *testing.T) {
	var result EnsureResult
	err := json.Unmarshal([]byte(`{
		"project": {
			"project_id": "project-a",
			"group_name": "nxp_project_a",
			"gid": 21001,
			"root": "/var/lib/nexus/shared-workspaces/project-a",
			"members": {"user-a": "write"},
			"generation": 7
		},
		"created": true
	}`), &result)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Project.ProjectID != "project-a" ||
		result.Project.Members["user-a"] != "write" {
		t.Fatalf("launcher project 响应解码错误: %+v", result)
	}
}

func TestNormalizeProjectIDRejectsPathSegments(t *testing.T) {
	for _, value := range []string{"", ".", "..", "../project", "team/project", `team\project`} {
		if _, err := normalizeProjectID(value); !errors.Is(err, ErrInvalidProjectID) {
			t.Fatalf("normalizeProjectID(%q) = %v, want ErrInvalidProjectID", value, err)
		}
	}
}

func TestGrantClosesOwnerRuntimeAfterLauncherUpdate(t *testing.T) {
	var arguments []string
	closer := &fakeOwnerRuntimeCloser{}
	service := &Service{
		runCommand: func(_ context.Context, args []string) ([]byte, error) {
			arguments = append([]string(nil), args...)
			return []byte(`{"changed":true}`), nil
		},
		runtimeCloser: closer,
	}

	result, err := service.Grant(context.Background(), "project-a", "owner-a", "read")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("launcher 返回 changed=true 后应保留变更结果")
	}
	if len(arguments) != 7 ||
		arguments[0] != "project-grant" ||
		arguments[2] != "project-a" ||
		arguments[4] != "owner-a" ||
		arguments[6] != "read" {
		t.Fatalf("launcher 参数错误: %#v", arguments)
	}
	if len(closer.owners) != 1 || closer.owners[0] != "owner-a" {
		t.Fatalf("runtime 回收 owner 错误: %#v", closer.owners)
	}
}

func TestGrantDoesNotCloseRuntimeWhenLauncherFails(t *testing.T) {
	closer := &fakeOwnerRuntimeCloser{}
	service := &Service{
		runCommand: func(context.Context, []string) ([]byte, error) {
			return nil, errors.New("launcher failed")
		},
		runtimeCloser: closer,
	}

	_, err := service.Grant(context.Background(), "project-a", "owner-a", "none")
	if err == nil || len(closer.owners) != 0 {
		t.Fatalf("launcher 失败后不应回收 runtime: err=%v owners=%#v", err, closer.owners)
	}
}

func TestGrantSkipsRuntimeCleanupWhenAccessDidNotChange(t *testing.T) {
	closer := &fakeOwnerRuntimeCloser{}
	service := &Service{
		runCommand: func(context.Context, []string) ([]byte, error) {
			return []byte(`{"changed":false}`), nil
		},
		runtimeCloser: closer,
	}

	result, err := service.Grant(context.Background(), "project-a", "owner-a", "write")
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || len(closer.owners) != 0 {
		t.Fatalf("无变化的授权不应回收 runtime: result=%+v owners=%#v", result, closer.owners)
	}
}

func TestGrantCleanupSurvivesRequestCancellationAfterACLUpdate(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	closer := &fakeOwnerRuntimeCloser{}
	service := &Service{
		runCommand: func(context.Context, []string) ([]byte, error) {
			cancelRequest()
			return []byte(`{"changed":true}`), nil
		},
		runtimeCloser: closer,
	}

	if _, err := service.Grant(requestCtx, "project-a", "owner-a", "none"); err != nil {
		t.Fatal(err)
	}
	if len(closer.contextErrs) != 1 || closer.contextErrs[0] != nil {
		t.Fatalf("ACL 落盘后的安全回收不应继承请求取消: %#v", closer.contextErrs)
	}
}
