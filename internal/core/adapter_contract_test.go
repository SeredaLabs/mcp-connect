package core

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

// The shared suite contains no vendor paths, commands, manifests or formats.
// Only the affected fixture factory changes when an upstream format changes.
type adapterContractFixture struct {
	adapter    ClientAdapter
	host       *Integrator
	request    RegistrationRequest
	seed       func(Registration) // nil Args is an external entry
	corrupt    func()
	snapshot   func() string
	calls      *int
	fail       func()
	skipWrites func()
}

func TestAdapterContract(t *testing.T) {
	factories := []struct {
		name   string
		create func(*testing.T) *adapterContractFixture
	}{
		{"codex", codexContractFixture}, {"claude", claudeContractFixture},
		{"claude-desktop", desktopContractFixture}, {"chatgpt", chatGPTContractFixture},
	}
	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("lifecycle", func(t *testing.T) {
				f := factory.create(t)
				ctx := context.Background()
				if _, ok, err := f.adapter.Read(f.request.ID); err != nil || ok {
					t.Fatal("fresh state", ok, err)
				}
				if err := f.adapter.Set(ctx, f.request); err != nil {
					t.Fatal(err)
				}
				before := f.snapshot()
				if err := f.adapter.Set(ctx, f.request); err != nil {
					t.Fatal(err)
				}
				if f.snapshot() != before {
					t.Fatal("repeat changed registration")
				}
				r, ok, err := f.adapter.Read(f.request.ID)
				if err != nil || !ok || !owned(r, f.request.ID) || !samePath(r.Command, f.request.Executable) {
					t.Fatal("unverified success", r, ok, err)
				}
				f.request.Executable = filepath.Join(f.host.Home, "New App With Spaces", "connect")
				if err := f.adapter.Set(ctx, f.request); err != nil {
					t.Fatal("repair", err)
				}
				r, ok, err = f.adapter.Read(f.request.ID)
				if err != nil || !ok || !samePath(r.Command, f.request.Executable) {
					t.Fatal("repair result", r, err)
				}
				f.request.Enabled = false
				if err := f.adapter.Set(ctx, f.request); err != nil {
					t.Fatal("remove", err)
				}
				if _, ok, err = f.adapter.Read(f.request.ID); err != nil || ok {
					t.Fatal("removal result", ok, err)
				}
				if err := f.adapter.Set(ctx, f.request); err != nil {
					t.Fatal("repeat removal", err)
				}
			})
			t.Run("external-entry", func(t *testing.T) {
				f := factory.create(t)
				f.seed(Registration{Command: "external"})
				before := f.snapshot()
				for _, on := range []bool{true, false} {
					f.request.Enabled = on
					if err := f.adapter.Set(context.Background(), f.request); err == nil {
						t.Fatal("external entry accepted")
					}
					if f.snapshot() != before {
						t.Fatal("external changed")
					}
				}
			})
			t.Run("invalid-config", func(t *testing.T) {
				f := factory.create(t)
				f.corrupt()
				before := f.snapshot()
				if err := f.adapter.Set(context.Background(), f.request); err == nil {
					t.Fatal("invalid config accepted")
				}
				if f.snapshot() != before {
					t.Fatal("invalid config changed")
				}
			})
			t.Run("missing-client", func(t *testing.T) {
				f := factory.create(t)
				f.host.Find = func(string) string { return "" }
				if err := f.adapter.Set(context.Background(), f.request); err == nil {
					t.Fatal("missing client accepted")
				}
			})
			t.Run("invalid-id", func(t *testing.T) {
				f := factory.create(t)
				f.request.ID = "../external"
				if err := f.adapter.Set(context.Background(), f.request); err == nil {
					t.Fatal("invalid ID accepted")
				}
				if _, _, err := f.adapter.Read(f.request.ID); err == nil {
					t.Fatal("invalid read ID accepted")
				}
			})
			{
				t.Run("mutation-failure", func(t *testing.T) {
					f := factory.create(t)
					f.fail()
					if err := f.adapter.Set(context.Background(), f.request); err == nil {
						t.Fatal("failure accepted")
					}
				})
				t.Run("verification-failure", func(t *testing.T) {
					f := factory.create(t)
					f.skipWrites()
					if err := f.adapter.Set(context.Background(), f.request); err == nil {
						t.Fatal("unverified registration accepted")
					}
				})
			}
			t.Run("unknown-version", func(t *testing.T) {
				f := factory.create(t)
				f.host.Clients, _ = NewClientRegistry(ClientBinding{f.adapter, "fixture", func(Connection) bool { return true }})
				if got := f.host.Inspect(f.adapter.ID(), f.request.ID).Compatibility; got != CompatibilityUnknown {
					t.Fatal("unverified version", got)
				}
			})
		})
	}
}

type additionalAdapter struct {
	fail     bool
	requests []RegistrationRequest
}

func (a *additionalAdapter) ID() string { return "future-client" }
func (a *additionalAdapter) Read(string) (Registration, bool, error) {
	return Registration{}, false, nil
}
func (a *additionalAdapter) Set(_ context.Context, r RegistrationRequest) error {
	a.requests = append(a.requests, r)
	if a.fail {
		return errors.New("incompatible upstream")
	}
	return nil
}
func (a *additionalAdapter) Detect() string                          { return "fixture" }
func (a *additionalAdapter) Settings(string) (ClientSettings, error) { return ClientSettings{}, nil }
func TestRegistryAndExtensibility(t *testing.T) {
	a := &additionalAdapter{}
	binding := ClientBinding{a, "Future Client", func(Connection) bool { return true }}
	r, err := NewClientRegistry(binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Lookup("missing"); err == nil {
		t.Fatal("unknown lookup")
	}
	if _, err = NewClientRegistry(binding, binding); err == nil {
		t.Fatal("duplicate accepted")
	}
	ids := r.IDs()
	ids[0] = "changed"
	if !reflect.DeepEqual(r.IDs(), []string{a.ID()}) {
		t.Fatal("mutable registry order")
	}
	i := &Integrator{Clients: r, Executable: filepath.Join(t.TempDir(), "connect")}
	c := Connection{ID: NewID(), Name: "Example", Enabled: true}
	report := i.Apply(context.Background(), c)
	if report.Err() != nil || len(a.requests) != 1 || a.requests[0].Name != c.Name || !a.requests[0].Enabled {
		t.Fatal(report, a.requests)
	}
	if err = i.Set(context.Background(), "missing", c.ID, true); err == nil {
		t.Fatal("unknown Set")
	}
	if _, _, err = i.Read("missing", c.ID); err == nil {
		t.Fatal("unknown Read")
	}
	if i.Inspect("missing", c.ID).Err == nil {
		t.Fatal("unknown Inspect")
	}
	a.fail = true
	report = i.Apply(context.Background(), c, "missing", a.ID())
	if len(report) != 2 || len(a.requests) != 2 || report[0].Err == nil || report[1].Err == nil {
		t.Fatal(report)
	}
}
