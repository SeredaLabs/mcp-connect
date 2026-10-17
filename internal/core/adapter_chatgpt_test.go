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

// No TOML or Codex config paths in ChatGPT fixtures: a storage-format migration
// changes only the Codex backend fixture. This fake speaks backend operations.
type pluginStoreFixture struct {
	home, id                      string
	active, fail, skip, badOutput bool
	calls                         int
	t                             *testing.T
}

func (s *pluginStoreFixture) pluginHome() string { return s.home }
func (s *pluginStoreFixture) Detect() string     { return "fixture" }
func (s *pluginStoreFixture) pluginState(market, selector, root string) (bool, error) {
	return s.active, nil
}
func (s *pluginStoreFixture) pluginOutput(_ context.Context, bin string, args ...string) ([]byte, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("unsupported plugin command")
	}
	if s.badOutput {
		return []byte(`{"changedResult":true}`), nil
	}
	if reflect.DeepEqual(args, []string{"plugin", "marketplace", "add", filepath.Join(s.home, "mcp-connect", "marketplaces", s.id), "--json"}) {
		return []byte(`{}`), nil
	}
	if reflect.DeepEqual(args, []string{"plugin", "remove", pluginSelector(s.id), "--json"}) {
		s.active = false
		return []byte(`{}`), nil
	}
	if !reflect.DeepEqual(args, []string{"plugin", "add", pluginSelector(s.id), "--json"}) {
		return nil, errors.New("unexpected plugin command")
	}
	installed := filepath.Join(s.home, "plugins", "cache", pluginMarket(s.id), s.id, "fixture-version")
	if !s.skip {
		b, err := os.ReadFile(filepath.Join(s.home, "mcp-connect", "marketplaces", s.id, "plugins", s.id, ".mcp.json"))
		if err != nil {
			return nil, err
		}
		if err = AtomicWrite(filepath.Join(installed, ".mcp.json"), b); err != nil {
			return nil, err
		}
		s.active = true
	}
	return json.Marshal(map[string]string{"pluginId": pluginSelector(s.id), "installedPath": installed})
}
func chatGPTContractFixture(t *testing.T) *adapterContractFixture {
	t.Helper()
	home := t.TempDir()
	host := &Integrator{Home: home, Find: func(string) string { return "fixture" }, Output: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("backend must own invocation")
		return nil, nil
	}}
	id := NewID()
	store := &pluginStoreFixture{home: home, id: id, t: t}
	a := &ChatGPTAdapter{host: host, backend: store}
	write := func(path string, value any) {
		b, _ := json.Marshal(value)
		if err := AtomicWrite(path, b); err != nil {
			t.Fatal(err)
		}
	}
	seed := func(r Registration) {
		installed := filepath.Join(home, "plugins", "cache", pluginMarket(id), id, "fixture-version")
		write(filepath.Join(a.pluginRoot(id), "owner.json"), pluginReceipt{Service, id, installed})
		doc := map[string]any{"mcpServers": map[string]pluginRegistration{id: pluginRegistration(r)}}
		write(filepath.Join(a.pluginRoot(id), "plugins", id, ".mcp.json"), doc)
		write(filepath.Join(installed, ".mcp.json"), doc)
		store.active = true
	}
	snapshot := func() string {
		var result string
		filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(path)
				result += path + string(b)
			}
			return nil
		})
		return result
	}
	return &adapterContractFixture{a, host, RegistrationRequest{ID: id, Name: "Fixture", Executable: filepath.Join(home, "App With Spaces", "connect"), Enabled: true}, seed, func() { write(filepath.Join(a.pluginRoot(id), "owner.json"), map[string]any{"invalid": true}) }, snapshot, &store.calls, func() { store.fail = true }, func() { store.skip = true }}
}
func TestChatGPTChangedInstallationOutput(t *testing.T) {
	f := chatGPTContractFixture(t)
	a := f.adapter.(*ChatGPTAdapter)
	a.backend.(*pluginStoreFixture).badOutput = true
	if err := a.Set(context.Background(), f.request); err == nil {
		t.Fatal("unknown output accepted")
	}
}
func TestChatGPTUnsafeManagedAncestor(t *testing.T) {
	f := chatGPTContractFixture(t)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(f.host.Home, "mcp-connect")); err != nil {
		t.Skip(err)
	}
	if err := f.adapter.Set(context.Background(), f.request); err == nil {
		t.Fatal("unsafe ancestor accepted")
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatal("followed unsafe ancestor", entries, err)
	}
}

func TestChatGPTManifestFixture(t *testing.T) {
	f := chatGPTContractFixture(t)
	if err := f.adapter.Set(context.Background(), f.request); err != nil {
		t.Fatal(err)
	}
	a := f.adapter.(*ChatGPTAdapter)
	b, err := os.ReadFile(filepath.Join(a.pluginRoot(f.request.ID), "plugins", f.request.ID, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != f.request.ID || doc["mcpServers"] != "./.mcp.json" || doc["requires_local_executor"] != true {
		t.Fatal("changed manifest contract", doc)
	}
	iface, ok := doc["interface"].(map[string]any)
	if !ok || iface["displayName"] != "MCP · Fixture" {
		t.Fatal("friendly name lost", iface)
	}
}
