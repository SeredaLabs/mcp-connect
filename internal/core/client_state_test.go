package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDisablePersistsBeforeFailedCleanup(t *testing.T) {
	id := NewID()
	s := &Store{Dir: t.TempDir()}
	c := Connection{ID: id, Name: "Fixture", URL: "https://example.com/mcp", Codex: true, Enabled: true}
	if err := s.Write([]Connection{c}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	exe := filepath.Join(t.TempDir(), "adapter")
	if err := AtomicWrite(path, []byte("[mcp_servers."+id+"]\ncommand = "+strconv.Quote(exe)+"\nargs = [\"serve\",\""+id+"\"]\n")); err != nil {
		t.Fatal(err)
	}
	i := fixtureIntegrator(integrationFixture{CodexPath: path, Executable: exe, Find: func(string) string { return "fixture" }, Run: func(context.Context, string, ...string) error {
		stored, err := s.Find(id)
		if err != nil || stored.Enabled {
			t.Fatal("cleanup started before disabling profile", err)
		}
		return errors.New("fixture failure")
	}})
	if err := s.Disable(context.Background(), i, c); err == nil {
		t.Fatal("cleanup failure not reported")
	}
	stored, err := s.Find(id)
	if err != nil || stored.Enabled {
		t.Fatal("failed cleanup reenabled profile", err)
	}
}

func TestApplyIndependentClientsAndTargetedRetry(t *testing.T) {
	id := NewID()
	path := filepath.Join(t.TempDir(), "config.toml")
	exe := filepath.Join(t.TempDir(), "adapter")
	c := Connection{ID: id, Name: "Fixture", URL: "https://example.com/mcp", Codex: true, ChatGPT: true, Enabled: true}
	runs, pluginCalls := 0, 0
	i := fixtureIntegrator(integrationFixture{Home: t.TempDir(), CodexPath: path, Executable: exe, Find: func(string) string { return "fixture" }})
	i.Run = func(context.Context, string, ...string) error {
		runs++
		return AtomicWrite(path, []byte("[mcp_servers."+id+"]\ncommand = "+strconv.Quote(exe)+"\nargs = [\"serve\",\""+id+"\"]\n"))
	}
	i.Output = func(context.Context, string, ...string) ([]byte, error) {
		pluginCalls++
		return nil, errors.New("plugin failure")
	}
	report := i.Apply(context.Background(), c)
	if len(report) != 4 || len(report.FailedClients()) != 1 || report.FailedClients()[0] != "chatgpt" || runs != 1 {
		t.Fatal("independent results lost", report, runs)
	}
	if reg, exists, err := i.Read("codex", id); err != nil || !exists || reg.Command != exe {
		t.Fatal("working Codex missing", err)
	}
	i.Apply(context.Background(), c, report.FailedClients()...)
	if runs != 1 || pluginCalls != 2 {
		t.Fatal("retry touched successful client", runs, pluginCalls)
	}
}

func TestSaveSettingsWithoutVaultAndRejectEndpointChanges(t *testing.T) {
	s := &Store{Dir: t.TempDir()} // nil vault ensures local settings never read secrets
	c := Connection{ID: NewID(), Name: "Fixture", URL: "https://example.com/mcp", Enabled: true}
	if err := s.Write([]Connection{c}); err != nil {
		t.Fatal(err)
	}
	c.Codex = true
	c.Name = "Renamed"
	if err := s.SaveSettings(c); err != nil {
		t.Fatal(err)
	}
	c.URL = "https://different.example/mcp"
	if err := s.SaveSettings(c); err == nil {
		t.Fatal("endpoint changed without credentials")
	}
	stored, _ := s.Find(c.ID)
	if stored.Name != "Renamed" || !stored.Codex || stored.URL != "https://example.com/mcp" {
		t.Fatal(stored)
	}
}

func TestInspectDisabledCodexRegistration(t *testing.T) {
	id := NewID()
	exe := filepath.Join(t.TempDir(), "adapter")
	path := filepath.Join(t.TempDir(), "config.toml")
	b := []byte("[mcp_servers." + id + "]\ncommand = " + strconv.Quote(exe) + "\nargs = [\"serve\",\"" + id + "\"]\nenabled = false\n")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	i := fixtureIntegrator(integrationFixture{CodexPath: path, Executable: exe})
	state := i.Inspect("codex", id)
	if !state.Registered || !state.Disabled || state.Err != nil {
		t.Fatal(state)
	}
}
