package cli

import (
	"strconv"
	"strings"
)

type intPointerFlag struct {
	target **int
}

func newIntPointerFlag(target **int) *intPointerFlag {
	return &intPointerFlag{target: target}
}

func (f *intPointerFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		*f.target = nil
		return nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return usageError(err)
	}
	*f.target = &parsed
	return nil
}

func (f *intPointerFlag) String() string {
	if f == nil || f.target == nil || *f.target == nil {
		return ""
	}
	return strconv.Itoa(**f.target)
}

func (f *intPointerFlag) Type() string {
	return "int"
}
