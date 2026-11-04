// INPUT: The supervisor's owned CUA process and the desktop host's private pipe.
// OUTPUT: Bounded best-effort OS foreground handoff; no target authority is issued.
// POS: Windows host integration; native handles never enter the model protocol.
package computeruse

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func prepareNativeProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}

func prepareNativeForeground(ctx context.Context, process *os.Process) {
	name := os.Getenv("NEXUS_CUA_FOREGROUND_PIPE")
	if process == nil || !strings.HasPrefix(name, "NexusCUAForeground-") || strings.ContainsAny(name, `\/`) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	connection, err := winio.DialPipeContext(ctx, `\\.\pipe\`+name)
	if err != nil {
		return
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	_ = connection.SetDeadline(deadline)
	var request [4]byte
	binary.LittleEndian.PutUint32(request[:], uint32(process.Pid))
	if _, err = connection.Write(request[:]); err != nil {
		return
	}
	var result [1]byte
	_, _ = io.ReadFull(connection, result[:])
	// A refusal is not a mutation failure: an already eligible runtime can
	// still act. Its native foreground readback remains authoritative.
}
