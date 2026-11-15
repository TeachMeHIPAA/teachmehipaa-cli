package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// State is what tmh remembers between runs that isn't configuration.
type State struct {
	// UpdateCheckedAt is when tmh last asked GitHub for its latest release.
	UpdateCheckedAt time.Time `toml:"update_checked_at,omitempty"`
}

// StateStore keeps State in state.toml next to config.toml.
type StateStore struct {
	Dir string
}

func (s StateStore) Path() string { return filepath.Join(s.Dir, "state.toml") }

// Load returns the zero State when nothing was saved yet.
func (s StateStore) Load() (State, error) {
	state := State{}
	_, err := toml.DecodeFile(s.Path(), &state)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return State{}, fmt.Errorf("reading %s: %w", s.Path(), err)
	}
	return state, nil
}

func (s StateStore) Save(state State) error {
	return writeTOML(s.Dir, s.Path(), state)
}
