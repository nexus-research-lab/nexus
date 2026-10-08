package runtime

// sandboxProcessMarkerIsActive applies the strongest process identity proof
// available for the marker's platform. A mismatched creation time proves that
// the recorded PID has been reused and is therefore safe to treat as dead;
// inability to obtain that proof remains active so recovery never guesses.
func sandboxProcessMarkerIsActive(marker SandboxLeaseMarker) bool {
	if marker.ProcessStartTimeUnixNano > 0 {
		start, alive, known := sandboxProcessIdentity(marker.ProcessID)
		if !known {
			return true
		}
		if !alive {
			return false
		}
		return start == marker.ProcessStartTimeUnixNano
	}
	alive, known := sandboxProcessAlive(marker.ProcessID)
	return alive || !known
}
