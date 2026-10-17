package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAIPluginStoreFixtures(t *testing.T) {
	for _, tc := range []struct {
		name, config      string
		active, wantError bool
	}{
		{"empty", "model = \"keep\"\n", false, false},
		{"enabled", "[marketplaces.owned]\nsource_type = \"local\"\nsource = %q\n[plugins.\"fixture@owned\"]\nenabled = true\n", true, false},
		{"disabled", "[plugins.\"fixture@owned\"]\nenabled = false\n", false, false},
		{"external-marketplace", "[marketplaces.owned]\nsource_type = \"remote\"\nsource = \"external\"\n", false, true},
		{"changed-enabled-format", "[plugins.\"fixture@owned\"]\nenabled = \"changed-upstream\"\n", false, true},
		{"malformed", "[broken", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "config.toml")
			root := filepath.Join(home, "owned-marketplace")
			content := tc.config
			if tc.name == "enabled" {
				content = fmt.Sprintf(content, root)
			}
			if err := AtomicWrite(path, []byte(content)); err != nil {
				t.Fatal(err)
			}
			store := newOpenAIStore(&Integrator{Home: home}, path)
			active, err := store.pluginState("owned", "fixture@owned", root)
			if (err != nil) != tc.wantError || active != tc.active {
				t.Fatal(active, err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != content {
				t.Fatal("inspection modified store")
			}
		})
	}
}
func TestPluginSchemaFailureDoesNotBreakDirectCodex(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config.toml")
	content := codexConfigFixture + "\n[plugins.\"fixture@owned\"]\nenabled = \"changed-upstream\"\n"
	if err := AtomicWrite(path, []byte(content)); err != nil {
		t.Fatal(err)
	}
	host := &Integrator{Home: home}
	store := newOpenAIStore(host, path)
	if _, err := store.pluginState("owned", "fixture@owned", home); err == nil {
		t.Fatal("changed plugin format accepted")
	}
	codex := NewCodexAdapter(host, path)
	r, ok, err := codex.Read("mcp-0123456789abcdef")
	if err != nil || !ok || r.Command != "/saved/connect" {
		t.Fatal("plugin error leaked into direct registration", r, ok, err)
	}
}
func TestOpenAIConfigLocation(t *testing.T) {
	home := t.TempDir()
	override := filepath.Join(home, "custom-store")
	t.Setenv("CODEX_HOME", override)
	host := &Integrator{Home: home}
	store := newOpenAIStore(host, "")
	codex := NewCodexAdapter(host, "")
	expected := filepath.Join(override, "config.toml")
	if store.configPath != expected || codex.configPath != expected {
		t.Fatal("shared physical store mismatch", store.configPath, codex.configPath)
	}
}
