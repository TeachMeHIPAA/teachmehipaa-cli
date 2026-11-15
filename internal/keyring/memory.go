package keyring

import "errors"

// Memory is an in-process Keyring for tests.
type Memory struct {
	secrets map[string]string
	broken  error
}

func NewMemory() *Memory { return &Memory{secrets: map[string]string{}} }

// NewUnavailable fails every call, like a machine with no credential store.
func NewUnavailable() *Memory {
	return &Memory{secrets: map[string]string{}, broken: errors.New("keyring: no credential store available")}
}

func (m *Memory) Get(service, user string) (string, error) {
	if m.broken != nil {
		return "", m.broken
	}
	secret, ok := m.secrets[service+"\x00"+user]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (m *Memory) Set(service, user, secret string) error {
	if m.broken != nil {
		return m.broken
	}
	m.secrets[service+"\x00"+user] = secret
	return nil
}

func (m *Memory) Delete(service, user string) error {
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
