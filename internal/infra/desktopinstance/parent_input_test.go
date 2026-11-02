// 父进程 pipe 关闭才触发有序退出，普通字节不触发退出。
package desktopinstance

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestParentInputEOFRequestsShutdown(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := WatchParentInput(reader, cancel)
	if _, err := writer.Write([]byte("not a command")); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("content requested shutdown")
	}
	_ = writer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parent EOF did not stop host")
	}
	if ctx.Err() == nil {
		t.Fatal("missing shutdown cancellation")
	}
}
