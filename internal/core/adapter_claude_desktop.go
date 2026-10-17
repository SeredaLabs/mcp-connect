package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func findClaudeDesktop() string {
	home, _ := os.UserHomeDir()
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{"/Applications/Claude.app", filepath.Join(home, "Applications", "Claude.app")}
	case "windows":
		if root := os.Getenv("LOCALAPPDATA"); root != "" {
			paths = []string{filepath.Join(root, "AnthropicClaude", "Claude.exe"), filepath.Join(root, "Programs", "Claude", "Claude.exe"), filepath.Join(root, "Microsoft", "WindowsApps", "Claude.exe")}
		}
	default:
		if p, e := exec.LookPath("claude-desktop"); e == nil {
			return p
		}
	}
	for _, p := range paths {
		if _, e := os.Stat(p); e == nil {
			return p
		}
	}
	// A pre-existing desktop configuration supports nonstandard app locations.
	if dir, e := os.UserConfigDir(); e == nil {
		p := filepath.Join(dir, "Claude", "claude_desktop_config.json")
		if _, e = os.Stat(p); e == nil {
			return p
		}
	}
	return ""
}

// Claude Desktop has no registration CLI. Update only our owned mcpServers key
// while retaining all other JSON values, and reject malformed/concurrently
// modified files. There are no credentials in the entry written here.
func (i *ClaudeDesktopAdapter) setDesktop(id string, on bool, executable string) error {
	path := i.configPath
	if path == "" {
		return errors.New("Claude Desktop configuration path is unavailable")
	}
	original, e := os.ReadFile(path)
	missing := os.IsNotExist(e)
	if e != nil && !missing {
		return errors.New("Cannot read Claude Desktop configuration")
	}
	doc := map[string]json.RawMessage{}
	servers := map[string]json.RawMessage{}
	if !missing {
		if json.Unmarshal(original, &doc) != nil || doc == nil {
			return errors.New("Claude Desktop configuration is not a JSON object; nothing changed")
		}
		if raw, ok := doc["mcpServers"]; ok {
			if json.Unmarshal(raw, &servers) != nil || servers == nil {
				return errors.New("Claude Desktop mcpServers must be an object; nothing changed")
			}
		}
	}
	if raw, exists := servers[id]; exists {
		var r desktopRegistration
		if json.Unmarshal(raw, &r) != nil || !owned(Registration(r), id) {
			return errors.New("Refusing to modify an external Claude Desktop MCP entry")
		}
	}
	if on {
		if !filepath.IsAbs(executable) {
			return errors.New("Application path must be absolute")
		}
		b, _ := json.Marshal(desktopRegistration{Command: executable, Args: []string{"serve", id}})
		servers[id] = b
	} else {
		delete(servers, id)
	}
	b, _ := json.Marshal(servers)
	doc["mcpServers"] = b
	updated, e := json.MarshalIndent(doc, "", "  ")
	if e != nil {
		return e
	}
	updated = append(updated, '\n')
	current, e := os.ReadFile(path)
	if missing {
		if e == nil || !os.IsNotExist(e) {
			return errors.New("Claude Desktop configuration changed; retry")
		}
	} else if e != nil || !bytes.Equal(current, original) {
		return errors.New("Claude Desktop configuration changed; retry")
	}
	if e = AtomicWrite(path, updated); e != nil {
		return errors.New("Cannot save Claude Desktop configuration")
	}
	r, exists, e := i.Read(id)
	if e != nil {
		return e
	}
	if on && (!exists || !owned(r, id) || !samePath(r.Command, executable)) {
		return errors.New("Claude Desktop registration verification failed")
	}
	if !on && exists {
		return errors.New("Claude Desktop removal verification failed")
	}
	return nil
}

type ClaudeDesktopAdapter struct {
	host       *Integrator
	configPath string
}

func NewClaudeDesktopAdapter(host *Integrator, path string) *ClaudeDesktopAdapter {
	if path == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			path = filepath.Join(dir, "Claude", "claude_desktop_config.json")
		}
	}
	return &ClaudeDesktopAdapter{host, path}
}
func (i *ClaudeDesktopAdapter) ID() string     { return "claude-desktop" }
func (i *ClaudeDesktopAdapter) Detect() string { return findClaudeDesktop() }
func (i *ClaudeDesktopAdapter) Read(id string) (Registration, bool, error) {
	if !idPattern.MatchString(id) {
		return Registration{}, false, errors.New("Invalid integration ID")
	}
	if i.configPath == "" {
		return Registration{}, false, errors.New("Claude Desktop configuration path is unavailable")
	}
	b, err := os.ReadFile(i.configPath)
	if os.IsNotExist(err) {
		return Registration{}, false, nil
	}
	if err != nil {
		return Registration{}, false, errors.New("Cannot read client configuration")
	}
	var doc struct {
		Servers map[string]desktopRegistration `json:"mcpServers"`
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
func (adapter *ClaudeDesktopAdapter) Set(ctx context.Context, r RegistrationRequest) error {
	if !idPattern.MatchString(r.ID) {
		return errors.New("Invalid integration ID")
	}
	old, exists, err := adapter.Read(r.ID)
	if err != nil {
		return err
	}
	if exists && !owned(old, r.ID) {
		return errors.New("Refusing to modify an external MCP entry")
	}
	if !r.Enabled && !exists {
		return nil
	}
	if r.Enabled && exists && old.Command == r.Executable && (old.Enabled == nil || *old.Enabled) {
		return nil
	}
	if r.Enabled && adapter.host.find(adapter) == "" {
		return errors.New("Install Claude Desktop first or use its standard user configuration location")
	}
	return adapter.setDesktop(r.ID, r.Enabled, r.Executable)
}
func (i *ClaudeDesktopAdapter) Settings(id string) (ClientSettings, error) {
	return ClientSettings{Title: "Claude Desktop · MCP settings", ConfigPath: i.configPath, Instructions: "In Claude, open:\nSettings → Desktop app → Developer\n\nThis is where you can check local MCP servers and their logs. Restart Claude after Save & Connect to load updated settings.", URL: &url.URL{Scheme: "claude", Host: "claude.ai", Path: "/new"}, ActionLabel: "Open Claude Desktop", OpenOnShow: true, Unavailable: "Configured · desktop app unavailable"}, nil
}

type desktopRegistration struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled *bool    `json:"enabled,omitempty"`
}
