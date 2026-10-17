package core

import (
	"context"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func codexContractFixture(t *testing.T) *adapterContractFixture {
	t.Helper()
	home := t.TempDir()
	host := &Integrator{Home: home, Find: func(string) string { return "fixture" }}
	a := NewCodexAdapter(host, filepath.Join(home, "config.toml"))
	id := NewID()
	calls := 0
	fail, skip := false, false
	seed := func(r Registration) {
		b, err := toml.Marshal(map[string]any{"model": "preserve", "mcp_servers": map[string]codexRegistration{id: codexRegistration(r)}})
		if err != nil {
			t.Fatal(err)
		}
		if err = AtomicWrite(a.configPath, b); err != nil {
			t.Fatal(err)
		}
	}
	host.Run = func(_ context.Context, bin string, args ...string) error {
		calls++
		if fail {
			return errors.New("upstream command unsupported")
		}
		if skip {
			return nil
		}
		switch args[1] {
		case "add":
			if len(args) != 7 || !reflect.DeepEqual(args[:4], []string{"mcp", "add", id, "--"}) || args[5] != "serve" || args[6] != id {
				return errors.New("unexpected Codex argv")
			}
			seed(Registration{Command: args[4], Args: []string{"serve", id}})
		case "remove":
			if !reflect.DeepEqual(args, []string{"mcp", "remove", id}) {
				return errors.New("unexpected remove")
			}
			return os.Remove(a.configPath)
		default:
			return errors.New("unknown command")
		}
		return nil
	}
	snapshot := func() string { b, _ := os.ReadFile(a.configPath); return string(b) }
	return &adapterContractFixture{a, host, RegistrationRequest{ID: id, Name: "Fixture", Executable: filepath.Join(home, "App With Spaces", "connect"), Enabled: true}, seed, func() { AtomicWrite(a.configPath, []byte("[mcp_servers broken")) }, snapshot, &calls, func() { fail = true }, func() { skip = true }}
}
func TestCodexSharedStateIsolation(t *testing.T) {
	f := codexContractFixture(t)
	a := f.adapter.(*CodexAdapter)
	a.shared = func(string) (Registration, bool, error) { return Registration{}, false, errors.New("broken plugin") }
	f.host.Clients, _ = NewClientRegistry(ClientBinding{a, "Codex", func(c Connection) bool { return c.Codex }})
	f.host.Executable = f.request.Executable
	if err := a.Set(context.Background(), f.request); err != nil {
		t.Fatal(err)
	}
	s := f.host.Inspect(a.ID(), f.request.ID)
	if !s.Registered || s.Err != nil || s.Shared {
		t.Fatal(s)
	}
	a.shared = func(string) (Registration, bool, error) {
		return Registration{Command: f.request.Executable, Args: []string{"serve", f.request.ID}}, true, nil
	}
	if !f.host.Inspect(a.ID(), f.request.ID).Shared {
		t.Fatal("shared registration lost")
	}
}

// Captured supported config shape; intentionally independent of the encoder.
const codexConfigFixture = `model = "preserve"
[mcp_servers.mcp-0123456789abcdef]
command = "/saved/connect"
args = ["serve", "mcp-0123456789abcdef"]
enabled = false
[mcp_servers.external]
command = "other-tool"
args = []
`

func TestCodexConfigFixture(t *testing.T) {
	home := t.TempDir()
	a := NewCodexAdapter(&Integrator{Home: home}, filepath.Join(home, "config.toml"))
	if err := AtomicWrite(a.configPath, []byte(codexConfigFixture)); err != nil {
		t.Fatal(err)
	}
	r, ok, err := a.Read("mcp-0123456789abcdef")
	if err != nil || !ok || r.Command != "/saved/connect" || r.Enabled == nil || *r.Enabled || !reflect.DeepEqual(r.Args, []string{"serve", "mcp-0123456789abcdef"}) {
		t.Fatal(r, ok, err)
	}
}
