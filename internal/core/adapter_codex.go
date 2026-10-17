package core

import (
	"context"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type CodexAdapter struct {
	host       *Integrator
	configPath string
	shared     func(string) (Registration, bool, error)
}

func NewCodexAdapter(host *Integrator, path string) *CodexAdapter {
	if path == "" {
		path = openAIConfigPath(host.Home)
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
	command, argv, err := openAICommand(bin, args)
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
			return clientCommandFailure(ctx, client, e)
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
func (i *CodexAdapter) Detect() string { return findOpenAICLI() }
func (i *CodexAdapter) ReadShared(id string) (Registration, bool, error) {
	if i.shared == nil {
		return Registration{}, false, nil
	}
	return i.shared(id)
}
func (i *CodexAdapter) Settings(id string) (ClientSettings, error) {
	return ClientSettings{Title: "Codex settings", URL: &url.URL{Scheme: "codex", Host: "settings"}, Instructions: "Open Settings in the OpenAI desktop app. If you use only Codex CLI, its settings are in: " + i.configPath, Direct: true}, nil
}

type codexRegistration struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Enabled *bool    `toml:"enabled,omitempty"`
}
