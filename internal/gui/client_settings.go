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
	"github.com/SeredaLabs/mcp-connect/internal/core"
)

// Render adapter-provided guidance without knowing client registration details.
func showClientSettings(a fyne.App, w fyne.Window, settings core.ClientSettings) {
	configPath := settings.ConfigPath
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

	title := settings.Title
	instructions := widget.NewLabel(settings.Instructions)
	instructions.Wrapping = fyne.TextWrapWord
	primary := widget.NewButtonWithIcon(settings.ActionLabel, theme.ContentCopyIcon(), func() { a.Clipboard().SetContent(settings.CopyText) })
	var openDesktop func()
	if settings.URL != nil {
		openDesktop = func() {
			if err := a.OpenURL(settings.URL); err != nil {
				instructions.SetText("Could not open the app. " + settings.Instructions)
			}
		}
		primary = widget.NewButtonWithIcon(settings.ActionLabel, theme.NavigateNextIcon(), openDesktop)
	}
	primary.Importance = widget.HighImportance
	popup := dialog.NewCustom(title, "Close", container.NewVBox(instructions, container.NewHBox(primary, openConfig)), w)
	popup.Resize(fyne.NewSize(540, 300))
	popup.Show()
	if openDesktop != nil && settings.OpenOnShow {
		openDesktop()
	}
}
