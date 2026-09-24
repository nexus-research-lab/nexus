//go:build plan9 || wasip1 || js

package runtime

// Process liveness is intentionally unknown on platforms without a safe,
// portable non-destructive probe. Recovery retains the marker there instead
// of risking deletion of an active runtime; native platform cleanup remains a
// separate acceptance item.
func sandboxProcessAlive(pid int) (alive, known bool) {
	return false, false
}

func sandboxProcessIdentity(pid int) (startTimeUnixNano int64, alive, known bool) {
	return 0, false, false
}

func currentProcessStartTimeUnixNano() (int64, error) {
	return 0, nil
}
