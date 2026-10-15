package core

import (
	"encoding/json"
	"errors"
	keychain "github.com/keybase/go-keychain"
)

func (OSVault) Get(id string) (Credentials, error) {
	b, e := keychain.GetGenericPassword(Service, id, "", "")
	if e == keychain.ErrorItemNotFound || (e == nil && len(b) == 0) {
		return Credentials{}, ErrSecretMissing
	}
	if e != nil {
		return Credentials{}, errors.New("Keychain unavailable, locked, or access denied")
	}
	var c Credentials
	if json.Unmarshal(b, &c) != nil {
		return c, errors.New("Invalid Keychain record")
	}
	return c, nil
}
func (OSVault) Set(id string, c Credentials) error {
	b, _ := json.Marshal(c)
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(Service)
	item.SetAccount(id)
	item.SetLabel("MCP Connect · " + id)
	item.SetData(b)
	item.SetAccessible(keychain.AccessibleWhenUnlockedThisDeviceOnly)
	e := keychain.AddItem(item)
	if e == keychain.ErrorDuplicateItem {
		q := keychain.NewItem()
		q.SetSecClass(keychain.SecClassGenericPassword)
		q.SetService(Service)
		q.SetAccount(id)
		update := keychain.NewItem()
		update.SetData(b)
		e = keychain.UpdateItem(q, update)
	}
	if e != nil {
		return errors.New("Cannot save to Keychain; unlock it and allow access")
	}
	return nil
}
func (OSVault) Delete(id string) error {
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(Service)
	q.SetAccount(id)
	e := keychain.DeleteItem(q)
	if e == keychain.ErrorItemNotFound {
		return ErrSecretMissing
	}
	if e != nil {
		return errors.New("Cannot delete Keychain record")
	}
	return nil
}
