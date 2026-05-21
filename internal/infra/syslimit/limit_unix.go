//go:build unix

package syslimit

import "syscall"

func init() {
	EnsureOpenFilesLimit = ensureOpenFilesLimitUnix
}

func ensureOpenFilesLimitUnix(target uint64) (OpenFilesLimitSnapshot, error) {
	var current syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &current); err != nil {
		return OpenFilesLimitSnapshot{}, err
	}

	snapshot := OpenFilesLimitSnapshot{
		Soft: current.Cur,
		Hard: current.Max,
	}
	if target == 0 || current.Cur >= target {
		return snapshot, nil
	}

	nextSoft := target
	if current.Max > 0 && nextSoft > current.Max {
		nextSoft = current.Max
	}
	if nextSoft <= current.Cur {
		return snapshot, nil
	}

	next := current
	next.Cur = nextSoft
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &next); err != nil {
		return snapshot, err
	}
	snapshot.Soft = nextSoft
	snapshot.Raised = true
	return snapshot, nil
}
