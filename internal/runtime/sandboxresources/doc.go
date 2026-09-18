// Package sandboxresources owns desktop runtime scratch directories.
// The host creates and removes each lease; SDK/Bridge only negotiates the
// resulting versioned policy and never performs lifecycle cleanup.
package sandboxresources
