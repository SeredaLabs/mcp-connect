package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Registration struct {
	Command string   `json:"command" toml:"command"`
	Args    []string `json:"args" toml:"args"`
	Enabled *bool    `json:"enabled,omitempty" toml:"enabled,omitempty"`
}
type Integrator struct {
	DisplayName       string
	Home              string
	Executable        string
	ClaudePath        string
	CodexPath         string
	ClaudeDesktopPath string
	Run               func(context.Context, string, ...string) error
	Output            func(context.Context, string, ...string) ([]byte, error)
	Find              func(string) string
}

func FindClient(name string) string {
	if name == "chatgpt" {
		home, _ := os.UserHomeDir()
		if runtime.GOOS == "darwin" {
			for _, p := range []string{"/Applications/ChatGPT.app", filepath.Join(home, "Applications/ChatGPT.app")} {
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
		if p, err := exec.LookPath("chatgpt"); err == nil {
			return p
		}
		if runtime.GOOS == "windows" {
			for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
				for _, tail := range []string{"ChatGPT/ChatGPT.exe", "Programs/ChatGPT/ChatGPT.exe", "Microsoft/WindowsApps/ChatGPT.exe"} {
					p := filepath.Join(base, filepath.FromSlash(tail))
					if _, err := os.Stat(p); base != "" && err == nil {
						return p
					}
				}
			}
		}
		return ""
	}
	if name == "claude-desktop" {
		return findClaudeDesktop()
	}
	if p, e := exec.LookPath(name); e == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	names := []string{filepath.Join(home, ".local", "bin", name), filepath.Join(home, ".cargo", "bin", name), filepath.Join("/opt/homebrew/bin", name), filepath.Join("/usr/local/bin", name)}
	if runtime.GOOS == "darwin" && name == "codex" {
		names = append(names, "/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex", "/Applications/Codex.app/Contents/Resources/codex", filepath.Join(home, "Applications/Codex.app/Contents/Resources/codex"), filepath.Join(home, "Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex"))
	}
	if runtime.GOOS == "windows" {
		names = []string{filepath.Join(home, ".local", "bin", name+".exe"), filepath.Join(home, ".cargo", "bin", name+".exe")}
		if name == "codex" {
			for _, root := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
				if root == "" {
					continue
				}
				for _, app := range []string{"Programs/ChatGPT/resources", "ChatGPT/resources", "Programs/Codex/resources", "Codex/resources"} {
					for _, exe := range []string{"codex.exe", "codex-cli/bin/codex.exe"} {
						names = append(names, filepath.Join(root, filepath.FromSlash(app), filepath.FromSlash(exe)))
					}
				}
			}
		}
		if root := os.Getenv("APPDATA"); root != "" {
			names = append(names, filepath.Join(root, "npm", name+".cmd"))
		}
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
			if root != "" {
				names = append(names, filepath.Join(root, "nodejs", name+".cmd"))
			}
		}
	}
	for _, p := range names {
		if f, e := os.Stat(p); e == nil && !f.IsDir() {
			return p
		}
	}
	return ""
}
func NewIntegrator() (*Integrator, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return nil, e
	}
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	exe, _ = filepath.Abs(exe)
	claudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
	claude := filepath.Join(home, ".claude.json")
	if claudeDir != "" {
		claude = filepath.Join(claudeDir, ".claude.json")
	}
	codexDir := os.Getenv("CODEX_HOME")
	if codexDir == "" {
		codexDir = filepath.Join(home, ".codex")
	}
	configDir, e := os.UserConfigDir()
	if e != nil {
		return nil, e
	}
	return &Integrator{Home: home, Executable: exe, ClaudePath: claude, CodexPath: filepath.Join(codexDir, "config.toml"), ClaudeDesktopPath: filepath.Join(configDir, "Claude", "claude_desktop_config.json"), Run: func(ctx context.Context, bin string, args ...string) error {
		command, arguments, e := clientCommand(bin, args)
		if e != nil {
			return e
		}
		cmd := exec.CommandContext(ctx, command, arguments...)
		cmd.Dir = home
		cmd.Env = clientEnvironment(bin)
		return cmd.Run()
	}, Output: func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		command, argv, err := clientCommand(bin, args)
		if err != nil {
			return nil, err
		}
		cmd := exec.CommandContext(ctx, command, argv...)
		cmd.Dir = home
		cmd.Env = clientEnvironment(bin)
		return cmd.Output()
	}}, nil
}
func (i *Integrator) path(client string) string {
	if client == "claude-desktop" {
		return i.ClaudeDesktopPath
	}
	if client == "claude" {
		return i.ClaudePath
	}
	return i.CodexPath
}
func (i *Integrator) Read(client, id string) (Registration, bool, error) {
	if client == "chatgpt" {
		return i.readChatGPT(id)
	}
	path := i.path(client)
	if path == "" {
		return Registration{}, false, nil
	}
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return Registration{}, false, nil
	}
	if e != nil {
		return Registration{}, false, errors.New("Cannot read client configuration")
	}
	var regs map[string]Registration
	if client == "claude" || client == "claude-desktop" {
		var doc struct {
			Servers map[string]Registration `json:"mcpServers"`
		}
		if json.Unmarshal(b, &doc) != nil {
			return Registration{}, false, errors.New("Claude configuration is invalid; nothing changed")
		}
		regs = doc.Servers
	} else {
		var doc struct {
			Servers map[string]Registration `toml:"mcp_servers"`
		}
		if toml.Unmarshal(b, &doc) != nil {
			return Registration{}, false, errors.New("Codex configuration is invalid; nothing changed")
		}
		regs = doc.Servers
	}
	r, ok := regs[id]
	return r, ok, nil
}
func owned(r Registration, id string) bool {
	return len(r.Args) == 2 && r.Args[0] == "serve" && r.Args[1] == id && filepath.IsAbs(r.Command)
}
func (i *Integrator) Status(client, id string) string {
	r, ok, e := i.Read(client, id)
	if client == "codex" && e == nil && !ok {
		r, ok, e = i.readChatGPT(id)
	}
	if e != nil {
		return e.Error()
	}
	if !ok {
		return "Not configured"
	}
	if !owned(r, id) {
		return "Name conflict — external entry"
	}
	if !samePath(r.Command, i.Executable) {
		if info, err := os.Stat(r.Command); err == nil && !info.IsDir() {
			return "Configured · saved adapter"
		}
		return "App moved — Save & Apply to repair"
	}
	if FindClient(client) == "" {
		if client == "claude-desktop" {
			return "Configured · desktop app unavailable"
		}
		return "Configured · client CLI unavailable"
	}
	return "Configured"
}
func (i *Integrator) Set(ctx context.Context, client, id string, on bool) error {
	if client == "chatgpt" {
		return i.setChatGPT(ctx, id, on)
	}
	if !idPattern.MatchString(id) {
		return errors.New("Invalid integration ID")
	}
	old, exists, e := i.Read(client, id)
	if e != nil {
		return e
	}
	if exists && !owned(old, id) {
		return errors.New("Refusing to modify an external MCP entry")
	}
	if !on && !exists {
		return nil
	}
	if client == "claude-desktop" && !on {
		return i.setDesktop(id, false)
	}
	if on && exists && old.Command == i.Executable && (old.Enabled == nil || *old.Enabled) {
		return nil
	}
	find := i.Find
	if find == nil {
		find = FindClient
	}
	bin := find(client)
	if bin == "" {
		if client == "claude-desktop" {
			return errors.New("Install Claude Desktop first or use its standard user configuration location")
		}
		return errors.New("Install " + client + " CLI first; reopen this app or put the CLI in ~/.local/bin")
	}

	if !filepath.IsAbs(i.Executable) {
		return errors.New("Application path must be absolute")
	}
	if client == "claude-desktop" {
		return i.setDesktop(id, on)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	run := func(args ...string) error {
		if e := i.Run(ctx, bin, args...); e != nil {
			return errors.New(client + " could not update its MCP configuration")
		}
		return nil
	}
	remove := func() error {
		if client == "claude" {
			return run("mcp", "remove", "--scope", "user", id)
		}
		return run("mcp", "remove", id)
	}
	add := func(command string) error {
		if client == "claude" {
			return run("mcp", "add", "--scope", "user", "--transport", "stdio", id, "--", command, "serve", id)
		}
		return run("mcp", "add", id, "--", command, "serve", id)
	}
	if !on {
		if e = remove(); e != nil {
			return e
		}
	} else {
		if exists && client == "claude" {
			if e = remove(); e != nil {
				return e
			}
		}
		if e = add(i.Executable); e != nil {
			if exists {
				_ = add(old.Command)
			}
			return e
		}
	}
	r, exists, e := i.Read(client, id)
	if e != nil {
		return e
	}
	if on && (!exists || !owned(r, id) || !samePath(r.Command, i.Executable) || (r.Enabled != nil && !*r.Enabled)) {
		return errors.New(client + " registration verification failed")
	}
	if !on && exists {
		return errors.New(client + " removal verification failed")
	}
	return nil
}
func (i *Integrator) Sync(ctx context.Context, c Connection) error {
	return i.Apply(ctx, c).Err()
}
func (s *Store) Disable(ctx context.Context, i *Integrator, c Connection) error {
	report, err := s.DisableClients(ctx, i, c)
	if err != nil {
		return err
	}
	if err := report.Err(); err != nil {
		return fmt.Errorf("Profile disabled. Some client entries could not be removed; retry Disconnect. Existing sessions must be closed.\n%w", err)
	}
	return nil
}

// DisableClients persists the disabled profile before attempting any cleanup.
// The report distinguishes cleanup failures from a failed local save.
func (s *Store) DisableClients(ctx context.Context, i *Integrator, c Connection) (ApplyReport, error) {
	cs, e := s.Load()
	if e != nil {
		return nil, e
	}
	found := false
	for n := range cs {
		if cs[n].ID == c.ID {
			c = cs[n]
			c.Enabled = false
			cs[n] = c
			found = true
		}
	}
	if !found {
		return nil, errors.New("Connection not found")
	}
	if e := s.Write(cs); e != nil {
		return nil, e
	}
	return i.Apply(ctx, c), nil
}

// Finder launches GUI applications with a minimal PATH. npm client launchers
// use /usr/bin/env node, so make the discovered client's directory available
// to its own dependencies without invoking a shell or reading shell profiles.
func clientEnvironment(bin string) []string {
	env := os.Environ()
	key := "PATH="
	dirs := []string{filepath.Dir(bin), os.Getenv("PATH")}
	if runtime.GOOS != "windows" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	} else {
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
			if root != "" {
				dirs = append(dirs, filepath.Join(root, "nodejs"))
			}
		}
	}
	path := strings.Join(dirs, string(os.PathListSeparator))
	for n, v := range env {
		if strings.HasPrefix(strings.ToUpper(v), key) {
			env[n] = "PATH=" + path
			return env
		}
	}
	return append(env, "PATH="+path)
}
