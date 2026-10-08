# 2026-09-29 changed-file sandbox baseline

SDK `062d93d7` routes turn-end changed-read-file detection through the active file
executor. Nexus `2c34b0546` and Bridge `c251a8d` passed all 61 host/macOS native checks
and 888 required names on macOS 27.0 arm64.

This remains development evidence; graphical App, macOS 14.0, signing/notarization,
clean-host, Intel and release acceptance are not proven (`releaseAccepted=false`).
