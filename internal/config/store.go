package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
)

const keyringService = "tmh"

// Credentials are one API client's id and secret, plus the Organization they
// are bound to when known.
type Credentials struct {
	ClientID       string
	ClientSecret   string
	OrganizationID string
	// PartnerApp means the id and secret are a Partner app's, and the
	// profile holds a Grant's Refresh token, read only under LockGrant.
	PartnerApp bool
}

// ErrNotLoggedIn means the profile has no stored credentials.
var ErrNotLoggedIn = errors.New("not logged in")

// Store keeps client ids in config.toml and secrets in the OS keyring. When
// the keyring is unusable, as in most CI containers, the secret goes into
// config.toml instead, which is why that file is always written 0600.
type Store struct {
	Keyring keyring.Keyring
	Dir     string
}

type fileConfig struct {
	Profiles map[string]fileProfile `toml:"profiles,omitempty"`
	// LocalSigningSecrets holds a profile's local Signing secret when the
	// keyring is unusable. It lives outside Profiles so logging out keeps
	// it, and partners needn't reconfigure their handler.
	LocalSigningSecrets map[string]string `toml:"local_signing_secrets,omitempty"`
}

type fileProfile struct {
	ClientID       string `toml:"client_id"`
	OrganizationID string `toml:"organization_id,omitempty"`
	PartnerApp     bool   `toml:"partner_app,omitempty"`
	ClientSecret   string `toml:"client_secret,omitempty"`
	RefreshToken   string `toml:"refresh_token,omitempty"`
}

// errNoGrant means another tmh process logged the profile out, or into
// something else, since this one read it.
func errNoGrant(profile string) error {
	return fmt.Errorf("%w: the %s profile no longer holds a Grant", ErrNotLoggedIn, profile)
}

// refreshTokenUser is the keyring entry for a profile's Grant.
func refreshTokenUser(profile string) string { return profile + ":refresh-token" }

// Path is config.toml's location.
func (s Store) Path() string { return filepath.Join(s.Dir, "config.toml") }

// Save reports whether the secret went to the config file because the
// keyring was unusable.
func (s Store) Save(profile string, credentials Credentials) (inFile bool, err error) {
	config, err := s.read()
	if err != nil {
		return false, err
	}
	entry := fileProfile{ClientID: credentials.ClientID, OrganizationID: credentials.OrganizationID}
	if s.Keyring.Set(keyringService, profile, credentials.ClientSecret) != nil {
		entry.ClientSecret = credentials.ClientSecret
		inFile = true
	}
	config.Profiles[profile] = entry
	// A Grant this profile held before is replaced, and a stale Refresh
	// token must not outlive it.
	_ = s.Keyring.Delete(keyringService, refreshTokenUser(profile))
	return inFile, s.write(config)
}

// SaveGrant stores a Partner app's credentials and its Grant's first
// Refresh token for the profile. It reports whether a secret went to the
// config file because the keyring was unusable.
func (s Store) SaveGrant(profile string, credentials Credentials, refreshToken string) (inFile bool, err error) {
	config, err := s.read()
	if err != nil {
		return false, err
	}
	entry := fileProfile{ClientID: credentials.ClientID, OrganizationID: credentials.OrganizationID, PartnerApp: true}
	if s.Keyring.Set(keyringService, profile, credentials.ClientSecret) != nil {
		entry.ClientSecret = credentials.ClientSecret
		inFile = true
	}
	if s.Keyring.Set(keyringService, refreshTokenUser(profile), refreshToken) != nil {
		entry.RefreshToken = refreshToken
		inFile = true
	}
	config.Profiles[profile] = entry
	return inFile, s.write(config)
}

// SetOrganizationID records the Organization the profile's credentials are
// bound to.
func (s Store) SetOrganizationID(profile, organizationID string) error {
	config, err := s.read()
	if err != nil {
		return err
	}
	entry, ok := config.Profiles[profile]
	if !ok {
		return ErrNotLoggedIn
	}
	entry.OrganizationID = organizationID
	config.Profiles[profile] = entry
	return s.write(config)
}

// RefreshToken is the profile's current Grant Refresh token. Read it while
// holding LockGrant, since another tmh process may have just rotated it.
func (s Store) RefreshToken(profile string) (string, error) {
	config, err := s.read()
	if err != nil {
		return "", err
	}
	entry, ok := config.Profiles[profile]
	if !ok || !entry.PartnerApp {
		return "", errNoGrant(profile)
	}
	// The file copy wins: it exists only when the keyring refused the
	// latest token, so a keyring copy would be older.
	if entry.RefreshToken != "" {
		return entry.RefreshToken, nil
	}
	token, err := s.Keyring.Get(keyringService, refreshTokenUser(profile))
	if err != nil {
		return "", fmt.Errorf("%w: the %s profile's Refresh token is missing from the OS keyring (%v)",
			ErrNotLoggedIn, profile, err)
	}
	return token, nil
}

// SaveRefreshToken replaces the profile's Grant Refresh token with the one
// an exchange returned. An error means the store still holds the superseded
// token.
func (s Store) SaveRefreshToken(profile, token string) error {
	config, err := s.read()
	if err != nil {
		return err
	}
	entry, ok := config.Profiles[profile]
	if !ok || !entry.PartnerApp {
		return errNoGrant(profile)
	}
	if s.Keyring.Set(keyringService, refreshTokenUser(profile), token) == nil {
		if entry.RefreshToken == "" {
			return nil
		}
		entry.RefreshToken = ""
	} else {
		entry.RefreshToken = token
	}
	config.Profiles[profile] = entry
	return s.write(config)
}

// Load returns the profile's credentials, or an error wrapping
// ErrNotLoggedIn when it has none or its secret is gone from the keyring.
func (s Store) Load(profile string) (Credentials, error) {
	config, err := s.read()
	if err != nil {
		return Credentials{}, err
	}
	entry, ok := config.Profiles[profile]
	if !ok || entry.ClientID == "" {
		return Credentials{}, ErrNotLoggedIn
	}
	credentials := Credentials{
		ClientID:       entry.ClientID,
		ClientSecret:   entry.ClientSecret,
		OrganizationID: entry.OrganizationID,
		PartnerApp:     entry.PartnerApp,
	}
	if credentials.ClientSecret != "" {
		return credentials, nil
	}
	secret, err := s.Keyring.Get(keyringService, profile)
	if err != nil {
		return Credentials{}, fmt.Errorf("%w: the %s profile's client secret is missing from the OS keyring (%v)",
			ErrNotLoggedIn, profile, err)
	}
	credentials.ClientSecret = secret
	return credentials, nil
}

// Delete reports whether anything was stored for the profile.
func (s Store) Delete(profile string) (bool, error) {
	config, err := s.read()
	if err != nil {
		return false, err
	}
	keyringErr := s.Keyring.Delete(keyringService, profile)
	_ = s.Keyring.Delete(keyringService, refreshTokenUser(profile))
	_, inFile := config.Profiles[profile]
	if !inFile {
		return keyringErr == nil, nil
	}
	delete(config.Profiles, profile)
	return true, s.write(config)
}

// LocalSigningSecret is the profile's local Signing secret for
// `tmh events tail --forward-to`, created by generate on first use and kept
// in the keyring, or config.toml when the keyring is unusable.
func (s Store) LocalSigningSecret(profile string, generate func() (string, error)) (string, error) {
	user := profile + ":local-signing-secret"
	if secret, err := s.Keyring.Get(keyringService, user); err == nil {
		return secret, nil
	}
	config, err := s.read()
	if err != nil {
		return "", err
	}
	if secret := config.LocalSigningSecrets[profile]; secret != "" {
		return secret, nil
	}
	secret, err := generate()
	if err != nil {
		return "", err
	}
	if s.Keyring.Set(keyringService, user, secret) == nil {
		return secret, nil
	}
	if config.LocalSigningSecrets == nil {
		config.LocalSigningSecrets = map[string]string{}
	}
	config.LocalSigningSecrets[profile] = secret
	return secret, s.write(config)
}

func (s Store) read() (fileConfig, error) {
	config := fileConfig{}
	_, err := toml.DecodeFile(s.Path(), &config)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config, fmt.Errorf("reading %s: %w", s.Path(), err)
	}
	if config.Profiles == nil {
		config.Profiles = map[string]fileProfile{}
	}
	return config, nil
}

func (s Store) write(config fileConfig) error {
	return writeTOML(s.Dir, s.Path(), config)
}

// writeTOML replaces path atomically and durably; CreateTemp makes it 0600,
// so a secret is never readable by others, even briefly. A rotated Refresh
// token must survive a crash once the exchange returns (ADR-0004): the
// superseded one left behind would revoke the Grant when presented.
func writeTOML(dir, path string, value any) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "."+strings.TrimSuffix(filepath.Base(path), ".toml")+"-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := toml.NewEncoder(file).Encode(value); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}
