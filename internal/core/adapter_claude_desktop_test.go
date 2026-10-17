package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func desktopContractFixture(t *testing.T) *adapterContractFixture {
	t.Helper()
	home := t.TempDir()
	host := &Integrator{Home: home, Find: func(string) string { return "fixture" }}
	a := NewClaudeDesktopAdapter(host, filepath.Join(home, "config.json"))
	id := NewID()
	calls := 0
	seed := func(r Registration) {
		b, _ := json.Marshal(map[string]any{"customSetting": true, "mcpServers": map[string]desktopRegistration{id: desktopRegistration(r)}})
		if err := AtomicWrite(a.configPath, b); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() string { b, _ := os.ReadFile(a.configPath); return string(b) }
	return &adapterContractFixture{adapter: a, host: host, request: RegistrationRequest{ID: id, Name: "Fixture", Executable: filepath.Join(home, "App With Spaces", "connect"), Enabled: true}, seed: seed, corrupt: func() { AtomicWrite(a.configPath, []byte("{invalid")) }, snapshot: snapshot, calls: &calls, fail: func() { a.writeFile = func(string, []byte) error { return errors.New("injected disk failure") } }, skipWrites: func() { a.writeFile = func(string, []byte) error { return nil } }}
}

const desktopConfigFixture = `{"preferences":{"theme":"dark"},"mcpServers":{"mcp-0123456789abcdef":{"command":"/saved/connect","args":["serve","mcp-0123456789abcdef"]},"external":{"command":"other-tool","args":[]}}}`

func TestDesktopConfigFixture(t *testing.T) {
	home := t.TempDir()
	a := NewClaudeDesktopAdapter(&Integrator{Home: home}, filepath.Join(home, "config.json"))
	if err := AtomicWrite(a.configPath, []byte(desktopConfigFixture)); err != nil {
		t.Fatal(err)
	}
	r, ok, err := a.Read("mcp-0123456789abcdef")
	if err != nil || !ok || r.Command != "/saved/connect" || len(r.Args) != 2 || r.Args[1] != "mcp-0123456789abcdef" {
		t.Fatal(r, ok, err)
	}
	for _, raw := range []string{"null", "[]", `{"mcpServers":null}`, `{"mcpServers":[]}`} {
		AtomicWrite(a.configPath, []byte(raw))
		if _, _, err := a.Read("mcp-0123456789abcdef"); err == nil {
			t.Fatal("invalid object accepted", raw)
		}
	}
}

func TestDesktopWriteFailurePreservesOriginal(t *testing.T) {
	f := desktopContractFixture(t)
	f.seed(Registration{Command: filepath.Join(f.host.Home, "old-connect"), Args: []string{"serve", f.request.ID}})
	before := f.snapshot()
	f.fail()
	if err := f.adapter.Set(context.Background(), f.request); err == nil {
		t.Fatal("failed write accepted")
	}
	if f.snapshot() != before {
		t.Fatal("failed write changed configuration")
	}
}
func TestDesktopRejectsConcurrentConfigurationChange(t *testing.T) {
	f := desktopContractFixture(t)
	a := f.adapter.(*ClaudeDesktopAdapter)
	original := []byte(`{"preferences":{"theme":"old"}}`)
	external := []byte(`{"preferences":{"theme":"external-edit"}}`)
	if err := AtomicWrite(a.configPath, original); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.readFile = func(path string) ([]byte, error) {
		calls++
		if calls == 3 {
			if err := AtomicWrite(path, external); err != nil {
				t.Fatal(err)
			}
		}
		return os.ReadFile(path)
	}
	writes := 0
	a.writeFile = func(string, []byte) error { writes++; return nil }
	if err := a.Set(context.Background(), f.request); err == nil || !strings.Contains(err.Error(), "configuration changed") {
		t.Fatal("concurrent change accepted", err)
	}
	after, _ := os.ReadFile(a.configPath)
	if writes != 0 || !bytes.Equal(after, external) {
		t.Fatal("external edit overwritten")
	}
}
func TestDesktopRemovalMustBeVerified(t *testing.T) {
	f := desktopContractFixture(t)
	if err := f.adapter.Set(context.Background(), f.request); err != nil {
		t.Fatal(err)
	}
	f.skipWrites()
	f.request.Enabled = false
	if err := f.adapter.Set(context.Background(), f.request); err == nil {
		t.Fatal("unverified removal accepted")
	}
}
