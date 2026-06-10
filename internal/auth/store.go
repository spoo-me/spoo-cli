// Package auth manages spoo CLI credentials and the device-auth login flow.
package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "spoo-cli"
	keyringAccount = "credentials"
	credsFileName  = "credentials.json"
)

type Mode string

const (
	ModeDevice Mode = "device"  // JWT pair from the connected-apps device flow
	ModeAPIKey Mode = "api_key" // spoo_... API key (non-interactive / CI)
)

type Credentials struct {
	Mode         Mode   `json:"mode"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
}

var ErrNotLoggedIn = errors.New("not logged in — run `spoo auth login`")

// Store persists credentials in the OS keyring, falling back to a
// 0600 file in the config dir when no keyring is available.
type Store struct {
	dir string
}

func NewStore(configDir string) *Store { return &Store{dir: configDir} }

func (s *Store) file() string { return filepath.Join(s.dir, credsFileName) }

func (s *Store) Save(c Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, keyringAccount, string(data)); err == nil {
		return nil
	}
	return os.WriteFile(s.file(), data, 0o600)
}

func (s *Store) Load() (*Credentials, error) {
	if raw, err := keyring.Get(keyringService, keyringAccount); err == nil {
		return unmarshalCreds([]byte(raw))
	}
	data, err := os.ReadFile(s.file())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}
	return unmarshalCreds(data)
}

func (s *Store) Clear() error {
	_ = keyring.Delete(keyringService, keyringAccount)
	if err := os.Remove(s.file()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func unmarshalCreds(data []byte) (*Credentials, error) {
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
