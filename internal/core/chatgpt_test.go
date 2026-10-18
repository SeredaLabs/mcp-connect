package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestChatGPTRejectsUnownedAndInvalidConfiguration(t *testing.T) {
	for _, scenario := range []string{"directory", "marketplace", "invalid"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			id := NewID()
			i := fixtureIntegrator(integrationFixture{Home: home, CodexPath: filepath.Join(home, ".codex", "config.toml"), Executable: filepath.Join(home, "connect")})
			original := []byte("model = \"preserve\"\n")
			if scenario == "directory" {
				os.MkdirAll(fixtureChatGPT(i).pluginRoot(id), 0700)
			}
			if scenario == "marketplace" {
				original = []byte("[marketplaces." + pluginMarket(id) + "]\nsource_type = \"local\"\nsource = \"/external\"\n")
			}
			if scenario == "invalid" {
				original = []byte("broken = [")
			}
			if err := AtomicWrite(fixturePath(i, "codex"), original); err != nil {
				t.Fatal(err)
			}
			if err := i.Set(context.Background(), "chatgpt", id, true); err == nil {
				t.Fatal("unsafe setup accepted")
			}
			after, _ := os.ReadFile(fixturePath(i, "codex"))
			if !bytes.Equal(after, original) {
				t.Fatal("external configuration changed")
			}
		})
	}
}

func TestChatGPTInstalledCLIIsolated(t *testing.T) {
	if os.Getenv("ONECMCP_TEST_CLIENT_CLIS") != "1" {
		t.Skip("Explicit CLI test opt-in required")
	}
	bin := FindClient("codex")
	if bin == "" {
		t.Fatal("Codex plugin CLI required")
	}
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	path := filepath.Join(codexHome, "config.toml")
	if err := AtomicWrite(path, []byte("model = \"preserve\"\n[mcp_servers.external]\ncommand = \"keep-me\"\n")); err != nil {
		t.Fatal(err)
	}
	i := fixtureIntegrator(integrationFixture{Home: home, CodexPath: path, ClaudePath: filepath.Join(home, "claude.json"), ClaudeDesktopPath: filepath.Join(home, "desktop.json"), Executable: filepath.Join(home, "App With Spaces", "connect"), Find: func(name string) string {
		if name == "codex" {
			return bin
		}
		return ""
	}})
	invoke := func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		command, argv, err := clientCommand(bin, args, "")
		if err != nil {
			return nil, err
		}
		cmd := exec.CommandContext(ctx, command, argv...)
		cmd.Dir = home
		for _, v := range clientEnvironment(bin) {
			if !strings.HasPrefix(v, "HOME=") && !strings.HasPrefix(v, "CODEX_HOME=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "HOME="+home, "CODEX_HOME="+codexHome)
		b, err := cmd.Output()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				t.Log(string(exit.Stderr))
			}
		}
		return b, err
	}
	i.Output = invoke
	i.Run = func(ctx context.Context, bin string, args ...string) error {
		_, err := invoke(ctx, bin, args...)
		return err
	}
	c := Connection{ID: NewID(), Name: "Fixture workspace", URL: "https://private-fixture.example/mcp", Enabled: true, ChatGPT: true, Codex: true}
	ctx := context.Background()
	for n := 0; n < 2; n++ {
		if err := i.Sync(ctx, c); err != nil {
			receipt, _, ownerErr := fixtureChatGPT(i).pluginOwner(c.ID)
			reg, present, readErr := fixtureChatGPT(i).Read(c.ID)
			t.Fatalf("%v; receipt=%+v ownerErr=%v registration=%+v present=%v readErr=%v expected=%q", err, receipt, ownerErr, reg, present, readErr, i.Executable)
		}
	}
	r, ok, err := i.Read("chatgpt", c.ID)
	if err != nil || !ok || r.Command != i.Executable || !owned(r, c.ID) {
		t.Fatal("installed cache not verified", err)
	}
	if _, ok, err := i.Read("codex", c.ID); err != nil || !ok {
		t.Fatal("direct Codex registration missing")
	}
	settings, err := i.Settings(chatGPTClientID, c.ID)
	if err != nil || settings.URL == nil || settings.URL.Query().Get("marketplacePath") != filepath.Join(fixtureChatGPT(i).pluginRoot(c.ID), ".agents", "plugins", "marketplace.json") {
		t.Fatal("setup link", err)
	}
	err = filepath.Walk(fixtureChatGPT(i).pluginRoot(c.ID), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if bytes.Contains(b, []byte(c.URL)) || bytes.Contains(b, []byte("sentinel-secret")) {
			t.Fatal("secret or endpoint exported")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := os.ReadFile(filepath.Join(fixtureChatGPT(i).pluginRoot(c.ID), "plugins", c.ID, ".codex-plugin", "plugin.json"))
	if !bytes.Contains(manifest, []byte(c.Name)) {
		t.Fatal("missing friendly profile name")
	}
	i.Executable = filepath.Join(home, "Moved App", "connect")
	if err := i.Sync(ctx, c); err != nil {
		t.Fatal("moved executable repair", err)
	}
	r, ok, err = i.Read("chatgpt", c.ID)
	if err != nil || !ok || r.Command != i.Executable {
		t.Fatal("stale installed cache", err)
	}
	// Switching to Codex-only removes the shared plugin and restores a direct entry.
	c.ChatGPT = false
	if err := i.Sync(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := i.Read("chatgpt", c.ID); err != nil || ok {
		t.Fatal("plugin not removed", err)
	}
	if _, ok, err := i.Read("codex", c.ID); err != nil || !ok {
		t.Fatal("direct Codex missing", err)
	}
	c.ChatGPT = true
	if err := i.Sync(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := i.Read("codex", c.ID); err != nil || !ok {
		t.Fatal("direct Codex removed by plugin setup", err)
	}
	c.Enabled = false
	if err := i.Sync(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := i.Read("chatgpt", c.ID); err != nil || ok {
		t.Fatal("disconnect retained active plugin", err)
	}
	b, _ := os.ReadFile(path)
	if !bytes.Contains(b, []byte("keep-me")) || !bytes.Contains(b, []byte("preserve")) {
		t.Fatal("unrelated settings lost")
	}
	// A source containing additional servers is not silently overwritten.
	extra := map[string]any{"mcpServers": map[string]Registration{c.ID: {Command: i.Executable, Args: []string{"serve", c.ID}}, "external": {Command: "external"}}}
	b, _ = json.Marshal(extra)
	source := filepath.Join(fixtureChatGPT(i).pluginRoot(c.ID), "plugins", c.ID, ".mcp.json")
	AtomicWrite(source, b)
	if err := i.Set(ctx, "chatgpt", c.ID, true); err == nil {
		t.Fatal("modified source accepted")
	}
	after, _ := os.ReadFile(source)
	if !bytes.Equal(b, after) {
		t.Fatal("modified source overwritten")
	}
}

func TestChatGPTFailureKeepsWorkingCodexEntry(t *testing.T) {
	home := t.TempDir()
	id := NewID()
	path := filepath.Join(home, ".codex", "config.toml")
	exe := filepath.Join(home, "connect")
	original := []byte("[mcp_servers." + id + "]\ncommand = " + strconv.Quote(exe) + "\nargs = [\"serve\", \"" + id + "\"]\n")
	if err := AtomicWrite(path, original); err != nil {
		t.Fatal(err)
	}
	i := fixtureIntegrator(integrationFixture{Home: home, CodexPath: path, Executable: exe, Find: func(string) string { return "fixture-cli" }, Output: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("fixture installer failure")
	}, Run: func(context.Context, string, ...string) error {
		t.Fatal("working direct entry modified after plugin failure")
		return nil
	}})
	c := Connection{ID: id, Name: "Fixture", URL: "https://example.com/mcp", Enabled: true, ChatGPT: true, Codex: true}
	if err := i.Sync(context.Background(), c); err == nil {
		t.Fatal("failure not reported")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("working configuration modified")
	}
}
