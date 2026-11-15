package keyring

import (
	"errors"
	"sync"
)

// Memory is an in-process Keyring for tests. It is safe to share between
// concurrent runs, as one machine's keyring is between processes.
type Memory struct {
	mu      sync.Mutex
	secrets map[string]string
	broken  error
}

// NewMemory is an empty keyring that works.
func NewMemory() *Memory { return &Memory{secrets: map[string]string{}} }

// NewUnavailable fails every call, like a machine with no credential store.
func NewUnavailable() *Memory {
	return &Memory{secrets: map[string]string{}, broken: errors.New("keyring: no credential store available")}
}

// Get returns the stored secret, or ErrNotFound.
func (m *Memory) Get(service, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken != nil {
		return "", m.broken
	}
	secret, ok := m.secrets[service+"\x00"+user]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

// Set stores the secret, replacing any before it.
func (m *Memory) Set(service, user, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken != nil {
		return m.broken
	}
	m.secrets[service+"\x00"+user] = secret
	return nil
}

// Delete removes the secret, or returns ErrNotFound.
func (m *Memory) Delete(service, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken != nil {
		return m.broken
	}
	key := service + "\x00" + user
	if _, ok := m.secrets[key]; !ok {
		return ErrNotFound
	}
	delete(m.secrets, key)
	return nil
}
