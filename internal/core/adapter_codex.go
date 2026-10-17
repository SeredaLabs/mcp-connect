package core

import (
	"context"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type CodexAdapter struct {
	host       *Integrator
	configPath string
	shared     func(string) (Registration, bool, error)
}

func NewCodexAdapter(host *Integrator, path string) *CodexAdapter {
	if path == "" {
		dir := os.Getenv("CODEX_HOME")
		if dir == "" {
			dir = filepath.Join(host.Home, ".codex")
		}
		path = filepath.Join(dir, "config.toml")
	}
	return &CodexAdapter{host: host, configPath: path}
}
func (i *CodexAdapter) ID() string { return "codex" }
func (i *CodexAdapter) Read(id string) (Registration, bool, error) {
	if !idPattern.MatchString(id) {
		return Registration{}, false, errors.New("Invalid integration ID")
	}
	b, err := os.ReadFile(i.configPath)
	if os.IsNotExist(err) {
		return Registration{}, false, nil
	}
	if err != nil {
		return Registration{}, false, errors.New("Cannot read client configuration")
	}
	var doc struct {
		Servers map[string]codexRegistration `toml:"mcp_servers"`
	}
	if toml.Unmarshal(b, &doc) != nil {
		return Registration{}, false, errors.New("Codex configuration is invalid; nothing changed")
	}
	r, ok := doc.Servers[id]
	return Registration(r), ok, nil
}
func (i *CodexAdapter) run(ctx context.Context, bin string, args ...string) error {
	command, argv, err := clientCommand(bin, args, filepath.FromSlash("@openai/codex/bin/codex.js"))
	if err != nil {
		return err
	}
	if i.host.Run == nil {
		return errors.New("Client executor unavailable")
	}
	return i.host.Run(ctx, command, argv...)
}
func (adapter *CodexAdapter) Set(ctx context.Context, request RegistrationRequest) error {
	i := adapter
	client, id, on := i.ID(), request.ID, request.Enabled
	old, exists, done, e := registrationPreflight(i, request)
	if e != nil || done {
		return e
	}
	find := i.host.Find
	if find == nil {
		find = func(string) string { return i.Detect() }
	}
	bin := find(client)
	if bin == "" {
		return errors.New("Install " + client + " CLI first; reopen this app or put the CLI in ~/.local/bin")
	}

	if !filepath.IsAbs(request.Executable) {
		return errors.New("Application path must be absolute")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	run := func(args ...string) error {
		if e := i.run(ctx, bin, args...); e != nil {
			return errors.New(client + " could not update its MCP configuration")
		}
		return nil
	}
	remove := func() error { return run("mcp", "remove", id) }
	add := func(command string) error { return run("mcp", "add", id, "--", command, "serve", id) }
	if !on {
		if e = remove(); e != nil {
			return e
		}
	} else {
		if e = add(request.Executable); e != nil {
			if exists {
				_ = add(old.Command)
			}
			return e
		}
	}
	return verifyRegistration(i, request)
}
func (i *CodexAdapter) Detect() string {
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
func (i *CodexAdapter) ReadShared(id string) (Registration, bool, error) {
	if i.shared == nil {
		return Registration{}, false, nil
	}
	return i.shared(id)
}
func (i *CodexAdapter) Settings(id string) (ClientSettings, error) {
	return ClientSettings{Title: "Codex settings", URL: &url.URL{Scheme: "codex", Host: "settings"}, Instructions: "Open Settings in the OpenAI desktop app. If you use only Codex CLI, its settings are in: " + i.configPath, Direct: true}, nil
}
func (i *CodexAdapter) pluginState(market, selector, root string) (bool, error) {
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

func (i *CodexAdapter) pluginHome() string { return filepath.Dir(i.configPath) }
func (i *CodexAdapter) pluginOutput(ctx context.Context, bin string, args ...string) ([]byte, error) {
	command, argv, err := clientCommand(bin, args, filepath.FromSlash("@openai/codex/bin/codex.js"))
	if err != nil {
		return nil, err
	}
	if i.host.Output == nil {
		return nil, errors.New("Plugin installer unavailable")
	}
	return i.host.Output(ctx, command, argv...)
}

type codexRegistration struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Enabled *bool    `toml:"enabled,omitempty"`
}
