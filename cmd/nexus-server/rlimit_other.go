//go:build !unix

package main

type openFilesLimitSnapshot struct {
	Soft   uint64
	Hard   uint64
	Raised bool
}

func ensureOpenFilesLimit(uint64) (openFilesLimitSnapshot, error) {
	return openFilesLimitSnapshot{}, nil
}
