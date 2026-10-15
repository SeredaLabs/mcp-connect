//go:build !darwin

package core

import (
	"encoding/json"
	"errors"
	"github.com/zalando/go-keyring"
)

func (OSVault) Get(id string) (Credentials, error) {
	v, e := keyring.Get(Service, id)
	if errors.Is(e, keyring.ErrNotFound) {
		return Credentials{}, ErrSecretMissing
	}
	if e != nil {
		return Credentials{}, errors.New("System secret store unavailable or locked")
	}
	var c Credentials
	if json.Unmarshal([]byte(v), &c) != nil {
		return c, errors.New("Invalid credential record")
	}
	return c, nil
}
func (OSVault) Set(id string, c Credentials) error {
	b, _ := json.Marshal(c)
	if e := keyring.Set(Service, id, string(b)); e != nil {
		if errors.Is(e, keyring.ErrSetDataTooBig) {
			return errors.New("Credential record exceeds the system secret store size limit")
		}
		return errors.New("Cannot save to system secret store; no plaintext fallback")
	}
	return nil
}
func (OSVault) Delete(id string) error {
	e := keyring.Delete(Service, id)
	if errors.Is(e, keyring.ErrNotFound) {
		return ErrSecretMissing
	}
	if e != nil {
		return errors.New("Cannot delete from system secret store")
	}
	return nil
}
