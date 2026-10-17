package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type pluginReceipt struct {
	Owner         string `json:"owner"`
	ID            string `json:"id"`
	InstalledPath string `json:"installed_path"`
}

func (i *ChatGPTAdapter) pluginHome() string { return i.backend.pluginHome() }
func (i *ChatGPTAdapter) pluginRoot(id string) string {
	return filepath.Join(i.pluginHome(), "mcp-connect", "marketplaces", id)
}
func pluginMarket(id string) string   { return "mcp-connect-" + id }
func pluginSelector(id string) string { return id + "@" + pluginMarket(id) }
func readPluginMCP(path, id string) (Registration, bool, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return Registration{}, false, nil
	}
	if e != nil {
		return Registration{}, false, errors.New("Cannot read local plugin")
	}
	var doc struct {
		Servers map[string]pluginRegistration `json:"mcpServers"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return Registration{}, false, errors.New("Local plugin configuration is invalid")
	}
	if len(doc.Servers) != 1 {
		return Registration{}, false, errors.New("Local plugin has unexpected MCP entries")
	}
	r, ok := doc.Servers[id]
	if ok && !owned(Registration(r), id) {
		return Registration(r), ok, errors.New("Name conflict — external plugin server")
	}
	return Registration(r), ok, nil
}
func (i *ChatGPTAdapter) pluginOwner(id string) (pluginReceipt, bool, error) {
	path := filepath.Join(i.pluginRoot(id), "owner.json")
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		if _, err := os.Lstat(i.pluginRoot(id)); err == nil {
			return pluginReceipt{}, false, errors.New("Name conflict — unowned plugin directory")
		}
		return pluginReceipt{}, false, nil
	}
	if e != nil {
		return pluginReceipt{}, false, errors.New("Cannot read plugin ownership")
	}
	var receipt pluginReceipt
	if json.Unmarshal(b, &receipt) != nil || receipt.Owner != Service || receipt.ID != id {
		return receipt, false, errors.New("Name conflict — external plugin directory")
	}
	if receipt.InstalledPath != "" {
		home := canonicalPath(i.pluginHome())
		base := filepath.Join(home, "plugins", "cache", pluginMarket(id), id)
		target := canonicalPath(receipt.InstalledPath)
		rel, err := filepath.Rel(base, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return receipt, false, errors.New("Invalid plugin installation path")
		}
	}
	return receipt, true, nil
}
func (i *ChatGPTAdapter) Read(id string) (Registration, bool, error) {
	if !idPattern.MatchString(id) {
		return Registration{}, false, errors.New("Invalid integration ID")
	}
	receipt, exists, e := i.pluginOwner(id)
	if e != nil || !exists {
		return Registration{}, false, e
	}
	enabled, e := i.backend.pluginState(pluginMarket(id), pluginSelector(id), i.pluginRoot(id))
	if e != nil || !enabled {
		return Registration{}, false, e
	}
	if receipt.InstalledPath == "" {
		return Registration{}, false, nil
	}
	return readPluginMCP(filepath.Join(receipt.InstalledPath, ".mcp.json"), id)
}
func (i *ChatGPTAdapter) SetupURL(id string) (*url.URL, error) {
	if !idPattern.MatchString(id) {
		return nil, errors.New("Invalid connection ID")
	}
	if _, ok, e := i.pluginOwner(id); e != nil || !ok {
		return nil, errors.New("Save this connection with ChatGPT selected first")
	}
	u := &url.URL{Scheme: "codex", Host: "plugins", Path: "/" + id}
	q := url.Values{}
	q.Set("marketplacePath", filepath.Join(i.pluginRoot(id), ".agents", "plugins", "marketplace.json"))
	u.RawQuery = q.Encode()
	return u, nil
}
func (adapter *ChatGPTAdapter) Set(ctx context.Context, request RegistrationRequest) error {
	i := adapter
	id, on := request.ID, request.Enabled
	if !idPattern.MatchString(id) {
		return errors.New("Invalid connection ID")
	}
	if err := i.checkManagedPath(id); err != nil {
		return err
	}
	receipt, exists, e := i.pluginOwner(id)
	if e != nil {
		return e
	}
	enabled, e := i.backend.pluginState(pluginMarket(id), pluginSelector(id), i.pluginRoot(id))
	if e != nil {
		return e
	}
	if !on && !exists && !enabled {
		return nil
	}
	if !on && !enabled {
		return nil
	}
	if !on && (!exists || receipt.InstalledPath == "") {
		return errors.New("Refusing to remove an unowned OpenAI plugin")
	}
	if exists {
		if _, ok, err := readPluginMCP(filepath.Join(i.pluginRoot(id), "plugins", id, ".mcp.json"), id); err != nil || (!ok && receipt.InstalledPath != "") {
			return errors.New("Owned plugin source was changed; nothing overwritten")
		}
	}
	find := i.host.Find
	if find == nil {
		find = func(string) string { return i.backend.Detect() }
	}
	bin := find("codex")
	if bin == "" {
		return errors.New("ChatGPT setup needs a compatible bundled or installed Codex CLI; update ChatGPT Desktop or install Codex CLI")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := func(args ...string) ([]byte, error) {
		b, err := i.backend.pluginOutput(ctx, bin, args...)
		if err != nil {
			return nil, errors.New("OpenAI could not update the local plugin; update ChatGPT/Codex and retry")
		}
		return b, nil
	}
	if !on {
		if _, ok, err := i.Read(id); err != nil || !ok {
			return errors.New("Cannot verify ownership of installed plugin")
		}
		if _, err := command("plugin", "remove", pluginSelector(id), "--json"); err != nil {
			return err
		}
		if active, err := i.backend.pluginState(pluginMarket(id), pluginSelector(id), i.pluginRoot(id)); err != nil || active {
			return errors.New("OpenAI plugin removal verification failed")
		}
		return nil
	}
	if !filepath.IsAbs(request.Executable) {
		return errors.New("Application path must be absolute")
	}
	root := i.pluginRoot(id)
	// Reject symlinked managed files before replacing executable registration.
	for _, rel := range []string{"owner.json", "plugins", filepath.Join("plugins", id), filepath.Join("plugins", id, ".codex-plugin"), filepath.Join("plugins", id, ".codex-plugin", "plugin.json"), filepath.Join("plugins", id, ".mcp.json"), ".agents", filepath.Join(".agents", "plugins"), filepath.Join(".agents", "plugins", "marketplace.json")} {
		if info, err := os.Lstat(filepath.Join(root, rel)); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Refusing symlinked plugin files")
		}
	}
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Refusing symlinked plugin directory")
	}
	write := func(rel string, value any) error {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return AtomicWrite(filepath.Join(root, rel), append(b, '\n'))
	}
	receipt.Owner = Service
	receipt.ID = id
	// Establish ownership before writing the rest so interrupted setup can retry.
	if e = write("owner.json", receipt); e != nil {
		return e
	}
	displayName := "MCP Connect"
	if request.Name != "" {
		displayName = "MCP · " + request.Name
	}
	manifest := map[string]any{"name": id, "version": Version, "description": "Secure local MCP access through MCP Connect", "author": map[string]string{"name": "SeredaLabs", "url": "https://github.com/SeredaLabs"}, "mcpServers": "./.mcp.json", "requires_local_executor": true, "interface": map[string]string{"displayName": displayName, "shortDescription": "Use your saved MCP connection on this computer", "developerName": "SeredaLabs", "category": "Productivity"}}
	if e = write(filepath.Join("plugins", id, ".codex-plugin", "plugin.json"), manifest); e != nil {
		return e
	}
	if e = write(filepath.Join("plugins", id, ".mcp.json"), map[string]any{"mcpServers": map[string]pluginRegistration{id: {Command: request.Executable, Args: []string{"serve", id}}}}); e != nil {
		return e
	}
	marketplace := map[string]any{"name": pluginMarket(id), "interface": map[string]string{"displayName": "MCP Connect"}, "plugins": []any{map[string]any{"name": id, "source": map[string]string{"source": "local", "path": "./plugins/" + id}, "policy": map[string]string{"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}}}
	if e = write(filepath.Join(".agents", "plugins", "marketplace.json"), marketplace); e != nil {
		return e
	}
	if _, e = command("plugin", "marketplace", "add", root, "--json"); e != nil {
		return e
	}
	b, e := command("plugin", "add", pluginSelector(id), "--json")
	if e != nil {
		return e
	}
	var installed struct {
		PluginID      string `json:"pluginId"`
		InstalledPath string `json:"installedPath"`
	}
	if json.Unmarshal(b, &installed) != nil || installed.PluginID != pluginSelector(id) || installed.InstalledPath == "" {
		return errors.New("OpenAI returned an invalid plugin installation result")
	}
	receipt.InstalledPath = installed.InstalledPath
	if e = write("owner.json", receipt); e != nil {
		return e
	}
	r, ok, e := i.Read(id)
	if e != nil || !ok || !samePath(r.Command, request.Executable) || (r.Enabled != nil && !*r.Enabled) {
		return errors.New("Local ChatGPT plugin verification failed")
	}
	return nil
}

type ChatGPTAdapter struct {
	host    *Integrator
	backend openAIPluginStore
}

func NewChatGPTAdapter(host *Integrator, backend openAIPluginStore) *ChatGPTAdapter {
	return &ChatGPTAdapter{host, backend}
}
func (i *ChatGPTAdapter) ID() string { return "chatgpt" }
func (i *ChatGPTAdapter) Detect() string {
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
func (i *ChatGPTAdapter) Settings(id string) (ClientSettings, error) {
	u, err := i.SetupURL(id)
	return ClientSettings{Title: "ChatGPT plugin", URL: u, Direct: true, Instructions: "Open ChatGPT Desktop → Plugins → MCP Connect"}, err
}

// OpenAI currently shares a local store and executable between its two clients.
// Only the Codex adapter understands that store's format and CLI launcher.
// ChatGPT owns the plugin protocol, manifests, receipts and installation outputs.
type openAIPluginStore interface {
	pluginHome() string
	pluginState(market, selector, root string) (bool, error)
	pluginOutput(context.Context, string, ...string) ([]byte, error)
	Detect() string
}

type pluginRegistration struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled *bool    `json:"enabled,omitempty"`
}

func (i *ChatGPTAdapter) checkManagedPath(id string) error {
	paths := []string{filepath.Join(i.pluginHome(), "mcp-connect"), filepath.Join(i.pluginHome(), "mcp-connect", "marketplaces"), i.pluginRoot(id), filepath.Join(i.pluginRoot(id), "owner.json")}
	for _, path := range paths {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Refusing symlinked plugin files or directories")
		} else if err != nil && !os.IsNotExist(err) {
			return errors.New("Cannot inspect managed plugin path")
		}
	}
	return nil
}
