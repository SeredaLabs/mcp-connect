package core

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func desktopIntegrator(t *testing.T) *Integrator {
	t.Helper()
	dir := t.TempDir()
	return fixtureIntegrator(integrationFixture{Home: dir, Executable: filepath.Join(dir, "App With Spaces", "1c-mcp-connect"), ClaudeDesktopPath: filepath.Join(dir, "Claude", "claude_desktop_config.json"), Find: func(string) string { return "desktop-installed" }})
}
func TestDesktopRegistrationPreservesOtherSettings(t *testing.T) {
	i := desktopIntegrator(t)
	id := NewID()
	original := []byte(`{"preferences":{"language":"uk","nested":[1,true,"keep"]},"mcpServers":{"external":{"command":"external-tool","args":["keep"],"env":{"TEST_VALUE":"preserve-this"}}}}`)
	if e := AtomicWrite(fixturePath(i, "claude-desktop"), original); e != nil {
		t.Fatal(e)
	}
	if e := i.Set(context.Background(), "claude-desktop", id, true); e != nil {
		t.Fatal(e)
	}
	first, e := os.ReadFile(fixturePath(i, "claude-desktop"))
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]json.RawMessage
	json.Unmarshal(first, &doc)
	var servers map[string]json.RawMessage
	json.Unmarshal(doc["mcpServers"], &servers)
	if !bytes.Contains(servers["external"], []byte("preserve-this")) || !bytes.Contains(doc["preferences"], []byte("nested")) {
		t.Fatal("Unrelated settings lost")
	}
	if e = i.Set(context.Background(), "claude-desktop", id, true); e != nil {
		t.Fatal(e)
	}
	second, _ := os.ReadFile(fixturePath(i, "claude-desktop"))
	if !bytes.Equal(first, second) {
		t.Fatal("Repeat registration rewrote configuration")
	}
	i.Executable = filepath.Join(i.Home, "New Location", "connect")
	if e = i.Set(context.Background(), "claude-desktop", id, true); e != nil {
		t.Fatal(e)
	}
	r, ok, e := i.Read("claude-desktop", id)
	if e != nil || !ok || r.Command != i.Executable {
		t.Fatal("Moved application not repaired", e)
	}
	i.Find = func(string) string { return "" }
	if e = i.Set(context.Background(), "claude-desktop", id, false); e != nil {
		t.Fatal("Removal should work even after app uninstall", e)
	}
	third, _ := os.ReadFile(fixturePath(i, "claude-desktop"))
	json.Unmarshal(third, &doc)
	json.Unmarshal(doc["mcpServers"], &servers)
	if bytes.Contains(third, []byte(id)) || !bytes.Contains(third, []byte("preserve-this")) {
		t.Fatal("Removal changed unrelated settings")
	}
}
func TestDesktopRejectsConflictsAndInvalidJSON(t *testing.T) {
	i := desktopIntegrator(t)
	id := NewID()
	cases := [][]byte{[]byte(`{"mcpServers":{"` + id + `":{"command":"external","args":[]}}}`), []byte(`broken`), []byte(`null`), []byte(`{"mcpServers":[]}`)}
	for _, original := range cases {
		if e := AtomicWrite(fixturePath(i, "claude-desktop"), original); e != nil {
			t.Fatal(e)
		}
		if e := i.Set(context.Background(), "claude-desktop", id, true); e == nil {
			t.Fatal("Invalid/conflicting configuration accepted")
		}
		got, _ := os.ReadFile(fixturePath(i, "claude-desktop"))
		if !bytes.Equal(got, original) {
			t.Fatal("Invalid config was overwritten")
		}
	}
}
func TestDesktopOnlyConnectionSync(t *testing.T) {
	i := desktopIntegrator(t)
	id := NewID()
	c := Connection{ID: id, Name: "Desktop only", URL: "https://example.com/mcp", Enabled: true, ClaudeDesktop: true}
	if e := i.Sync(context.Background(), c); e != nil {
		t.Fatal(e)
	}
	r, ok, e := i.Read("claude-desktop", id)
	if e != nil || !ok || !owned(r, id) {
		t.Fatal("Desktop-only registration failed", e)
	}
	c.Enabled = false
	if e = i.Sync(context.Background(), c); e != nil {
		t.Fatal(e)
	}
	_, ok, e = i.Read("claude-desktop", id)
	if e != nil || ok {
		t.Fatal("Desktop disconnect failed", e)
	}
}
