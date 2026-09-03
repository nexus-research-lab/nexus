//go:build darwin

package credentials

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

func loadHostKeychainConnectorKey() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(
		ctx,
		"security",
		"find-generic-password",
		"-s", "com.leemysw.nexus.desktop",
		"-a", "connector-credentials-key",
		"-w",
	).Output()
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", errors.New("Connector credentials Keychain item 为空")
	}
	return value, nil
}
