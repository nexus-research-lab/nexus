// Package computer adapts the Nexus Computer Use service to the round-scoped
// nexus.command protocol. The package exposes only bounded semantic operations;
// transport credentials, native handles, and sidecar process details remain in
// internal/service/computeruse.
// Action results may carry a fresh observation and transient image, while read
// failures remain separate from dispatch evidence.
// Screenshots use native MCP image blocks, never JSON text or structured data.
// launch.go adapts separately approved application lifecycle commands.
package computer
