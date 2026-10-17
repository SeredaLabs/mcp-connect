package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/pelletier/go-toml/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIntegrationOwnership(t *testing.T) {
	d := t.TempDir()
	id := NewID()
	cp := filepath.Join(d, "claude.json")
	xp := filepath.Join(d, "config.toml")
	i := fixtureIntegrator(integrationFixture{Home: d, Executable: filepath.Join(d, "App With Spaces", "connect"), ClaudePath: cp, CodexPath: xp})
	os.WriteFile(cp, []byte(`{"otherSetting":true,"mcpServers":{"external":{"command":"keep-me"}}}`), 0600)
	os.WriteFile(xp, []byte("model = \"preserve\"\n[mcp_servers.external]\ncommand = \"keep-me\"\n"), 0600)
	i.Run = func(_ context.Context, bin string, args ...string) error {
		client := "codex"
		if strings.Contains(filepath.Base(bin), "claude") {
			client = "claude"
		}
		var doc map[string]any
		b, _ := os.ReadFile(fixturePath(i, client))
		key := "mcp_servers"
		if client == "claude" {
			key = "mcpServers"
			json.Unmarshal(b, &doc)
		} else {
			toml.Unmarshal(b, &doc)
		}
		servers := doc[key].(map[string]any)
		if args[1] == "remove" {
			delete(servers, id)
		} else {
			servers[id] = map[string]any{"command": i.Executable, "args": []string{"serve", id}}
		}
		if client == "claude" {
			b, _ = json.Marshal(doc)
		} else {
			b, _ = toml.Marshal(doc)
		}
		return os.WriteFile(fixturePath(i, client), b, 0600)
	}
	i.Find = func(name string) string { return filepath.Join(d, name) }
	for _, client := range []string{"claude", "codex"} {
		for n := 0; n < 2; n++ {
			if e := i.Set(context.Background(), client, id, true); e != nil {
				t.Fatal(e)
			}
		}
		b, _ := os.ReadFile(fixturePath(i, client))
		if !strings.Contains(string(b), "keep-me") {
			t.Fatal("external entry lost")
		}
		for _, secret := range []string{"sentinel-secret", base64.StdEncoding.EncodeToString([]byte("user:sentinel-secret"))} {
			if strings.Contains(string(b), secret) {
				t.Fatal("secret leaked")
			}
		}
		if e := i.Set(context.Background(), client, id, false); e != nil {
			t.Fatal(e)
		}
	}
	collision := map[string]any{"mcpServers": map[string]any{id: map[string]any{"command": "external"}}}
	b, _ := json.Marshal(collision)
	os.WriteFile(cp, b, 0600)
	if i.Set(context.Background(), "claude", id, true) == nil {
		t.Fatal("collision should be rejected")
	}
	os.WriteFile(xp, []byte("broken = ["), 0600)
	if i.Set(context.Background(), "codex", id, true) == nil {
		t.Fatal("invalid TOML accepted")
	}
}
func TestInstalledCLIsIsolated(t *testing.T) {
	if os.Getenv("ONECMCP_TEST_CLIENT_CLIS") != "1" {
		t.Skip("Explicit CLI test opt-in required")
	}
	d := t.TempDir()
	claudeBin, codexBin := FindClient("claude"), FindClient("codex")
	if claudeBin == "" || codexBin == "" {
		t.Fatal("Both client CLIs required")
	}
	cDir, xDir := filepath.Join(d, "claude"), filepath.Join(d, "codex")
	os.MkdirAll(cDir, 0700)
	os.MkdirAll(xDir, 0700)
	cp, xp := filepath.Join(cDir, ".claude.json"), filepath.Join(xDir, "config.toml")
	os.WriteFile(cp, []byte(`{"customSetting":true,"mcpServers":{"external":{"command":"keep-me","args":[]}}}`), 0600)
	os.WriteFile(xp, []byte("# retain comment\nmodel = \"preserve\"\n[mcp_servers.external]\ncommand = \"keep-me\"\n"), 0600)
	i := fixtureIntegrator(integrationFixture{Home: d, Executable: filepath.Join(d, "App With Spaces", "connect"), ClaudePath: cp, CodexPath: xp})
	i.Run = func(ctx context.Context, bin string, args ...string) error {
		command, argv, e := clientCommand(bin, args, "")
		if e != nil {
			return e
		}
		cmd := exec.CommandContext(ctx, command, argv...)
		cmd.Dir = d
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "HOME=") && !strings.HasPrefix(v, "CODEX_HOME=") && !strings.HasPrefix(v, "CLAUDE_CONFIG_DIR=") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "HOME="+d, "CODEX_HOME="+xDir, "CLAUDE_CONFIG_DIR="+cDir)
		return cmd.Run()
	}
	id := NewID()
	for _, client := range []string{"claude", "codex"} {
		if e := i.Set(context.Background(), client, id, true); e != nil {
			t.Fatal(client, e)
		}
		if e := i.Set(context.Background(), client, id, true); e != nil {
			t.Fatal(client, e)
		}
		b, _ := os.ReadFile(fixturePath(i, client))
		if !strings.Contains(string(b), "keep-me") {
			t.Fatal("external entry lost")
		}
		if e := i.Set(context.Background(), client, id, false); e != nil {
			t.Fatal(client, e)
		}
		b, _ = os.ReadFile(fixturePath(i, client))
		if !strings.Contains(string(b), "keep-me") {
			t.Fatal("external entry lost after removal")
		}
	}
}

func TestFinderPATHWithInstalledClient(t *testing.T) {
	if os.Getenv("ONECMCP_TEST_CLIENT_CLIS") != "1" {
		t.Skip("Explicit CLI test opt-in required")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Finder environment regression test is macOS-specific")
	}
	bin := FindClient("codex")
	if bin == "" {
		t.Fatal("Installed Codex CLI required")
	}
	home := t.TempDir()
	if e := os.MkdirAll(filepath.Join(home, ".codex"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("PATH", "/usr/bin:/bin")
	i, e := NewIntegrator()
	if e != nil {
		t.Fatal(e)
	}
	i.Find = func(string) string { return bin }
	id := NewID()
	if e = i.Set(context.Background(), "codex", id, true); e != nil {
		t.Fatal("GUI-style PATH registration failed", e)
	}
	if e = i.Set(context.Background(), "codex", id, false); e != nil {
		t.Fatal(e)
	}
}

func TestStatusPreservesPreviousWorkingApp(t *testing.T) {
	d := t.TempDir()
	old := filepath.Join(d, "old-connect")
	id := NewID()
	if err := os.WriteFile(old, []byte("existing adapter"), 0700); err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(d, "claude.json")
	b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{id: Registration{Command: old, Args: []string{"serve", id}}}})
	if err := os.WriteFile(cp, b, 0600); err != nil {
		t.Fatal(err)
	}
	i := fixtureIntegrator(integrationFixture{ClaudePath: cp, Executable: filepath.Join(d, "new-connect")})
	if got := i.Status("claude", id); got != "Configured · saved adapter" {
		t.Fatal(got)
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if got := i.Status("claude", id); !strings.Contains(got, "repair") {
		t.Fatal(got)
	}
}
