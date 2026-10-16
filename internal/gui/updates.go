package gui

import (
	"context"
	"errors"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/SeredaLabs/mcp-connect/internal/core"
	"github.com/SeredaLabs/mcp-connect/internal/updater"
)

type updateControls struct {
	app       fyne.App
	window    fyne.Window
	dir       string
	busy      func() bool
	setBusy   func(bool)
	available *updater.Update
	checking  bool
	cancel    context.CancelFunc
	ctx       context.Context
	sidebar   *widget.Button
	status    *widget.Label
	check     *widget.Button
	install   *widget.Button
	auto      *widget.Check
}

func newUpdateControls(a fyne.App, w fyne.Window, dir string, busy func() bool, setBusy func(bool)) *updateControls {
	ctx, cancel := context.WithCancel(context.Background())
	u := &updateControls{app: a, window: w, dir: dir, busy: busy, setBusy: setBusy, ctx: ctx}
	a.Lifecycle().SetOnStopped(cancel)
	u.status = widget.NewLabel("Check for a newer release on GitHub.")
	u.status.Wrapping = fyne.TextWrapWord
	u.sidebar = widget.NewButtonWithIcon("Update available", theme.DownloadIcon(), u.startInstall)
	u.sidebar.Importance = widget.HighImportance
	u.sidebar.Hide()
	u.check = widget.NewButton("Check for updates", u.checkNow)
	u.install = widget.NewButtonWithIcon("Install update", theme.DownloadIcon(), u.startInstall)
	u.install.Hide()
	u.auto = widget.NewCheck("Check for updates on launch", nil)
	u.auto.SetChecked(updater.CheckOnLaunch(dir))
	u.auto.OnChanged = func(enabled bool) {
		if e := updater.SetCheckOnLaunch(dir, enabled); e != nil {
			handler := u.auto.OnChanged
			u.auto.OnChanged = nil
			u.auto.SetChecked(!enabled)
			u.auto.OnChanged = handler
			dialog.ShowError(e, w)
			return
		}
		if enabled {
			u.checkNow()
		}
	}
	return u
}
func (u *updateControls) aboutContent() fyne.CanvasObject {
	privacy := widget.NewLabel("Checks GitHub public releases, including Preview. No server addresses or credentials are sent.")
	privacy.Wrapping = fyne.TextWrapWord
	return container.NewVBox(u.status, container.NewHBox(u.check, u.install), u.auto,
		widget.NewAccordion(widget.NewAccordionItem("Update privacy", privacy)))
}
func (u *updateControls) checkNow() {
	if u.checking || u.cancel != nil {
		return
	}
	u.checking = true
	u.check.Disable()
	u.status.SetText("Checking GitHub…")
	go func() {
		result, e := updater.Check(u.ctx, core.Version)
		if u.ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			u.checking = false
			u.check.Enable()
			if e != nil {
				u.status.SetText(e.Error())
				return
			}
			u.available = result
			if result == nil {
				u.status.SetText("You’re up to date · v" + core.Version)
				u.sidebar.Hide()
				u.install.Hide()
				return
			}
			u.status.SetText("New version available · v" + result.Version)
			u.sidebar.SetText("Update · v" + result.Version)
			u.sidebar.Show()
			u.install.Show()
		})
	}()
}
func (u *updateControls) startInstall() {
	if u.cancel != nil {
		u.cancel()
		return
	}
	if u.available == nil {
		u.checkNow()
		return
	}
	if u.busy() {
		dialog.ShowInformation("Action in progress", "Finish the current connection action before updating.", u.window)
		return
	}
	if !updater.Installed() {
		text := widget.NewLabel("This is a portable copy. Download the new portable archive, close this app and your AI clients, then replace the app at the same location. An installer would move it to a different location.")
		text.Wrapping = fyne.TextWrapWord
		link := widget.NewHyperlink("Download portable update", &url.URL{Scheme: "https", Host: "seredalabs.github.io", Path: "/mcp-connect/"})
		dialog.NewCustom("Update portable copy", "Close", container.NewVBox(text, link), u.window).Show()
		return
	}
	message := "Download and verify v" + u.available.Version + ", then close MCP Connect and open the standard installer. Your profiles and credentials stay in place. Close active AI-client adapter sessions before installing; restart clients afterward."
	dialog.ShowConfirm("Update MCP Connect", message, func(ok bool) {
		if ok {
			u.download()
		}
	}, u.window)
}
func (u *updateControls) download() {
	if u.cancel != nil || u.busy() {
		return
	}
	ctx, cancel := context.WithTimeout(u.ctx, 6*time.Minute)
	u.cancel = cancel
	u.setBusy(true)
	u.check.Disable()
	u.install.SetText("Cancel download")
	u.sidebar.SetText("Cancel update")
	u.status.SetText("Downloading and verifying…")
	go func() {
		latest, e := updater.Check(ctx, core.Version)
		var path string
		if e == nil && latest != nil {
			path, e = updater.Download(ctx, latest)
		}
		canceled := errors.Is(ctx.Err(), context.Canceled)
		var launchError error
		if e == nil && latest != nil && !canceled {
			launchError = updater.OpenInstaller(path)
		}
		cancel()
		if u.ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			u.cancel = nil
			u.setBusy(false)
			u.check.Enable()
			u.install.SetText("Install update")
			if u.available != nil {
				u.sidebar.SetText("Update · v" + u.available.Version)
			}
			if canceled {
				u.status.SetText("Download canceled. You can try again.")
				return
			}
			if e != nil {
				u.status.SetText(e.Error())
				dialog.ShowError(e, u.window)
				return
			}
			if latest == nil {
				u.available = nil
				u.sidebar.Hide()
				u.install.Hide()
				u.status.SetText("You’re up to date · v" + core.Version)
				return
			}
			if launchError != nil {
				u.status.SetText("Verified installer saved: " + path)
				dialog.ShowInformation("Open installer manually", launchError.Error()+"\n\n"+path+"\n\nOn Linux, you can use: sudo apt install <path-to-downloaded.deb>", u.window)
				return
			}
			u.app.Quit()
		})
	}()
}
