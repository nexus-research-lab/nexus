//go:build !windows

package computeruse

import (
	"os"
)

func signalSidecar(process *os.Process) error {
	return process.Signal(os.Interrupt)
}
