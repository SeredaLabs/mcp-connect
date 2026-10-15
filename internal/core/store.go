package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Version = "0.4.4"
const Service = "eu.onecmcp.connect"

type Credentials struct {
	AuthType   string `json:"auth_type,omitempty"`
	AuthHeader string `json:"auth_header,omitempty"`
	Token      string `json:"token,omitempty"`
	Login      string `json:"login"`
	Password   string `json:"password"`
	Endpoint   string `json:"endpoint,omitempty"`
}
type Vault interface {
	Get(string) (Credentials, error)
	Set(string, Credentials) error
	Delete(string) error
}
type Connection struct {
	AuthType      string `json:"auth_type,omitempty"`
	AuthHeader    string `json:"auth_header,omitempty"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Claude        bool   `json:"claude"`
	ClaudeDesktop bool   `json:"claude_desktop,omitempty"`
	Codex         bool   `json:"codex"`
	ChatGPT       bool   `json:"chatgpt,omitempty"`
	Enabled       bool   `json:"enabled"`
}
type Store struct {
	Dir   string
	Vault Vault
}

var idPattern = regexp.MustCompile(`^(onecmcp|mcp)-[a-f0-9]{16}$`)

func NewID() string {
	var b [8]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return "mcp-" + hex.EncodeToString(b[:])
}
func DefaultStore() (*Store, error) {
	d, e := os.UserConfigDir()
	if e != nil {
		return nil, e
	}
	return &Store{connectionDir(d), OSVault{}}, nil
}

func connectionDir(d string) string {
	dir := filepath.Join(d, "MCP Connect")
	if _, err := os.Stat(filepath.Join(dir, "connections.json")); os.IsNotExist(err) {
		legacy := filepath.Join(d, "1C MCP Connect")
		if _, err := os.Stat(filepath.Join(legacy, "connections.json")); err == nil {
			dir = legacy
		}
	}
	return dir
}
func Validate(c Connection) error {
	if !idPattern.MatchString(c.ID) {
		return errors.New("Invalid connection ID")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("Enter a connection name")
	}
	if err := validateAuthConfig(c.AuthType, c.AuthHeader); err != nil {
		return err
	}
	return ValidateURL(c.URL)
}
func ValidateURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("Use an HTTPS URL without credentials, query parameters or fragments")
	}
	return nil
}
func (s *Store) Load() ([]Connection, error) {
	b, e := os.ReadFile(filepath.Join(s.Dir, "connections.json"))
	if os.IsNotExist(e) {
		return []Connection{}, nil
	}
	if e != nil {
		return nil, e
	}
	var cs []Connection
	if e = json.Unmarshal(b, &cs); e != nil {
		return nil, errors.New("Connection file is invalid; it has not been modified")
	}
	seen := map[string]bool{}
	for _, c := range cs {
		if e = Validate(c); e != nil {
			return nil, e
		}
		if seen[c.ID] {
			return nil, errors.New("Duplicate connection ID")
		}
		seen[c.ID] = true
	}
	return cs, nil
}
func (s *Store) Find(id string) (Connection, error) {
	cs, e := s.Load()
	if e != nil {
		return Connection{}, e
	}
	for _, c := range cs {
		if c.ID == id {
			return c, nil
		}
	}
	return Connection{}, errors.New("Connection not found")
}
func (s *Store) Write(cs []Connection) error {
	for _, c := range cs {
		if e := Validate(c); e != nil {
			return e
		}
	}
	b, e := json.MarshalIndent(cs, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(filepath.Join(s.Dir, "connections.json"), b)
}
func AtomicWrite(path string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".connect-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func (s *Store) Save(c Connection, cred Credentials) error {
	if e := Validate(c); e != nil {
		return e
	}
	if err := ValidateCredentials(c, cred); err != nil {
		return err
	}
	cs, e := s.Load()
	if e != nil {
		return e
	}
	old, oldErr := s.Vault.Get(c.ID)
	if oldErr != nil && !errors.Is(oldErr, ErrSecretMissing) {
		return oldErr
	}
	cred.Endpoint = c.URL
	if e = s.Vault.Set(c.ID, cred); e != nil {
		return e
	}
	found := false
	for i := range cs {
		if cs[i].ID == c.ID {
			cs[i] = c
			found = true
		}
	}
	if !found {
		cs = append(cs, c)
	}
	if e = s.Write(cs); e != nil {
		if oldErr == nil {
			_ = s.Vault.Set(c.ID, old)
		} else {
			_ = s.Vault.Delete(c.ID)
		}
		return fmt.Errorf("Could not save connection: %w", e)
	}
	return nil
}
func (s *Store) Remove(id string) error {
	cs, e := s.Load()
	if e != nil {
		return e
	}
	if e = s.Vault.Delete(id); e != nil && !errors.Is(e, ErrSecretMissing) {
		return e
	}
	out := []Connection{}
	for _, c := range cs {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return s.Write(out)
}

// ResolveCredentials refuses to reuse a saved secret for a changed endpoint.
// Legacy records must be explicitly re-entered before gaining endpoint binding.
func (s *Store) ResolveCredentials(c Connection, input Credentials) (Credentials, error) {
	if err := validateAuthConfig(c.AuthType, c.AuthHeader); err != nil {
		return Credentials{}, err
	}
	if input.Endpoint != "" && input.Endpoint != c.URL {
		return Credentials{}, errors.New("Server URL changed; enter credentials again for this endpoint")
	}
	if !sameAuth(c.AuthType, c.AuthHeader, input.AuthType, input.AuthHeader) {
		return Credentials{}, errors.New("Authentication method changed; enter credentials again")
	}
	needsSaved := input.Token == ""
	if AuthMode(c.AuthType) == AuthBasic {
		needsSaved = input.Login == "" || input.Password == ""
	}
	if needsSaved {
		saved, err := s.Find(c.ID)
		if err != nil {
			return Credentials{}, errors.New("Enter credentials for the new connection")
		}
		if saved.URL != c.URL || !sameAuth(saved.AuthType, saved.AuthHeader, c.AuthType, c.AuthHeader) {
			return Credentials{}, errors.New("Server URL, authentication method or key header changed; enter credentials again")
		}
		old, err := s.Vault.Get(c.ID)
		if err != nil {
			return Credentials{}, err
		}
		if old.Endpoint == "" {
			return Credentials{}, errors.New("Legacy credentials are not bound to a server; enter them again and use Save & Connect")
		}
		if err := ValidateCredentials(c, old); err != nil {
			return Credentials{}, err
		}
		if AuthMode(c.AuthType) == AuthBasic {
			if input.Login == "" {
				input.Login = old.Login
			}
			if input.Password == "" {
				input.Password = old.Password
			}
		} else {
			input.Token = old.Token
		}
	}
	input.Endpoint = c.URL
	if err := ValidateCredentials(c, input); err != nil {
		return Credentials{}, err
	}
	return input, nil
}
