package team

import (
	"context"
	"errors"
	"testing"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

type syncRemote struct {
	key               string
	token             string
	snapshotOptions   relaycontract.SnapshotOptions
	differenceOptions relaycontract.DifferenceOptions
	err               error
}

func (f *syncRemote) ListRooms(context.Context, string) (relaycontract.RoomList, error) {
	return relaycontract.RoomList{Rooms: []relaycontract.RoomView{{}}}, f.err
}
func (f *syncRemote) CreateRoom(_ context.Context, token, key string, _ relaycontract.CreateRoomInput) (relaycontract.RoomView, error) {
	f.key = key
	f.token = token
	return relaycontract.RoomView{}, f.err
}
func (f *syncRemote) PostMessage(_ context.Context, token, _, key string, _ relaycontract.CreateMessageInput) (relaycontract.MessageCommit, error) {
	f.key = key
	f.token = token
	return relaycontract.MessageCommit{EventSeq: 9, Replayed: true, StreamEpoch: "epoch"}, f.err
}
func (f *syncRemote) Snapshot(_ context.Context, _, _ string, options relaycontract.SnapshotOptions) (relaycontract.Snapshot, error) {
	f.snapshotOptions = options
	return relaycontract.Snapshot{SnapshotSeq: 9}, f.err
}
func (f *syncRemote) Difference(_ context.Context, _, _ string, options relaycontract.DifferenceOptions) (relaycontract.Difference, error) {
	f.differenceOptions = options
	return relaycontract.Difference{NextSeq: 9}, f.err
}

type syncProjection struct {
	err               error
	calls             int
	owner, deployment string
}

func (f *syncProjection) ProjectRoom(_ context.Context, owner, deployment string, _ relaycontract.RoomView) error {
	f.calls++
	f.owner = owner
	f.deployment = deployment
	return f.err
}
func (f *syncProjection) ProjectCommit(_ context.Context, owner string, _ relaycontract.MessageCommit) error {
	f.calls++
	f.owner = owner
	return f.err
}
func (f *syncProjection) ProjectSnapshot(_ context.Context, owner string, _ relaycontract.Snapshot) error {
	f.calls++
	f.owner = owner
	return f.err
}
func (f *syncProjection) ProjectDifference(_ context.Context, owner string, _ relaycontract.Difference) error {
	f.calls++
	f.owner = owner
	return f.err
}

func TestSyncProjectionFailurePreservesRemoteCommitAndReadFence(t *testing.T) {
	remote := &syncRemote{}
	projection := &syncProjection{err: errors.New("disk unavailable")}
	service := New(remote, projection, nil)
	access := Access{OwnerUserID: "owner-a", DeploymentID: "deployment-a", Token: "trusted-token"}
	ctx := context.Background()
	if _, err := service.ListRooms(ctx, access); !errors.Is(err, ErrProjection) {
		t.Fatalf("list err=%v", err)
	}
	if _, err := service.CreateRoom(ctx, access, "create-key", relaycontract.CreateRoomInput{}); !errors.Is(err, ErrProjection) {
		t.Fatalf("create err=%v", err)
	}
	snapshot := int64(9)
	through := int64(8)
	options := relaycontract.SnapshotOptions{SnapshotSeq: &snapshot, ThroughMessageSeq: &through, AfterMessageSeq: 3, StreamEpoch: "epoch", Limit: 10}
	if _, err := service.Snapshot(ctx, access, "conversation", options); !errors.Is(err, ErrProjection) {
		t.Fatalf("snapshot err=%v", err)
	}
	difference := relaycontract.DifferenceOptions{AfterSeq: 7, StreamEpoch: "epoch", Limit: 10}
	if _, err := service.Difference(ctx, access, "stream", difference); !errors.Is(err, ErrProjection) {
		t.Fatalf("difference err=%v", err)
	}
	result, err := service.PostMessage(ctx, access, "conversation", "message-key", relaycontract.CreateMessageInput{})
	if err != nil || result.EventSeq != 9 || !result.Replayed || result.StreamEpoch != "epoch" {
		t.Fatalf("commit=%+v err=%v", result, err)
	}
	if projection.owner != access.OwnerUserID || projection.deployment != access.DeploymentID || projection.calls != 5 {
		t.Fatalf("projection=%+v", projection)
	}
	if remote.key != "message-key" || remote.token != access.Token || remote.snapshotOptions != options || remote.differenceOptions != difference {
		t.Fatalf("远端身份或游标被改写: %+v", remote)
	}
	remote.err = errors.New("remote unavailable")
	if _, err := service.PostMessage(ctx, access, "conversation", "message-key", relaycontract.CreateMessageInput{}); err != remote.err || projection.calls != 5 {
		t.Fatal("远端失败不得开始投影")
	}
}
