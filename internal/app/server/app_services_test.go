package server

import (
	"context"
	"os"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestAppServicesCloseReleasesOwnedDatabase(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	services, err := NewAppServices(cfg, nil)
	if err != nil {
		t.Fatalf("创建自有数据库 AppServices 失败: %v", err)
	}
	t.Cleanup(func() { _ = services.DB.Close() })
	if err = services.Close(context.Background()); err != nil {
		t.Fatalf("关闭自有数据库 AppServices 失败: %v", err)
	}
	if err = services.DB.Ping(); err == nil {
		t.Fatal("AppServices.Close 后数据库仍可用")
	}
	if err = os.Remove(cfg.DatabaseURL); err != nil {
		t.Fatalf("AppServices.Close 后数据库文件仍被占用: %v", err)
	}
}

func TestAppServicesClosePreservesBorrowedDatabase(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开外部数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	services := NewAppServicesWithDB(cfg, db, nil)
	if err = services.Close(context.Background()); err != nil {
		t.Fatalf("关闭借用数据库 AppServices 失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		t.Fatalf("AppServices.Close 不应关闭外部数据库: %v", err)
	}
}
