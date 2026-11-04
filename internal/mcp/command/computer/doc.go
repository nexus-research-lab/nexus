// Package computer adapts the Nexus Computer Use service to the round-scoped
// nexus.command protocol. The package exposes only bounded semantic operations;
// transport credentials, native handles, and sidecar process details remain in
// internal/service/computeruse.
// Action results may carry a fresh observation and transient image, while read
// failures remain separate from dispatch evidence.
// Screenshots use native MCP image blocks, never JSON text or structured data.
// Observe includes screenshots by default while explicit false permits metadata
// recovery. The model input boundary normalizes key casing and Return/Esc aliases.
// Error Content and structured receipts both retain mutation status and recovery
// fields; background capture errors spell out both metadata-only observe options.
// Unknown thumbnail states guide one focus recovery; indeterminate mutations
// permit one reconciliation and must not be bypassed by a new target or launch.
// Capability output distinguishes authorization kinds from restore_window's model alias.
// launch.go adapts separately approved application lifecycle commands.
// Both app catalogs support a bounded name/id query to avoid unrelated result
// pagination. Pre-dispatch stale/running-app failures explain recovery within authority.
package computer
