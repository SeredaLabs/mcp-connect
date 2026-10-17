package core

import (
	"context"
	"errors"
	"strings"
)

type ClientState struct {
	Compatibility ClientCompatibility
	Registered    bool
	Shared        bool
	Disabled      bool
	Repair        bool
	Err           error
}

// Inspect reports registration, not a live connection or tool availability.
func (i *Integrator) Inspect(client, id string) ClientState {
	a, err := i.adapter(client)
	if err != nil {
		return ClientState{Err: err}
	}
	r, exists, err := a.Read(id)
	s := ClientState{Err: err, Compatibility: CompatibilityUnknown}
	if err != nil {
		return s
	}
	if shared, ok := a.(sharedRegistration); ok {
		_, present, err := shared.ReadShared(id)
		if err == nil {
			s.Shared = present
		}
	}
	if !exists {
		return s
	}
	if !owned(r, id) {
		s.Err = errors.New("Name conflict: this entry belongs to another integration")
		return s
	}
	s.Registered = true
	s.Disabled = r.Enabled != nil && !*r.Enabled
	s.Repair = !samePath(r.Command, i.Executable)
	return s
}

// Registration verification is not certification of an upstream client version.
// Unknown versions remain unknown even when a known config shape can be read.
type ClientCompatibility string

const CompatibilityUnknown ClientCompatibility = "unknown"

type ClientResult struct {
	Client string
	name   string
	On     bool
	Err    error
}

type ApplyReport []ClientResult

func (r ApplyReport) FailedClients() []string {
	var failed []string
	for _, result := range r {
		if result.Err != nil {
			failed = append(failed, result.Client)
		}
	}
	return failed
}

func (r ApplyReport) Err() error {
	var messages []string
	for _, result := range r {
		if result.Err != nil {
			name := result.name
			if name == "" {
				name = ClientName(result.Client)
			}
			messages = append(messages, name+": "+result.Err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(messages, "\n"))
}

// Apply retries only the requested clients when a subset is supplied. Failures
// never remove or prevent successful independent registrations.
func (i *Integrator) Apply(ctx context.Context, c Connection, clients ...string) ApplyReport {
	if i.Clients == nil {
		return ApplyReport{{Err: errors.New("Client registry unavailable")}}
	}
	if len(clients) == 0 {
		clients = i.Clients.IDs()
	}
	var report ApplyReport
	for _, client := range clients {
		binding, err := i.Clients.Lookup(client)
		if err != nil {
			report = append(report, ClientResult{Client: client, Err: err})
			continue
		}
		on := c.Enabled && binding.Selected(c)
		err = i.set(ctx, client, RegistrationRequest{ID: c.ID, Name: c.Name, Executable: i.Executable, Enabled: on})
		report = append(report, ClientResult{Client: client, name: binding.Name, On: on, Err: err})
	}
	return report
}
