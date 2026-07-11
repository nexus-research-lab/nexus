package automation

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

func TestSchedulerLeaseAllowsSingleLeaderAndExpiryTakeover(t *testing.T) {
	db := newAutomationTestDB(t)
	newService := func(ownerID string) *Service {
		service := NewService(
			config.Config{
				DatabaseDriver:                  "sqlite",
				AutomationSchedulerLeaseSeconds: 3,
			},
			db,
			nil,
			nil,
			nil,
			permissionctx.NewContext(),
			&fakeWorkspaceReader{},
			nil,
		)
		service.schedulerOwnerID = ownerID
		return service
	}

	first := newService("scheduler-a")
	second := newService("scheduler-b")
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)

	held, becameLeader, err := first.refreshSchedulerLease(context.Background(), now)
	if err != nil || !held || !becameLeader {
		t.Fatalf("第一个实例应获取租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("租约有效期内第二个实例不应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}

	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || !held || !becameLeader {
		t.Fatalf("租约过期后第二个实例应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = first.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("原 leader 不应覆盖新租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
}

func TestSchedulerLeaseReleaseRequiresOwner(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	service.schedulerOwnerID = "scheduler-owner"
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	if held, _, err := service.refreshSchedulerLease(context.Background(), now); err != nil || !held {
		t.Fatalf("获取租约失败: held=%v err=%v", held, err)
	}
	if err := service.repository.ReleaseSchedulerLease(context.Background(), automationSchedulerLeaseName, "other-owner"); err != nil {
		t.Fatalf("非 owner 释放租约失败: %v", err)
	}
	if held, _, err := service.refreshSchedulerLease(context.Background(), now.Add(11*time.Second)); err != nil || !held {
		t.Fatalf("非 owner 不应影响现有租约: held=%v err=%v", held, err)
	}
	if err := service.releaseSchedulerLease(context.Background()); err != nil {
		t.Fatalf("owner 释放租约失败: %v", err)
	}
}
