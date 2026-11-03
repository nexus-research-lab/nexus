// INPUT: Raw JSON bytes, a nesting limit and an optional key predicate.
// OUTPUT: An error for duplicate/invalid keys, excess depth or trailing data.
// POS: Pre-decode guard; encoding/json silently keeps the last duplicate key.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	ErrDuplicateKey = errors.New("duplicate or invalid JSON field")
	ErrTooDeep      = errors.New("JSON nesting limit exceeded")
	ErrTrailingData = errors.New("trailing JSON data")
)

// ValidateKeys walks one JSON value. validKey may be nil to accept any key.
func ValidateKeys(payload []byte, maxDepth int, validKey func(string) bool) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > maxDepth {
			return ErrTooDeep
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] || (validKey != nil && !validKey(name)) {
					return ErrDuplicateKey
				}
				keys[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid JSON delimiter %q", delimiter)
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrTrailingData
	}
	return nil
}
