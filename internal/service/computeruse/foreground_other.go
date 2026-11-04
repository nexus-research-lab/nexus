//go:build !windows

// INPUT: Host context and an owned runtime process.
// OUTPUT: No operation on platforms without Windows foreground eligibility.
// POS: Platform complement of the private desktop foreground handoff.
package computeruse

import (
	"context"
	"os"
)

func prepareNativeForeground(context.Context, *os.Process) {}
