# 2026-09-29 plan-file sandbox baseline

This evidence records the host and macOS native baseline after SDK commit `65a028ce`.
The SDK plan-mode and restored-read paths now use the active file executor for restricted
runtime IO. The baseline passed all 61 checks and 888 required check names on macOS
27.0 arm64 using Nexus `2c34b0546` and Bridge `c251a8d`.

The report is a development baseline only. It does not prove macOS 14.0 signal
compatibility, graphical App end-to-end acceptance, signing/notarization, clean-host or
Intel installation, or release acceptance; `releaseAccepted` remains `false`.

Command:

```text
node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox --sdk-ref 65a028ce
```
