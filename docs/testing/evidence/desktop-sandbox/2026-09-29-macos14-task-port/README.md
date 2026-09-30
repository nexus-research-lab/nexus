# macOS 14.0 task-port termination probe

Host: macOS 27.0 arm64. This is a negative compatibility probe, not a product implementation.

## Result

A same-user `/bin/sleep` child was started, `task_name_for_pid` returned `KERN_SUCCESS`, and the returned Mach task port was passed to `task_terminate`. The kernel returned `KERN_PROTECTION_FAILURE` and the child remained running. Therefore a task port obtained through the existing non-privileged identity query cannot replace `proc_signal_with_audittoken` for the desktop sidecar.

The probe did not send a bare PID signal. The production implementation continues to fail closed when `proc_signal_with_audittoken` is unavailable. No minimum macOS version was raised based on this result.

## Probe output

```text
acquire 0 pid <fixture>
terminate 4
running true
```

`4` is `KERN_PROTECTION_FAILURE`.

## Boundary

This rules out one unsafe/unsupported fallback. It does not prove a deployable macOS 14.0 exact-termination design, and it does not change the release status. Signed/notarized packaging, clean-host installation and graphical App acceptance remain separate.
