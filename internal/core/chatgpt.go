package core

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"net/url"
	"os"
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

func (i *Integrator) pluginHome() string {
	if i.CodexPath != "" {
		return filepath.Dir(i.CodexPath)
	}
	return filepath.Join(i.Home, ".codex")
}
func (i *Integrator) pluginRoot(id string) string {
	return filepath.Join(i.pluginHome(), "mcp-connect", "marketplaces", id)
}
func pluginMarket(id string) string   { return "mcp-connect-" + id }
func pluginSelector(id string) string { return id + "@" + pluginMarket(id) }
func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(platformLongPath(path))
}
func samePath(a, b string) bool {
	a, b = canonicalPath(a), canonicalPath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func (i *Integrator) pluginConfig(id string) (bool, error) {
	b, e := os.ReadFile(i.CodexPath)
	if os.IsNotExist(e) || i.CodexPath == "" {
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
	if m, ok := doc.Marketplaces[pluginMarket(id)]; ok && (m.SourceType != "local" || !samePath(m.Source, i.pluginRoot(id))) {
		return false, errors.New("Name conflict — external OpenAI marketplace")
	}
	return doc.Plugins[pluginSelector(id)].Enabled, nil
}
func readPluginMCP(path, id string) (Registration, bool, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return Registration{}, false, nil
	}
	if e != nil {
		return Registration{}, false, errors.New("Cannot read local plugin")
	}
	var doc struct {
		Servers map[string]Registration `json:"mcpServers"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return Registration{}, false, errors.New("Local plugin configuration is invalid")
	}
	if len(doc.Servers) != 1 {
		return Registration{}, false, errors.New("Local plugin has unexpected MCP entries")
	}
	r, ok := doc.Servers[id]
	if ok && !owned(r, id) {
		return r, ok, errors.New("Name conflict — external plugin server")
	}
	return r, ok, nil
}
func (i *Integrator) pluginOwner(id string) (pluginReceipt, bool, error) {
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
func (i *Integrator) readChatGPT(id string) (Registration, bool, error) {
	receipt, exists, e := i.pluginOwner(id)
	if e != nil || !exists {
		return Registration{}, false, e
	}
	enabled, e := i.pluginConfig(id)
	if e != nil || !enabled {
		return Registration{}, false, e
	}
	if receipt.InstalledPath == "" {
		return Registration{}, false, nil
	}
	return readPluginMCP(filepath.Join(receipt.InstalledPath, ".mcp.json"), id)
}
func (i *Integrator) ChatGPTSetupURL(id string) (*url.URL, error) {
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
func (i *Integrator) setChatGPT(ctx context.Context, id string, on bool) error {
	if !idPattern.MatchString(id) {
		return errors.New("Invalid connection ID")
	}
	receipt, exists, e := i.pluginOwner(id)
	if e != nil {
		return e
	}
	enabled, e := i.pluginConfig(id)
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
	find := i.Find
	if find == nil {
		find = FindClient
	}
	bin := find("codex")
	if bin == "" {
		return errors.New("ChatGPT setup needs a compatible bundled or installed Codex CLI; update ChatGPT Desktop or install Codex CLI")
	}
	if i.Output == nil {
		return errors.New("Plugin installer unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := func(args ...string) ([]byte, error) {
		b, err := i.Output(ctx, bin, args...)
		if err != nil {
			return nil, errors.New("OpenAI could not update the local plugin; update ChatGPT/Codex and retry")
		}
		return b, nil
	}
	if !on {
		if _, ok, err := i.readChatGPT(id); err != nil || !ok {
			return errors.New("Cannot verify ownership of installed plugin")
		}
		if _, err := command("plugin", "remove", pluginSelector(id), "--json"); err != nil {
			return err
		}
		if active, err := i.pluginConfig(id); err != nil || active {
			return errors.New("OpenAI plugin removal verification failed")
		}
		return nil
	}
	if !filepath.IsAbs(i.Executable) {
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
	if i.DisplayName != "" {
		displayName = "MCP · " + i.DisplayName
	}
	manifest := map[string]any{"name": id, "version": Version, "description": "Secure local MCP access through MCP Connect", "author": map[string]string{"name": "SeredaLabs", "url": "https://github.com/SeredaLabs"}, "mcpServers": "./.mcp.json", "requires_local_executor": true, "interface": map[string]string{"displayName": displayName, "shortDescription": "Use your saved MCP connection on this computer", "developerName": "SeredaLabs", "category": "Productivity"}}
	if e = write(filepath.Join("plugins", id, ".codex-plugin", "plugin.json"), manifest); e != nil {
		return e
	}
	if e = write(filepath.Join("plugins", id, ".mcp.json"), map[string]any{"mcpServers": map[string]Registration{id: {Command: i.Executable, Args: []string{"serve", id}}}}); e != nil {
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
	r, ok, e := i.readChatGPT(id)
	if e != nil || !ok || !samePath(r.Command, i.Executable) {
		return errors.New("Local ChatGPT plugin verification failed")
	}
	return nil
}
