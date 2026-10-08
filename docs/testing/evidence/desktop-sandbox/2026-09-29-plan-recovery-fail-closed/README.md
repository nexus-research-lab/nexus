# 2026-09-29 restricted plan recovery fail-closed baseline

SDK `867e788c` makes restricted plan recovery use an exclusive-create file port and
only proceed after an explicit not-found result. Permission, cancellation, transport
unknown, existing-file and partial-result cases are covered by regression tests and do
not overwrite or replay the plan write. Nexus `2c34b0546` and Bridge `c251a8d` passed
all 61 host/macOS native checks and 888 required names on macOS 27.0 arm64.

This remains development evidence; release acceptance is false.
