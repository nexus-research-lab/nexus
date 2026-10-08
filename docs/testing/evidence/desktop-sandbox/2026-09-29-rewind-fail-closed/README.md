# 2026-09-29 restricted file rewind fail-closed baseline

SDK `7090d9c5` refuses legacy multi-file file-history rewind in restricted desktop
mode instead of bypassing the active file executor; unrestricted compatibility behavior
is unchanged. The host/macOS native baseline passed all 61 checks and 888 required
names on macOS 27.0 arm64. The atomic rewind file-executor port remains an explicit
follow-up; this is development evidence and `releaseAccepted=false`.
