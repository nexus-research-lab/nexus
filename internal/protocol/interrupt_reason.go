// INPUT: Host stop requests from the user, host control surfaces and process exit.
// OUTPUT: The single wire identity of each interrupt reason and of the Attempt failure reasons they produce.
// POS: Shared truth for runtime, message projection, DM/Room and orchestration; no stop policy lives here.
package protocol

const (
	// InterruptWithoutMessage marks the user's stop control; no stop text is
	// written into the result body.
	InterruptWithoutMessage = "__nexus_interrupt_without_message__"
	// InterruptHostControlWithoutMessage marks a host control stop (Goal pause,
	// automation cancel or timeout, Echo disable, Team remote cancel). Like a
	// user stop it writes no stop text, but it is never a user stop.
	InterruptHostControlWithoutMessage = "__nexus_interrupt_host_control__"
	// InterruptHostShutdown marks rounds cancelled by host exit; recovery treats
	// work interrupted this way as resumable. DM and Room also record it as the
	// failure reason of the root Attempts they settle on exit.
	InterruptHostShutdown = "host_shutdown"
	// WorkAttemptReasonHostControlStop is the root Attempt failure reason for a
	// host control stop, kept free of the internal sentinel above.
	WorkAttemptReasonHostControlStop = "host_control_stop"
)
