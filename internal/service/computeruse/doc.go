// Package computeruse manages the independent Nexus Computer Use Runtime package,
// sidecar lifecycle, and round-scoped host policy used by `nexus computer`.
//
// L2 | parent: internal/service (L1 in AGENTS.md)
//
// Members:
//   - model.go: package, doctor, sidecar, and round-facing public models.
//   - package_manager.go: pinned manifest resolution, installation, status, and removal.
//   - package_archive.go: bounded tar.gz/zip binary extraction.
//   - replace_file_*.go: atomic current-package marker replacement per host platform.
//   - supervisor.go: private token/endpoint generation and sidecar process lifecycle.
//   - process_signal_*.go: exact supervised-process shutdown per host platform.
//   - runtime_client.go: narrow official Go SDK adapter and immutable action handle.
//   - service.go: owner preference gate and settings/runtime orchestration.
//   - round.go: physical-round target, observation, mutation reconciliation, and cleanup.
//   - operations.go / input.go / result.go: closed `nexus computer` operations,
//     typed input conversion, host policy, and stable errors.
//   - artifact.go: checksum-verified private screenshot projection into round storage.
//
// [PROTOCOL]: update this header, docs/specs/computer-use-integration-spec.md,
// the computer-use Skill, handlers, and tests when package or command semantics change.
package computeruse
