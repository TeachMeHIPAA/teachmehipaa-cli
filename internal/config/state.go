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
	// EventCursors is the last Event `tmh events tail` saw, by profile
	// name, then Organization id.
	EventCursors map[string]map[string]string `toml:"event_cursors,omitempty"`
}

// EventCursor is where `tmh events tail` stopped for the profile and
// Organization, or "" when it never ran for them.
func (s State) EventCursor(profile, organizationID string) string {
	return s.EventCursors[profile][organizationID]
}

// SetEventCursor records the last Event seen for the profile and
// Organization.
func (s *State) SetEventCursor(profile, organizationID, eventID string) {
	if s.EventCursors == nil {
		s.EventCursors = map[string]map[string]string{}
	}
	if s.EventCursors[profile] == nil {
		s.EventCursors[profile] = map[string]string{}
	}
	s.EventCursors[profile][organizationID] = eventID
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

// Update re-reads the state before changing it, so fields another part of
// the same run saved meanwhile, such as the update check, survive.
func (s StateStore) Update(change func(*State)) error {
	state, err := s.Load()
	if err != nil {
		return err
	}
	change(&state)
	return s.Save(state)
}
