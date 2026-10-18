package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Registration struct {
	Command string
	Args    []string
	Enabled *bool
}
type Integrator struct {
	DisplayName, Home, Executable string
	Clients                       *ClientRegistry
	Run                           func(context.Context, string, ...string) error
	Output                        func(context.Context, string, ...string) ([]byte, error)
	Find                          func(string) string
}

func NewIntegrator() (*Integrator, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return nil, err
	}
	i := &Integrator{Home: home, Executable: exe}
	i.Run = func(ctx context.Context, bin string, args ...string) error {
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = home
		cmd.Env = clientEnvironment(bin)
		return cmd.Run()
	}
	i.Output = func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = home
		cmd.Env = clientEnvironment(bin)
		return cmd.Output()
	}
	i.Clients = defaultClients(i)
	return i, nil
}
func (i *Integrator) adapter(client string) (ClientAdapter, error) {
	if i.Clients == nil {
		return nil, errors.New("Client registry unavailable")
	}
	binding, err := i.Clients.Lookup(client)
	return binding.Adapter, err
}
func (i *Integrator) find(a ClientAdapter) string {
	if i.Find != nil {
		return i.Find(a.ID())
	}
	return a.Detect()
}
func (i *Integrator) Read(client, id string) (Registration, bool, error) {
	a, err := i.adapter(client)
	if err != nil {
		return Registration{}, false, err
	}
	return a.Read(id)
}
func owned(r Registration, id string) bool {
	return len(r.Args) == 2 && r.Args[0] == "serve" && r.Args[1] == id && filepath.IsAbs(r.Command)
}
func (i *Integrator) Set(ctx context.Context, client, id string, on bool) error {
	return i.set(ctx, client, RegistrationRequest{ID: id, Name: i.DisplayName, Executable: i.Executable, Enabled: on})
}
func (i *Integrator) set(ctx context.Context, client string, r RegistrationRequest) error {
	a, err := i.adapter(client)
	if err != nil {
		return err
	}
	return a.Set(ctx, r)
}
func (i *Integrator) Settings(client, id string) (ClientSettings, error) {
	a, err := i.adapter(client)
	if err != nil {
		return ClientSettings{}, err
	}
	return a.Settings(id)
}

func (i *Integrator) Status(client, id string) string {
	a, err := i.adapter(client)
	if err != nil {
		return err.Error()
	}
	r, ok, err := a.Read(id)
	if err == nil && !ok {
		if shared, yes := a.(sharedRegistration); yes {
			r, ok, err = shared.ReadShared(id)
		}
	}
	if err != nil {
		return err.Error()
	}
	if !ok {
		return "Not configured"
	}
	if !owned(r, id) {
		return "Name conflict — external entry"
	}
	if !samePath(r.Command, i.Executable) {
		if f, err := os.Stat(r.Command); err == nil && !f.IsDir() {
			return "Configured · saved adapter"
		}
		return "App moved — Save & Apply to repair"
	}
	if i.find(a) == "" {
		settings, _ := a.Settings(id)
		if settings.Unavailable != "" {
			return settings.Unavailable
		}
		return "Configured · client CLI unavailable"
	}
	return "Configured"
}
func (i *Integrator) Sync(ctx context.Context, c Connection) error {
	return i.Apply(ctx, c).Err()
}
func (s *Store) Disable(ctx context.Context, i *Integrator, c Connection) error {
	report, err := s.DisableClients(ctx, i, c)
	if err != nil {
		return err
	}
	if err := report.Err(); err != nil {
		return fmt.Errorf("Profile disabled. Some client entries could not be removed; retry Disconnect. Existing sessions must be closed.\n%w", err)
	}
	return nil
}

// DisableClients persists the disabled profile before attempting any cleanup.
// The report distinguishes cleanup failures from a failed local save.
func (s *Store) DisableClients(ctx context.Context, i *Integrator, c Connection) (ApplyReport, error) {
	cs, e := s.Load()
	if e != nil {
		return nil, e
	}
	found := false
	for n := range cs {
		if cs[n].ID == c.ID {
			c = cs[n]
			c.Enabled = false
			cs[n] = c
			found = true
		}
	}
	if !found {
		return nil, errors.New("Connection not found")
	}
	if e := s.Write(cs); e != nil {
		return nil, e
	}
	return i.Apply(ctx, c), nil
}
