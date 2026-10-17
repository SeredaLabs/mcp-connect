package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func claudeContractFixture(t *testing.T) *adapterContractFixture {
	t.Helper()
	home := t.TempDir()
	host := &Integrator{Home: home, Find: func(string) string { return "fixture" }}
	a := NewClaudeAdapter(host, filepath.Join(home, "config.json"))
	id := NewID()
	calls := 0
	fail, skip := false, false
	seed := func(r Registration) {
		b, _ := json.Marshal(map[string]any{"customSetting": true, "mcpServers": map[string]claudeRegistration{id: claudeRegistration(r)}})
		if err := AtomicWrite(a.configPath, b); err != nil {
			t.Fatal(err)
		}
	}
	host.Run = func(_ context.Context, bin string, args ...string) error {
		calls++
		if fail {
			return errors.New("unsupported CLI")
		}
		if skip {
			return nil
		}
		switch args[1] {
		case "add":
			if len(args) != 11 || !reflect.DeepEqual(args[:8], []string{"mcp", "add", "--scope", "user", "--transport", "stdio", id, "--"}) || args[9] != "serve" || args[10] != id {
				return errors.New("unexpected Claude argv")
			}
			seed(Registration{Command: args[8], Args: []string{"serve", id}})
		case "remove":
			if !reflect.DeepEqual(args, []string{"mcp", "remove", "--scope", "user", id}) {
				return errors.New("unexpected Claude removal")
			}
			return os.Remove(a.configPath)
		default:
			return errors.New("unknown command")
		}
		return nil
	}
	snapshot := func() string { b, _ := os.ReadFile(a.configPath); return string(b) }
	return &adapterContractFixture{a, host, RegistrationRequest{ID: id, Name: "Fixture", Executable: filepath.Join(home, "App With Spaces", "connect"), Enabled: true}, seed, func() { AtomicWrite(a.configPath, []byte("{invalid")) }, snapshot, &calls, func() { fail = true }, func() { skip = true }}
}
func TestClaudeReplacementRollback(t *testing.T) {
	f := claudeContractFixture(t)
	old := Registration{Command: filepath.Join(f.host.Home, "old-connect"), Args: []string{"serve", f.request.ID}}
	f.seed(old)
	original := f.host.Run
	f.host.Run = func(ctx context.Context, bin string, args ...string) error {
		if args[1] == "add" && args[8] == f.request.Executable {
			return errors.New("incompatible version")
		}
		return original(ctx, bin, args...)
	}
	if err := f.adapter.Set(context.Background(), f.request); err == nil {
		t.Fatal("failure swallowed")
	}
	r, ok, err := f.adapter.Read(f.request.ID)
	if err != nil || !ok || !samePath(r.Command, old.Command) {
		t.Fatal("rollback lost", r, ok, err)
	}
}

const claudeConfigFixture = `{"customSetting":true,"mcpServers":{"mcp-0123456789abcdef":{"command":"/saved/connect","args":["serve","mcp-0123456789abcdef"]},"external":{"command":"other-tool","args":[]}}}`

func TestClaudeConfigFixture(t *testing.T) {
	home := t.TempDir()
	a := NewClaudeAdapter(&Integrator{Home: home}, filepath.Join(home, "config.json"))
	if err := AtomicWrite(a.configPath, []byte(claudeConfigFixture)); err != nil {
		t.Fatal(err)
	}
	r, ok, err := a.Read("mcp-0123456789abcdef")
	if err != nil || !ok || r.Command != "/saved/connect" || !reflect.DeepEqual(r.Args, []string{"serve", "mcp-0123456789abcdef"}) {
		t.Fatal(r, ok, err)
	}
	for _, raw := range []string{"null", "[]", `{"mcpServers":null}`, `{"mcpServers":[]}`} {
		AtomicWrite(a.configPath, []byte(raw))
		if _, _, err := a.Read("mcp-0123456789abcdef"); err == nil {
			t.Fatal("invalid object accepted", raw)
		}
	}
}
