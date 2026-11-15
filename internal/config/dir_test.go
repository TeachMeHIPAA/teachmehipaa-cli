package config_test

import (
	"errors"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
)

func TestTheConfigDirIsXDGStyleOnEveryOS(t *testing.T) {
	home := func() (string, error) { return "/home/ana", nil }
	tests := []struct {
		name string
		vars map[string]string
		want string
	}{
		{name: "XDG_CONFIG_HOME set", vars: map[string]string{"XDG_CONFIG_HOME": "/srv/config"}, want: "/srv/config/tmh"},
		{name: "XDG_CONFIG_HOME unset", vars: map[string]string{}, want: "/home/ana/.config/tmh"},
		{name: "XDG_CONFIG_HOME empty", vars: map[string]string{"XDG_CONFIG_HOME": ""}, want: "/home/ana/.config/tmh"},
		// The XDG spec says to ignore a relative path.
		{name: "XDG_CONFIG_HOME relative", vars: map[string]string{"XDG_CONFIG_HOME": "config"}, want: "/home/ana/.config/tmh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) { v, ok := tt.vars[name]; return v, ok }

			got, err := config.DefaultDir(lookup, home)

			if err != nil || got != tt.want {
				t.Errorf("DefaultDir() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestTheConfigDirNeedsAHomeWithoutXDGConfigHome(t *testing.T) {
	noHome := func() (string, error) { return "", errors.New("$HOME is not defined") }

	if _, err := config.DefaultDir(func(string) (string, bool) { return "", false }, noHome); err == nil {
		t.Error("DefaultDir() succeeded without a home directory")
	}
}
