package gui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/SeredaLabs/mcp-connect/assets"
	"github.com/SeredaLabs/mcp-connect/internal/core"
	"image/color"
	"net/url"
	"strings"
	"time"
)

func Run(store *core.Store) error {
	integrator, err := core.NewIntegrator()
	if err != nil {
		return err
	}
	a := app.NewWithID("eu.onecmcp.connect")
	w := newWindow(a, store, integrator)
	w.ShowAndRun()
	return nil
}

func newWindow(a fyne.App, store *core.Store, integrator *core.Integrator) fyne.Window {
	brandIcon := fyne.NewStaticResource("mcp-connect.png", assets.Icon)
	a.SetIcon(brandIcon)
	a.Settings().SetTheme(&connectTheme{})
	w := a.NewWindow("MCP Connect")
	w.Resize(fyne.NewSize(1020, 800))
	w.SetMaster()
	var connections []core.Connection
	var selected string
	var busy bool
	var loading bool
	var savedProfile *core.Connection
	var credentialsChanged bool
	clientResults := map[string]map[string]core.ClientResult{}
	retryButtons := map[string]*widget.Button{}
	changes := widget.NewLabel("Changes apply when you choose Save & Apply.")
	changes.Wrapping = fyne.TextWrapWord
	type checkResult struct {
		endpoint string
		at       time.Time
		ok       bool
	}
	checks := map[string]checkResult{}
	serverLamp := newIndicator("Server not checked", neutralTone)
	feedbackLamp := newIndicator("Ready", neutralTone)
	progress := widget.NewProgressBarInfinite()
	progress.Stop()
	progress.Hide()
	name := widget.NewEntry()
	name.SetPlaceHolder("My workspace")
	endpoint := widget.NewEntry()
	endpoint.SetPlaceHolder("https://mcp.example.com/mcp")
	login := widget.NewEntry()
	password := widget.NewPasswordEntry()
	loadedEndpoint := ""
	password.SetPlaceHolder("Leave blank to use saved password")
	authLabels := map[string]string{core.AuthBasic: "Login & password", core.AuthBearer: "Bearer token", core.AuthAPIKey: "API key"}
	authMethod := newAuthSelector([]string{"Login & password", "Bearer token", "API key"})
	authMethod.SetSelected("Login & password")
	apiHeader := widget.NewSelect([]string{"X-API-Key", "Api-Key", "X-Auth-Token"}, nil)
	apiHeader.SetSelected("X-API-Key")
	headerLabel := func(header string) string {
		if core.APIKeyHeader(header) == "X-Api-Key" {
			return "X-API-Key"
		}
		return core.APIKeyHeader(header)
	}
	currentAuth := func() string {
		switch authMethod.Selected {
		case "Bearer token":
			return core.AuthBearer
		case "API key":
			return core.AuthAPIKey
		default:
			return core.AuthBasic
		}
	}
	var refreshAuthFields func()

	claude := newHintCheck("Claude Code", func() string { return clientHoverHint("claude") })
	codex := newHintCheck("Codex", func() string { return clientHoverHint("codex") })
	desktop := newHintCheck("Claude Desktop", func() string { return clientHoverHint("claude-desktop") })
	chatgpt := newHintCheck("ChatGPT Desktop", func() string { return clientHoverHint("chatgpt") })
	status := widget.NewLabel("Add a connection, then test and connect.")
	status.Truncation = fyne.TextTruncateEllipsis
	details := widget.NewLabel("")
	details.Truncation = fyne.TextTruncateEllipsis
	details.Hide()
	var fullDetails string
	var verified *core.ProbeResult
	var checkedAt time.Time
	var verificationNote string
	setDetails := func(text string) {
		fullDetails = text
		verified = nil
		checkedAt = time.Time{}
		verificationNote = ""
		details.SetText(strings.Join(strings.Fields(text), " "))
		if text == "" {
			details.Hide()
		} else {
			details.Show()
		}
	}
	var list *widget.List
	var buttons []*widget.Button
	pageTitle := canvas.NewText("New connection", ink)
	pageTitle.TextSize = 23
	pageTitle.TextStyle.Bold = true
	clientLabels := map[string]*indicator{}
	for _, key := range []string{"claude", "codex", "claude-desktop", "chatgpt"} {
		clientLabels[key] = newIndicator("Not configured", neutralTone)
	}
	chosenClients := func() map[string]bool {
		return map[string]bool{"claude": claude.Checked, "codex": codex.Checked, "claude-desktop": desktop.Checked, "chatgpt": chatgpt.Checked}
	}
	formDirty := func() bool {
		if savedProfile == nil {
			return name.Text != "" || endpoint.Text != "" || login.Text != "" || password.Text != ""
		}
		if credentialsChanged || strings.TrimSpace(name.Text) != savedProfile.Name || strings.TrimSpace(endpoint.Text) != savedProfile.URL || currentAuth() != core.AuthMode(savedProfile.AuthType) || (currentAuth() == core.AuthAPIKey && core.APIKeyHeader(apiHeader.Selected) != core.APIKeyHeader(savedProfile.AuthHeader)) {
			return true
		}
		for client, chosen := range chosenClients() {
			if chosen != savedProfile.WantsClient(client) {
				return true
			}
		}
		return false
	}
	refreshStatus := func() {
		chosen := chosenClients()
		for _, key := range core.ClientIDs {
			state := core.ClientState{}
			if selected != "" {
				state = integrator.Inspect(key, selected)
			}
			pending := savedProfile != nil && chosen[key] != savedProfile.WantsClient(key)
			result, hasResult := clientResults[selected][key]
			value, tone := clientStatus(state, chosen[key], pending, hasResult && result.Err != nil)
			if savedProfile != nil && !savedProfile.Enabled && state.Registered && !pending {
				value, tone = "Cleanup needed", amberTone
			}
			clientLabels[key].Set(value, tone)
			if retry := retryButtons[key]; retry != nil {
				if hasResult && result.Err != nil && !formDirty() {
					retry.Show()
				} else {
					retry.Hide()
				}
			}
		}
		if formDirty() {
			changes.SetText("Unsaved changes · choose Save & Apply.")
		} else if savedProfile != nil && !savedProfile.Enabled {
			changes.SetText("Profile disabled. Save & Apply reconnects the selected clients.")
		} else {
			changes.SetText("Select clients, then Save & Apply. Badges open settings.")
		}
	}

	refreshServerLamp := func() {
		if savedProfile != nil && !savedProfile.Enabled {
			serverLamp.Set("Profile disabled", neutralTone)
			return
		}
		result, ok := checks[selected]
		if ok && result.endpoint == endpoint.Text {
			if result.ok {
				serverLamp.Set("Verified · "+result.at.Format("15:04"), greenTone)
			} else {
				serverLamp.Set("Check failed", redTone)
			}
		} else {
			serverLamp.Set("Server not checked", neutralTone)
		}
	}
	invalidateCheck := func() {
		if loading || busy {
			return
		}
		credentialsChanged = true
		refreshStatus()
		delete(checks, selected)
		serverLamp.Set("Changes not checked", amberTone)
		feedbackLamp.Set("Test required", amberTone)
		status.SetText("Connection settings changed")
		setDetails("Test the server to verify these settings. No background checks are performed.")
		if list != nil {
			list.Refresh()
		}
	}
	name.OnChanged = func(string) {
		if !loading && !busy {
			refreshStatus()
		}
	}
	endpoint.OnChanged = func(string) { invalidateCheck() }
	login.OnChanged = func(string) { invalidateCheck() }
	password.OnChanged = func(string) { loadedEndpoint = ""; invalidateCheck() }
	authMethod.OnChanged = func(string) {
		if !loading {
			login.SetText("")
			password.SetText("")
			loadedEndpoint = ""
			invalidateCheck()
		}
		if refreshAuthFields != nil {
			refreshAuthFields()
		}
	}
	apiHeader.OnChanged = func(string) {
		if !loading {
			password.SetText("")
			loadedEndpoint = ""
			invalidateCheck()
		}
	}
	for _, check := range []*hintCheck{claude, codex, desktop, chatgpt} {
		check.OnChanged = func(bool) {
			if !loading {
				refreshStatus()
			}
		}
	}

	reload := func() {
		cs, e := store.Load()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		connections = cs
		for _, c := range cs {
			if c.ID == selected {
				savedProfile = &c
				break
			}
		}
		if list != nil {
			list.Refresh()
		}
		refreshStatus()
	}
	clear := func() {
		loading = true
		defer func() { loading = false }()
		selected = ""
		savedProfile = nil
		credentialsChanged = false
		loadedEndpoint = ""
		authMethod.SetSelected("Login & password")
		apiHeader.SetSelected("X-API-Key")
		pageTitle.Text = "New connection"
		pageTitle.Refresh()
		name.SetText("")
		endpoint.SetText("")
		login.SetText("")
		password.SetText("")
		claude.SetChecked(core.FindClient("claude") != "")
		codex.SetChecked(core.FindClient("codex") != "")
		desktop.SetChecked(core.FindClient("claude-desktop") != "")
		chatgpt.SetChecked(false)
		setDetails("")
		status.SetText("New connection")
		serverLamp.Set("Server not checked", neutralTone)
		feedbackLamp.Set("Ready", neutralTone)
		refreshStatus()
	}
	list = widget.NewList(func() int { return len(connections) }, func() fyne.CanvasObject {
		title := widget.NewLabelWithStyle("Connection", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		title.Truncation = fyne.TextTruncateEllipsis
		return container.NewVBox(title, newIndicator("Disabled", neutralTone))
	}, func(id widget.ListItemID, o fyne.CanvasObject) {
		box := o.(*fyne.Container)
		c := connections[id]
		box.Objects[0].(*widget.Label).SetText(c.Name)
		text, tone := "Disabled", neutralTone
		if c.Enabled {
			text, tone = "Integration enabled", blueTone
		}
		if result, ok := checks[c.ID]; ok && c.Enabled && result.endpoint == c.URL {
			if result.ok {
				text, tone = "Verified · "+result.at.Format("15:04"), greenTone
			} else {
				text, tone = "Check failed", redTone
			}
		}
		box.Objects[1].(*indicator).Set(text, tone)
	})
	list.OnSelected = func(id widget.ListItemID) {
		if busy {
			if connections[id].ID != selected {
				for n, c := range connections {
					if c.ID == selected {
						list.Select(n)
						return
					}
				}
				list.UnselectAll()
			}
			return
		}
		loading = true
		defer func() { loading = false }()
		c := connections[id]
		selected = c.ID
		savedProfile = &c
		credentialsChanged = false
		loadedEndpoint = ""
		authMethod.SetSelected(authLabels[core.AuthMode(c.AuthType)])
		apiHeader.SetSelected(headerLabel(c.AuthHeader))
		pageTitle.Text = c.Name
		pageTitle.Refresh()
		name.SetText(c.Name)
		endpoint.SetText(c.URL)
		password.SetText("")
		login.SetText("")
		claude.SetChecked(c.Claude)
		codex.SetChecked(c.Codex)
		desktop.SetChecked(c.ClaudeDesktop)
		chatgpt.SetChecked(c.ChatGPT)
		setDetails("")
		status.SetText("Saved connection")
		feedbackLamp.Set("Saved", blueTone)
		refreshServerLamp()
		setDetails("Credentials are kept in your system vault. Leave the fields blank to reuse them.")
		refreshStatus()
	}
	snapshot := func() (core.Connection, core.Credentials, error) {
		id := selected
		if id == "" {
			id = core.NewID()
		}
		c := core.Connection{AuthType: currentAuth(), ID: id, Name: strings.TrimSpace(name.Text), URL: strings.TrimSpace(endpoint.Text), Claude: claude.Checked, Codex: codex.Checked, ClaudeDesktop: desktop.Checked, ChatGPT: chatgpt.Checked, Enabled: true}
		if currentAuth() == core.AuthAPIKey {
			c.AuthHeader = apiHeader.Selected
		}
		if e := core.Validate(c); e != nil {
			return c, core.Credentials{}, e
		}
		cred := core.Credentials{AuthType: c.AuthType, AuthHeader: c.AuthHeader, Endpoint: loadedEndpoint}
		if currentAuth() == core.AuthBasic {
			cred.Login = login.Text
			cred.Password = password.Text
		} else {
			cred.Token = password.Text
		}
		return c, cred, nil
	}
	resolveCredentials := store.ResolveCredentials

	setBusy := func(on bool) {
		busy = on
		for _, b := range buttons {
			if on {
				b.Disable()
			} else {
				b.Enable()
			}
		}
		for _, entry := range []*widget.Entry{name, endpoint, login, password} {
			if on {
				entry.Disable()
			} else {
				entry.Enable()
			}
		}
		if on {
			authMethod.Disable()
			apiHeader.Disable()
			claude.Disable()
			codex.Disable()
			desktop.Disable()
			chatgpt.Disable()
		} else {
			authMethod.Enable()
			apiHeader.Enable()
			claude.Enable()
			codex.Enable()
			desktop.Enable()
			chatgpt.Enable()
		}
	}
	work := func(kind string, job func() (string, string, error), after func()) {
		id, url := selected, endpoint.Text
		setBusy(true)
		setDetails("")
		status.SetText("Working…")
		feedbackLamp.Set("In progress", amberTone)
		if kind == "test" || kind == "connect" {
			serverLamp.Set("Checking…", amberTone)
		}
		progress.Start()
		progress.Show()
		go func() {
			s, d, e := job()
			fyne.Do(func() {
				setBusy(false)
				progress.Stop()
				progress.Hide()
				if kind == "test" {
					checks[id] = checkResult{endpoint: url, at: time.Now(), ok: e == nil}
				}
				if kind == "connect" && e == nil {
					checks[id] = checkResult{endpoint: url, at: time.Now(), ok: true}
				}
				if kind == "disconnect" || kind == "delete" {
					delete(checks, id)
				}
				refreshServerLamp()
				if e != nil {
					status.SetText("Action incomplete")
					feedbackLamp.Set("Needs attention", redTone)
					setDetails(e.Error())
					dialog.ShowError(e, w)
				} else {
					status.SetText(s)
					tone, label := blueTone, "Updated"
					if kind == "test" || kind == "connect" {
						tone, label = greenTone, "Verified"
					}
					if kind == "disconnect" || kind == "delete" {
						tone, label = neutralTone, "Disabled"
					}
					feedbackLamp.Set(label, tone)
					setDetails(d)
					if after != nil {
						after()
					}
				}
				reload()
				refreshServerLamp()
			})
		}()
	}
	test := widget.NewButtonWithIcon("Test Server", testConnectionIcon, func() {
		c, cred, e := snapshot()
		if e != nil {
			feedbackLamp.Set("Check settings", redTone)
			status.SetText("Connection settings need attention")
			setDetails(e.Error())
			dialog.ShowError(e, w)
			return
		}
		var result core.ProbeResult
		work("test", func() (string, string, error) {
			var err error
			cred, err = resolveCredentials(c, cred)
			if err != nil {
				return "", "", err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			r, e := core.Probe(ctx, c.URL, cred)
			if e != nil {
				return "", "", e
			}
			result = r
			return fmt.Sprintf("Authorization OK · MCP %s · %d tools", r.Protocol, len(r.Tools)), r.Server + "\n" + strings.Join(r.Tools, " · "), nil
		}, func() {
			verified = &result
			checkedAt = time.Now()
		})
	})
	showReport := func(c core.Connection, report core.ApplyReport) {
		if clientResults[c.ID] == nil {
			clientResults[c.ID] = map[string]core.ClientResult{}
		}
		for _, result := range report {
			clientResults[c.ID][result.Client] = result
		}
		text, failed := integrationSummary(clientResults[c.ID])
		if !c.Enabled {
			text += "\nProfile disabled. Close existing client sessions; new adapter sessions are blocked."
		}
		setDetails(text)
		if !c.Enabled && failed > 0 {
			status.SetText("Disabled · cleanup needed")
			feedbackLamp.Set("Retry cleanup", amberTone)
			details.SetText("New adapter sessions are blocked. Close existing sessions; retry remaining cleanup.")
		} else if failed > 0 {
			status.SetText("Saved · some clients need attention")
			feedbackLamp.Set("Retry available", amberTone)
			details.SetText("Successful changes kept. Retry the affected client; open Details for errors.")
		} else if !c.Enabled {
			status.SetText("Profile disabled · client entries removed")
			feedbackLamp.Set("Disabled", neutralTone)
			details.SetText("New adapter sessions are blocked. Close any existing client sessions.")
		} else {
			status.SetText("Applied · restart your clients")
			feedbackLamp.Set("Applied", blueTone)
			if c.ChatGPT {
				details.SetText("Restart clients. Enable the ChatGPT plugin in a local-execution chat.")
			} else {
				details.SetText("Client settings saved. Test Server checks the remote connection separately.")
			}
		}
	}
	save := widget.NewButtonWithIcon("Save & Apply", connectIcon, func() {
		c, cred, e := snapshot()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		existing, findErr := store.Find(c.ID)
		if !c.Claude && !c.Codex && !c.ClaudeDesktop && !c.ChatGPT {
			if findErr != nil {
				dialog.ShowInformation("Choose a client", "Select at least one client for this new connection.", w)
				return
			}
			c.Enabled = false
		}
		localOnly := findErr == nil && existing.URL == c.URL && core.AuthMode(existing.AuthType) == core.AuthMode(c.AuthType) && core.APIKeyHeader(existing.AuthHeader) == core.APIKeyHeader(c.AuthHeader) && !credentialsChanged
		selected = c.ID
		var report core.ApplyReport
		var result *core.ProbeResult
		kind := "apply"
		if !localOnly {
			kind = "connect"
		}
		work(kind, func() (string, string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if localOnly {
				if err := store.SaveSettings(c); err != nil {
					return "", "", err
				}
			} else {
				var err error
				cred, err = resolveCredentials(c, cred)
				if err != nil {
					return "", "", err
				}
				probeCtx, stopProbe := context.WithTimeout(ctx, 30*time.Second)
				r, err := core.Probe(probeCtx, c.URL, cred)
				stopProbe()
				if err != nil {
					return "", "", err
				}
				result = &r
				if err := store.Save(c, cred); err != nil {
					return "", "", err
				}
			}
			report = integrator.Apply(ctx, c)
			return "Client settings applied", "", nil
		}, func() {
			savedProfile = &c
			credentialsChanged = false
			loading = true
			password.SetText("")
			login.SetText("")
			loadedEndpoint = ""
			loading = false
			pageTitle.Text = c.Name
			pageTitle.Refresh()
			showReport(c, report)
			if result != nil {
				verified = result
				checkedAt = time.Now()
				verificationNote = fullDetails
			}
		})
	})

	save.Importance = widget.HighImportance
	edit := widget.NewButtonWithIcon("Edit saved credentials", theme.DocumentCreateIcon(), func() {
		if selected == "" {
			return
		}
		id := selected
		var cred core.Credentials
		work("credentials", func() (string, string, error) {
			var e error
			cred, e = store.Vault.Get(id)
			return "Editing connection", "", e
		}, func() {
			loading = true
			defer func() { loading = false }()
			authMethod.SetSelected(authLabels[core.AuthMode(cred.AuthType)])
			apiHeader.SetSelected(headerLabel(cred.AuthHeader))
			login.SetText(cred.Login)
			if core.AuthMode(cred.AuthType) == core.AuthBasic {
				password.SetText(cred.Password)
			} else {
				password.SetText(cred.Token)
			}
			loadedEndpoint = cred.Endpoint
			if loadedEndpoint == "" {
				if saved, err := store.Find(id); err == nil {
					loadedEndpoint = saved.URL
				}
			}
		})
	})
	disconnect := widget.NewButtonWithIcon("Disconnect", disconnectIcon, func() {
		if selected == "" {
			return
		}
		c, e := store.Find(selected)
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		var report core.ApplyReport
		work("disconnect", func() (string, string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			var err error
			report, err = store.DisableClients(ctx, integrator, c)
			return "Disconnected", "Saved credentials retained for reconnecting.", err
		}, func() {
			c.Enabled = false
			savedProfile = &c
			showReport(c, report)
		})
	})
	remove := widget.NewButtonWithIcon("Delete", deleteConnectionIcon, func() {
		if selected == "" {
			return
		}
		c, e := store.Find(selected)
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		dialog.ShowConfirm("Delete connection?", "Remove this connection, client integrations and its system credentials?", func(ok bool) {
			if !ok {
				return
			}
			work("delete", func() (string, string, error) {
				if e := store.Disable(context.Background(), integrator, c); e != nil {
					return "", "", e
				}
				e := store.Remove(c.ID)
				return "Connection deleted", "", e
			}, clear)
		}, w)
	})
	confirmDiscard := func(next func()) {
		if formDirty() {
			dialog.ShowConfirm("Discard unsaved changes?", "Your edits have not been applied. Discard them and continue?", func(ok bool) {
				if ok {
					next()
				}
			}, w)
			return
		}
		next()
	}
	loadSelection := list.OnSelected
	restoringSelection := false
	selectionPending := false
	list.OnSelected = func(id widget.ListItemID) {
		if restoringSelection {
			return
		}
		if busy || !formDirty() {
			loadSelection(id)
			return
		}
		restoreSelection := func() {
			restoringSelection = true
			defer func() { restoringSelection = false }()
			list.UnselectAll()
			for n, c := range connections {
				if c.ID == selected {
					list.Select(n)
					break
				}
			}
		}
		restoreSelection()
		if selectionPending {
			return
		}
		selectionPending = true
		dialog.ShowConfirm("Discard unsaved changes?", "Your edits have not been applied. Discard them and open another connection?", func(ok bool) {
			selectionPending = false
			if !ok || busy {
				return
			}
			loadSelection(id)
			restoringSelection = true
			list.Select(id)
			restoringSelection = false
		}, w)
	}
	add := widget.NewButtonWithIcon("Add Connection", theme.ContentAddIcon(), func() {
		confirmDiscard(func() { list.UnselectAll(); clear() })
	})
	buttons = []*widget.Button{test, save, edit, disconnect, remove, add}
	edit.Importance = widget.LowImportance
	disconnect.Importance = widget.LowImportance
	remove.Importance = widget.LowImportance

	openClientSettings := func(key string) {
		if busy {
			return
		}
		settings, err := integrator.Settings(key, selected)
		if err != nil {
			dialog.ShowInformation(core.ClientName(key), err.Error(), w)
			return
		}
		if settings.Direct {
			if err := a.OpenURL(settings.URL); err != nil {
				dialog.ShowInformation(settings.Title, settings.Instructions, w)
			}
			return
		}
		showClientSettings(a, w, settings)
	}
	clientRow := func(check *hintCheck, key string) fyne.CanvasObject {
		clientLabels[key].label.Truncation = fyne.TextTruncateEllipsis
		retry := widget.NewButton("Retry", func() {
			if busy || formDirty() {
				return
			}
			c, err := store.Find(selected)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			var report core.ApplyReport
			work("apply", func() (string, string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				report = integrator.Apply(ctx, c, key)
				return "Client settings applied", "", nil
			}, func() { showReport(c, report) })
		})
		retry.Importance = widget.LowImportance
		retry.Hide()
		retryButtons[key] = retry
		buttons = append(buttons, retry)
		badge := newIndicatorAction(clientLabels[key], func() { openClientSettings(key) })
		row := container.NewGridWithColumns(2, check, badge)
		return container.NewVBox(row, retry)
	}
	group := func(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
		text := canvas.NewText(title, ink)
		text.TextSize = 15
		text.TextStyle.Bold = true
		return panel(container.NewVBox(text, container.NewVBox(rows...)), 8)
	}

	field := func(label string, entry fyne.CanvasObject) fyne.CanvasObject {
		caption := canvas.NewText(label, ink)
		caption.TextSize = 12
		caption.TextStyle.Bold = true
		return container.NewVBox(caption, entry)
	}
	edit.SetText("Edit credentials")
	credentialFields := container.NewGridWithColumns(2, field("Login", login), field("Password", password))
	refreshAuthFields = func() {
		credentialFields.Layout = layout.NewGridLayoutWithColumns(2)
		switch currentAuth() {
		case core.AuthBearer:
			credentialFields.Layout = layout.NewGridLayoutWithColumns(1)
			password.SetPlaceHolder("Enter token, or leave blank to keep the saved token")
			credentialFields.Objects = []fyne.CanvasObject{field("Bearer token", password)}
		case core.AuthAPIKey:
			password.SetPlaceHolder("Leave blank to keep the saved key")
			credentialFields.Objects = []fyne.CanvasObject{field("API key header", apiHeader), field("API key", password)}
		default:
			password.SetPlaceHolder("Leave blank to use saved password")
			credentialFields.Objects = []fyne.CanvasObject{field("Login", login), field("Password", password)}
		}
		credentialFields.Refresh()
	}
	refreshAuthFields()
	fields := container.NewVBox(
		container.NewGridWithColumns(2, field("Connection name", name), field("Server HTTPS URL", endpoint)), authMethod, credentialFields)
	title := canvas.NewText("Connection settings", ink)
	title.TextSize = 16
	title.TextStyle.Bold = true
	settingsCard := panel(container.NewVBox(container.NewBorder(nil, nil, title, edit), fields), 12)
	clients := container.NewGridWithColumns(2, group("OpenAI", clientRow(chatgpt, "chatgpt"), clientRow(codex, "codex")), group("Anthropic", clientRow(desktop, "claude-desktop"), clientRow(claude, "claude")))
	clientsTitle := canvas.NewText("Connect your apps", ink)
	clientsTitle.TextSize = 16
	clientsTitle.TextStyle.Bold = true
	connectionHelp := widget.NewButtonWithIcon("How to connect", theme.HelpIcon(), func() {
		section := func(title, body string) *widget.AccordionItem {
			text := widget.NewLabel(body)
			text.Wrapping = fyne.TextWrapWord
			return widget.NewAccordionItem(title, text)
		}
		basics := widget.NewAccordion(
			section("Connect once", "Choose your clients, then Save & Apply. Checkboxes are your choices; badges show registration. Unsaved changes do not alter clients. Test Server checks only the server. Changing client choices does not require a server check. Retry fixes one failed client. Restart clients and start a new chat after applying."),
			section("OpenAI", "Codex uses a local MCP adapter. ChatGPT Desktop uses a local plugin: enable it in a chat that runs on this computer. The plugin can also appear in Codex. Codex keeps its direct MCP entry for CLI and IDE compatibility. If both entries appear in a Codex session, enable only one there. Click a status badge to open settings."),
			section("Anthropic", "Claude Code registers for all projects of this user. Claude Desktop registers for its local chat, not Cowork or claude.ai. Click a status badge for the app’s MCP settings. Configuration files are available as an additional action."),
			section("Web & mobile", "This utility configures this computer. A regular browser or mobile chat does not load its local adapter. Those surfaces need a separate remote integration."))
		basics.Open(0)
		popup := dialog.NewCustom("How to connect", "Close", container.NewVScroll(basics), w)
		popup.Resize(fyne.NewSize(620, 440))
		popup.Show()
	})
	connectionHelp.Importance = widget.LowImportance
	clientsCard := container.NewVBox(container.NewBorder(nil, nil, clientsTitle, connectionHelp), clients)
	viewDetails := widget.NewButton("Details", func() {
		detail := fullDetails
		if verified != nil {
			detail = verificationNote
		}
		content := connectionDetailsContent(status.Text, detail, verified, checkedAt)
		popup := dialog.NewCustom("Connection details", "Close", content, w)
		popup.Resize(fyne.NewSize(620, 500))
		popup.Show()
	})
	viewDetails.Importance = widget.LowImportance
	status.TextStyle.Bold = true
	feedback := panel(container.NewVBox(container.NewBorder(nil, nil, feedbackLamp, viewDetails, status), details, progress), 10)
	content := container.NewVBox(settingsCard, spacer(4), clientsCard)
	header := pad(container.NewBorder(nil, nil, nil, serverLamp, pageTitle), 12)
	footer := pad(container.NewVBox(changes, feedback, container.NewBorder(nil, nil, container.NewHBox(disconnect, remove), container.NewHBox(test, save))), 10)
	right := container.NewBorder(header, footer, nil, nil, container.NewVScroll(pad(container.NewVBox(content), 12)))

	brand := canvas.NewText("MCP Connect", ink)
	brand.TextSize = 18
	brand.TextStyle.Bold = true
	logo := canvas.NewImageFromResource(brandIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(24, 24))
	updates := newUpdateControls(a, w, store.Dir, func() bool { return busy }, setBusy)
	aboutButton := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		active := 0
		for _, c := range connections {
			if c.Enabled {
				active++
			}
		}
		title := widget.NewLabelWithStyle("MCP Connect", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		aboutLogo := canvas.NewImageFromResource(brandIcon)
		aboutLogo.FillMode = canvas.ImageFillContain
		aboutLogo.SetMinSize(fyne.NewSize(48, 48))
		identity := container.NewHBox(aboutLogo, container.NewVBox(title, widget.NewLabel("Version "+core.Version)))
		credit := widget.NewHyperlink("SeredaLabs", &url.URL{Scheme: "https", Host: "github.com", Path: "/SeredaLabs"})
		credit.TextStyle.Bold = true
		docs := widget.NewHyperlink("Documentation", &url.URL{Scheme: "https", Host: "github.com", Path: "/SeredaLabs/mcp-connect"})
		maintenance := widget.NewLabel(fmt.Sprintf("Saved connections with integrations enabled: %d.\n\nClose the app and existing client sessions before updating. Disconnect each connection before uninstalling. After moving the app or switching from a portable copy, use Save & Apply from its permanent location to repair adapter paths. Uninstalling preserves profiles and vault secrets; Delete in the app removes a saved connection and its secret.", active))
		maintenance.Wrapping = fyne.TextWrapWord
		content := container.NewVBox(identity, updates.aboutContent(), widget.NewSeparator(), container.NewHBox(widget.NewLabel("Made & designed by"), credit), docs,
			widget.NewAccordion(widget.NewAccordionItem("Moving, updating & removing", maintenance)))
		popup := dialog.NewCustom("About MCP Connect", "Close", container.NewVScroll(content), w)
		popup.Resize(fyne.NewSize(560, 480))
		popup.Show()
	})
	aboutButton.Importance = widget.LowImportance
	brandRow := container.NewBorder(nil, nil, container.NewHBox(logo, brand), aboutButton)
	sidebarTop := container.NewVBox(brandRow, spacer(10), widget.NewLabelWithStyle("CONNECTIONS", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	versionNote := canvas.NewText("MCP Connect · v"+core.Version, neutralTone)
	versionNote.TextSize = 11
	signature := canvas.NewText("Made & designed by", neutralTone)
	signature.TextSize = 11
	author := widget.NewHyperlink("SeredaLabs", &url.URL{Scheme: "https", Host: "github.com", Path: "/SeredaLabs"})
	author.TextStyle = fyne.TextStyle{Bold: true}
	// Match the hyperlink's built-in text inset without adding vertical space.
	creditLine := func(text fyne.CanvasObject) fyne.CanvasObject {
		return container.New(layout.NewCustomPaddedLayout(0, 0, a.Settings().Theme().Size(theme.SizeNameInnerPadding), a.Settings().Theme().Size(theme.SizeNameInnerPadding)), text)
	}
	credits := container.NewVBox(creditLine(versionNote), creditLine(signature), author)
	sidebarFooter := container.NewVBox(add, updates.sidebar, spacer(4), credits)
	sidebar := pad(container.NewBorder(sidebarTop, sidebarFooter, nil, nil, list), 12)

	shell := container.New(&shellLayout{}, sidebar, right)
	hintLayer := container.New(&hintLayerLayout{})
	for _, check := range []*hintCheck{claude, codex, desktop, chatgpt} {
		check.hint.layer = hintLayer
	}
	for _, button := range authMethod.buttons {
		button.hint.layer = hintLayer
	}
	w.SetContent(container.NewStack(shell, hintLayer))
	reload()
	clear()
	if len(connections) > 0 {
		list.Select(0)
	}

	w.SetCloseIntercept(func() {
		if busy {
			dialog.ShowInformation("Action in progress", "Wait for the current action to finish before closing.", w)
			return
		}
		confirmDiscard(func() {
			loading = true
			password.SetText("")
			w.SetCloseIntercept(nil)
			w.Close()
		})
	})
	w.SetOnClosed(func() {
		for _, check := range []*hintCheck{claude, codex, desktop, chatgpt} {
			check.hint.leave()
		}
		for _, button := range authMethod.buttons {
			button.hint.leave()
		}
	})
	if updates.auto.Checked {
		updates.checkNow()
	}
	return w
}

var ink = color.NRGBA{R: 26, G: 39, B: 62, A: 255}

type connectTheme struct{}

func (*connectTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.NRGBA{246, 248, 252, 255}
	case theme.ColorNameForeground:
		return ink
	case theme.ColorNamePrimary:
		return color.NRGBA{54, 96, 228, 255}
	case theme.ColorNameButton:
		return color.NRGBA{235, 240, 248, 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{248, 250, 253, 255}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{222, 229, 239, 255}
	case theme.ColorNameSelection:
		return color.NRGBA{220, 231, 255, 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{132, 144, 163, 255}
	}
	return theme.DefaultTheme().Color(n, theme.VariantLight)
}
func (*connectTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(n)
}
func (*connectTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (*connectTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func pad(o fyne.CanvasObject, amount float32) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(amount, amount, amount, amount), o)
}
func spacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}
func panel(o fyne.CanvasObject, padding float32) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.White)
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{229, 234, 243, 255}
	bg.StrokeWidth = 1
	return container.NewStack(bg, pad(o, padding))
}

type shellLayout struct{}

func (*shellLayout) MinSize(objects []fyne.CanvasObject) fyne.Size { return fyne.NewSize(960, 680) }
func (*shellLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(220, size.Height))
	objects[1].Move(fyne.NewPos(220, 0))
	objects[1].Resize(fyne.NewSize(size.Width-220, size.Height))
}
