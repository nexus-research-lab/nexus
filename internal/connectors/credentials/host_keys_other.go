//go:build !darwin

package credentials

import "errors"

func loadHostKeychainConnectorKey() (string, error) {
	return "", errors.New("当前平台不提供 macOS Connector credentials Keychain")
}
