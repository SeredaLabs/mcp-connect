package gui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Claude documents chat/session links, but no direct Developer-settings link.
// Keep the native-app guidance separate from the optional config-file action.
func showClaudeSettings(a fyne.App, w fyne.Window, client, configPath string) {
	openConfig := widget.NewButtonWithIcon("Configuration file", theme.DocumentIcon(), func() {
		info, err := os.Stat(configPath)
		if os.IsNotExist(err) {
			dialog.ShowInformation("Configuration file", "No configuration file yet. Select this app and use Save & Connect first.", w)
			return
		}
		if err != nil || info.IsDir() {
			dialog.ShowInformation("Configuration file", "Cannot open this app's configuration file. Check its installation and file permissions.", w)
			return
		}
		path := filepath.ToSlash(configPath)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if err := a.OpenURL(&url.URL{Scheme: "file", Path: path}); err != nil {
			dialog.ShowInformation("Configuration file", "Open this file in your editor: "+configPath, w)
		}
	})
	openConfig.Importance = widget.LowImportance

	title := "Claude Code · MCP settings"
	instructions := widget.NewLabel("In your existing Claude Code terminal session, enter /mcp to open the MCP menu and check your servers.\n\nIf Claude Code is not running, start it with claude first.")
	instructions.Wrapping = fyne.TextWrapWord
	primary := widget.NewButtonWithIcon("Copy /mcp", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent("/mcp")
	})
	var openDesktop func()
	if client == "claude-desktop" {
		title = "Claude Desktop · MCP settings"
		instructions.SetText("In Claude, open:\nSettings → Desktop app → Developer\n\nThis is where you can check local MCP servers and their logs. Restart Claude after Save & Connect to load updated settings.")
		openDesktop = func() {
			if err := a.OpenURL(&url.URL{Scheme: "claude", Host: "claude.ai", Path: "/new"}); err != nil {
				instructions.SetText("Could not open Claude Desktop. Launch it from your applications, then go to:\nSettings → Desktop app → Developer")
			}
		}
		primary = widget.NewButtonWithIcon("Open Claude Desktop", theme.NavigateNextIcon(), openDesktop)
	}
	primary.Importance = widget.HighImportance
	popup := dialog.NewCustom(title, "Close", container.NewVBox(instructions, container.NewHBox(primary, openConfig)), w)
	popup.Resize(fyne.NewSize(540, 300))
	popup.Show()
	if openDesktop != nil {
		openDesktop()
	}
}
