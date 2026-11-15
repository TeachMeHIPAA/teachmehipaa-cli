// Package keyring stores secrets in the operating system's credential store.
package keyring

import (
	"errors"

	gokeyring "github.com/zalando/go-keyring"
)

// ErrNotFound means nothing is stored under that service and user.
var ErrNotFound = errors.New("keyring: secret not found")

// Keyring is the OS credential store. Any error other than ErrNotFound means
// the store itself is unusable, e.g. no Secret Service in a CI container.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// System is the macOS Keychain, Windows Credential Manager or Secret Service.
func System() Keyring { return systemKeyring{} }

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) {
	secret, err := gokeyring.Get(service, user)
	return secret, normalize(err)
}

func (systemKeyring) Set(service, user, secret string) error {
	return normalize(gokeyring.Set(service, user, secret))
}

func (systemKeyring) Delete(service, user string) error {
	return normalize(gokeyring.Delete(service, user))
}

func normalize(err error) error {
	if errors.Is(err, gokeyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
