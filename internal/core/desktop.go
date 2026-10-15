package core

import (
	"bytes"
	"encoding/json"
	"errors"
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
func (i *Integrator) setDesktop(id string, on bool) error {
	path := i.ClaudeDesktopPath
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
		var r Registration
		if json.Unmarshal(raw, &r) != nil || !owned(r, id) {
			return errors.New("Refusing to modify an external Claude Desktop MCP entry")
		}
	}
	if on {
		if !filepath.IsAbs(i.Executable) {
			return errors.New("Application path must be absolute")
		}
		b, _ := json.Marshal(Registration{Command: i.Executable, Args: []string{"serve", id}})
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
	r, exists, e := i.Read("claude-desktop", id)
	if e != nil {
		return e
	}
	if on && (!exists || !owned(r, id) || !samePath(r.Command, i.Executable)) {
		return errors.New("Claude Desktop registration verification failed")
	}
	if !on && exists {
		return errors.New("Claude Desktop removal verification failed")
	}
	return nil
}
