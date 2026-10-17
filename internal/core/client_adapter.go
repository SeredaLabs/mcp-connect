package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// ClientAdapter reports registrations, never a live MCP connection. Implementations
// own detection, formats, compatibility checks, mutation and verification.
// A successful Set must verify the requested state; it must not mutate other clients.
type ClientAdapter interface {
	ID() string
	Read(id string) (Registration, bool, error)
	Set(context.Context, RegistrationRequest) error
	Detect() string
	Settings(id string) (ClientSettings, error)
}

// RegistrationRequest carries application intent, independent of vendor formats.
type RegistrationRequest struct {
	ID, Name, Executable string
	Enabled              bool
}

// ClientSettings contains presentation data, not a registration mechanism.
type ClientSettings struct {
	Title, Instructions, ConfigPath, CopyText, ActionLabel, Unavailable string
	URL                                                                 *url.URL
	Direct, OpenOnShow                                                  bool
}

// Some clients can also see registrations shared by another integration.
// Errors here do not invalidate the client's own independent registration.
type sharedRegistration interface {
	ReadShared(id string) (Registration, bool, error)
}

type ClientBinding struct {
	Adapter  ClientAdapter
	Name     string
	Selected func(Connection) bool
}

type ClientRegistry struct {
	ordered  []string
	bindings map[string]ClientBinding
}

func NewClientRegistry(bindings ...ClientBinding) (*ClientRegistry, error) {
	r := &ClientRegistry{bindings: make(map[string]ClientBinding)}
	for _, binding := range bindings {
		if binding.Adapter == nil || binding.Adapter.ID() == "" || binding.Selected == nil {
			return nil, errors.New("Invalid client adapter binding")
		}
		id := binding.Adapter.ID()
		if _, exists := r.bindings[id]; exists {
			return nil, fmt.Errorf("Duplicate client adapter: %s", id)
		}
		r.ordered = append(r.ordered, id)
		r.bindings[id] = binding
	}
	return r, nil
}

func (r *ClientRegistry) IDs() []string { return append([]string(nil), r.ordered...) }
func (r *ClientRegistry) Lookup(id string) (ClientBinding, error) {
	if r != nil {
		if b, ok := r.bindings[id]; ok {
			return b, nil
		}
	}
	return ClientBinding{}, fmt.Errorf("Unknown client: %s", id)
}

func registrationPreflight(a ClientAdapter, request RegistrationRequest) (Registration, bool, bool, error) {
	if !idPattern.MatchString(request.ID) {
		return Registration{}, false, false, errors.New("Invalid integration ID")
	}
	old, exists, err := a.Read(request.ID)
	if err != nil {
		return old, exists, false, err
	}
	if exists && !owned(old, request.ID) {
		return old, exists, false, errors.New("Refusing to modify an external MCP entry")
	}
	done := (!request.Enabled && !exists) || (request.Enabled && exists && samePath(old.Command, request.Executable) && (old.Enabled == nil || *old.Enabled))
	return old, exists, done, nil
}
func verifyRegistration(a ClientAdapter, request RegistrationRequest) error {
	r, exists, err := a.Read(request.ID)
	if err != nil {
		return err
	}
	if request.Enabled && (!exists || !owned(r, request.ID) || !samePath(r.Command, request.Executable) || (r.Enabled != nil && !*r.Enabled)) {
		return errors.New(a.ID() + " registration verification failed")
	}
	if !request.Enabled && exists {
		return errors.New(a.ID() + " removal verification failed")
	}
	return nil
}
