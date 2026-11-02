// INPUT: Windows 桌面父进程独占写端的继承管道与应用取消函数。
// OUTPUT: 父端关闭后触发正常服务关闭，先让 runtime 持久化回收结果。
// POS: 桌面进程寿命信号，不承载命令或授权协议。
package desktopinstance

import (
	"context"
	"io"
)

// WatchParentInput 只识别 EOF/管道失败；内容不作为命令解释。
func WatchParentInput(input io.Reader, stop context.CancelFunc) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); _, _ = io.Copy(io.Discard, input); stop() }()
	return done
}
