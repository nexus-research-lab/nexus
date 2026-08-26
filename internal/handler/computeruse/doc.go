// Package computeruse exposes the authenticated settings lifecycle for the
// host-managed Nexus Computer Use Runtime package and sidecar.
//
// L2 members:
//   - handlers.go: owner-scoped status/install/update/doctor/start/stop/remove adapters.
//
// L2 interface:
//   - New returns nil only when the service is absent.
//   - lifecycle endpoints never accept executable paths, URLs, versions, tokens, or endpoints.
package computeruse
