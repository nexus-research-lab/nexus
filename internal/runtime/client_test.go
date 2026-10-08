// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package runtime

func (c *agentClient) currentSandboxLease() *SandboxResourceLease {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sandboxLease
}
