package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type ClaudeAdapter struct {
	host       *Integrator
	configPath string
}

func NewClaudeAdapter(host *Integrator, path string) *ClaudeAdapter {
	if path == "" {
		path = filepath.Join(host.Home, ".claude.json")
		if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
			path = filepath.Join(dir, ".claude.json")
		}
	}
	return &ClaudeAdapter{host: host, configPath: path}
}
func (i *ClaudeAdapter) ID() string { return "claude" }
func (i *ClaudeAdapter) Read(id string) (Registration, bool, error) {
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
		Servers map[string]claudeRegistration `json:"mcpServers"`
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(b, &object) != nil || object == nil || json.Unmarshal(b, &doc) != nil {
		return Registration{}, false, errors.New("Claude configuration is invalid; nothing changed")
	}
	if raw, ok := object["mcpServers"]; ok && string(raw) == "null" {
		return Registration{}, false, errors.New("Claude mcpServers must be an object; nothing changed")
	}
	r, ok := doc.Servers[id]
	return Registration(r), ok, nil
}
func (i *ClaudeAdapter) run(ctx context.Context, bin string, args ...string) error {
	command, argv, err := clientCommand(bin, args, filepath.FromSlash("@anthropic-ai/claude-code/cli.js"))
	if err != nil {
		return err
	}
	if i.host.Run == nil {
		return errors.New("Client executor unavailable")
	}
	return i.host.Run(ctx, command, argv...)
}
func (adapter *ClaudeAdapter) Set(ctx context.Context, request RegistrationRequest) error {
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
	remove := func() error { return run("mcp", "remove", "--scope", "user", id) }
	add := func(command string) error {
		return run("mcp", "add", "--scope", "user", "--transport", "stdio", id, "--", command, "serve", id)
	}
	if !on {
		if e = remove(); e != nil {
			return e
		}
	} else {
		if exists {
			if e = remove(); e != nil {
				return e
			}
		}
		if e = add(request.Executable); e != nil {
			if exists {
				_ = add(old.Command)
			}
			return e
		}
	}
	return verifyRegistration(i, request)
}
func (i *ClaudeAdapter) Detect() string { return findCLI("claude", nil) }
func (i *ClaudeAdapter) Settings(id string) (ClientSettings, error) {
	return ClientSettings{Title: "Claude Code · MCP settings", ConfigPath: i.configPath, Instructions: "In your existing Claude Code terminal session, enter /mcp to open the MCP menu and check your servers.\n\nIf Claude Code is not running, start it with claude first.", CopyText: "/mcp", ActionLabel: "Copy /mcp"}, nil
}

type claudeRegistration struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled *bool    `json:"enabled,omitempty"`
}
