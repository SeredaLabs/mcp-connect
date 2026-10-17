package core

import (
	"context"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"runtime"
)

// openAIStore owns the physical configuration and CLI launcher currently shared
// by OpenAI clients. Plugin fields never belong to the direct Codex MCP adapter.
// It has no registration workflow, plugin manifests or profile selection logic.
type openAIStore struct {
	host       *Integrator
	configPath string
}

func newOpenAIStore(host *Integrator, path string) *openAIStore {
	if path == "" {
		path = openAIConfigPath(host.Home)
	}
	return &openAIStore{host: host, configPath: path}
}
func openAIConfigPath(home string) string {
	dir := os.Getenv("CODEX_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".codex")
	}
	return filepath.Join(dir, "config.toml")
}
func openAICommand(bin string, args []string) (string, []string, error) {
	return clientCommand(bin, args, filepath.FromSlash("@openai/codex/bin/codex.js"))
}
func (i *openAIStore) Detect() string { return findOpenAICLI() }
func findOpenAICLI() string {
	home, _ := os.UserHomeDir()
	var paths []string
	if runtime.GOOS == "darwin" {
		paths = []string{"/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex", "/Applications/Codex.app/Contents/Resources/codex", filepath.Join(home, "Applications/Codex.app/Contents/Resources/codex"), filepath.Join(home, "Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex")}
	}
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
			if root == "" {
				continue
			}
			for _, app := range []string{"Programs/ChatGPT/resources", "ChatGPT/resources", "Programs/Codex/resources", "Codex/resources"} {
				for _, exe := range []string{"codex.exe", "codex-cli/bin/codex.exe"} {
					paths = append(paths, filepath.Join(root, filepath.FromSlash(app), filepath.FromSlash(exe)))
				}
			}
		}
	}
	return findCLI("codex", paths)
}
func (i *openAIStore) pluginState(market, selector, root string) (bool, error) {
	b, e := os.ReadFile(i.configPath)
	if os.IsNotExist(e) || i.configPath == "" {
		return false, nil
	}
	if e != nil {
		return false, errors.New("Cannot read OpenAI configuration")
	}
	var doc struct {
		Marketplaces map[string]struct {
			Source     string `toml:"source"`
			SourceType string `toml:"source_type"`
		} `toml:"marketplaces"`
		Plugins map[string]struct {
			Enabled bool `toml:"enabled"`
		} `toml:"plugins"`
	}
	if toml.Unmarshal(b, &doc) != nil {
		return false, errors.New("OpenAI configuration is invalid; nothing changed")
	}
	if m, ok := doc.Marketplaces[market]; ok && (m.SourceType != "local" || !samePath(m.Source, root)) {
		return false, errors.New("Name conflict — external OpenAI marketplace")
	}
	return doc.Plugins[selector].Enabled, nil
}

func (i *openAIStore) pluginHome() string { return filepath.Dir(i.configPath) }
func (i *openAIStore) pluginOutput(ctx context.Context, bin string, args ...string) ([]byte, error) {
	command, argv, err := openAICommand(bin, args)
	if err != nil {
		return nil, err
	}
	if i.host.Output == nil {
		return nil, errors.New("Plugin installer unavailable")
	}
	return i.host.Output(ctx, command, argv...)
}
