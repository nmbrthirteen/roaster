// Package secret stores the one credential a kiosk device has to hold: the
// token that identifies it to the roast service.
//
// Model keys never come near a device. A stand in a public hall holds a token
// that can do exactly one thing, is rate limited, expires with the event, and
// can be revoked from the server the moment the device goes missing. Encryption
// here is defence in depth, not the defence.
package secret

import (
	"fmt"
	"os"
	"strings"

	"github.com/upgaming/roaster/internal/state"
)

const fileName = "terminal.token"

func path() (string, error) { return state.Path(fileName), nil }

// Load returns the terminal token, or an empty string if none is stored.
func Load() (string, error) {
	p, err := path()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	token, err := unprotect(raw)
	if err != nil {
		return "", fmt.Errorf("%s could not be read on this machine: %w", fileName, err)
	}
	return strings.TrimSpace(string(token)), nil
}

// Store writes the token, encrypted to this machine where the platform allows.
func Store(token string) error {
	p, err := path()
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	sealed, err := protect([]byte(strings.TrimSpace(token)))
	if err != nil {
		return err
	}
	return os.WriteFile(p, sealed, 0o600)
}
