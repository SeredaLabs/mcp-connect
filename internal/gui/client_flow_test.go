package gui

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/SeredaLabs/mcp-connect/internal/core"
	"github.com/SeredaLabs/mcp-connect/internal/updater"
)

type flowDriver struct {
	fyne.Driver
	done chan struct{}
}

func (d *flowDriver) DoFromGoroutine(f func(), _ bool) { f(); d.done <- struct{}{} }

type flowApp struct {
	fyne.App
	driver *flowDriver
}

func (a *flowApp) Driver() fyne.Driver { return a.driver }

func walkFlow(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(o)
	switch v := o.(type) {
	case *fyne.Container:
		for _, child := range v.Objects {
			walkFlow(child, visit)
		}
	case *container.Scroll:
		walkFlow(v.Content, visit)
	case *hintCheck:
		visit(&v.Check)
	}
}

func TestOfflineApplyRetryAndUnsavedChanges(t *testing.T) {
	base := test.NewApp()
	a := &flowApp{App: base, driver: &flowDriver{Driver: base.Driver(), done: make(chan struct{}, 10)}}
	fyne.SetCurrentApp(a)
	t.Cleanup(func() { fyne.SetCurrentApp(base) })
	dir := t.TempDir()
	s := &core.Store{Dir: dir} // nil vault: any unexpected credential access panics
	c := core.Connection{ID: core.NewID(), Name: "Demo workspace", URL: "https://offline.example.invalid/mcp", Codex: true, Enabled: true}
	if err := s.Write([]core.Connection{c}); err != nil {
		t.Fatal(err)
	}
	if err := updater.SetCheckOnLaunch(dir, false); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "adapter")
	path := filepath.Join(dir, "codex.toml")
	if err := core.AtomicWrite(path, []byte("[mcp_servers."+c.ID+"]\ncommand = "+strconv.Quote(exe)+"\nargs = [\"serve\",\""+c.ID+"\"]\n")); err != nil {
		t.Fatal(err)
	}
	i := &core.Integrator{Home: dir, Executable: exe, Find: func(string) string { return "fixture" }}
	codexAdapter := core.NewCodexAdapter(i, path)
	i.Clients, _ = core.NewClientRegistry(
		core.ClientBinding{Adapter: codexAdapter, Name: "Codex", Selected: func(c core.Connection) bool { return c.Codex }},
		core.ClientBinding{Adapter: core.NewChatGPTAdapter(i, path), Name: "ChatGPT Desktop", Selected: func(c core.Connection) bool { return c.ChatGPT }},
		core.ClientBinding{Adapter: core.NewClaudeAdapter(i, filepath.Join(dir, "claude.json")), Name: "Claude Code", Selected: func(c core.Connection) bool { return c.Claude }},
		core.ClientBinding{Adapter: core.NewClaudeDesktopAdapter(i, filepath.Join(dir, "desktop.json")), Name: "Claude Desktop", Selected: func(c core.Connection) bool { return c.ClaudeDesktop }},
	)
	failRemoval := true
	i.Run = func(context.Context, string, ...string) error {
		if failRemoval {
			return errors.New("fixture unavailable")
		}
		return core.AtomicWrite(path, []byte(""))
	}
	w := newWindow(a, s, i)
	t.Cleanup(func() { w.SetCloseIntercept(nil); w.Close() })
	w.Resize(fyne.NewSize(1020, 800))
	w.Show()
	var codex *widget.Check
	var save, retry *widget.Button
	walkFlow(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Check); ok && v.Text == "Codex" {
			codex = v
		}
		if v, ok := o.(*widget.Button); ok && v.Text == "Save & Apply" {
			save = v
		}
	})
	if codex == nil || save == nil {
		t.Fatal("missing primary controls")
	}
	codex.SetChecked(false)
	stored, _ := s.Find(c.ID)
	if !stored.Codex || !stored.Enabled {
		t.Fatal("checkbox mutated persisted state before Apply")
	}
	test.Tap(save)
	select {
	case <-a.driver.done:
	case <-time.After(5 * time.Second):
		t.Fatal("offline Apply did not finish")
	}
	stored, _ = s.Find(c.ID)
	if stored.Enabled || stored.Codex {
		t.Fatal("all-unchecked profile not disabled", stored)
	}
	walkFlow(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Button); ok && v.Text == "Retry" && v.Visible() {
			retry = v
		}
	})
	if retry == nil {
		t.Fatal("failed cleanup has no Retry")
	}
	captureFlow(t, w, "cleanup-retry")

	failRemoval = false
	test.Tap(retry)
	select {
	case <-a.driver.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Retry did not finish")
	}
	if retry.Visible() {
		t.Fatal("successful Retry still visible")
	}
	if _, exists, err := i.Read("codex", c.ID); err != nil || exists {
		t.Fatal("Retry did not clean up registration", err)
	}
	var desktop *widget.Check
	walkFlow(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Check); ok && v.Text == "Claude Desktop" {
			desktop = v
		}
	})
	if desktop == nil {
		t.Fatal("missing desktop control")
	}
	desktop.SetChecked(true)
	test.Tap(save)
	select {
	case <-a.driver.done:
	case <-time.After(5 * time.Second):
		t.Fatal("offline client addition did not finish")
	}
	stored, _ = s.Find(c.ID)
	if !stored.Enabled || !stored.ClaudeDesktop {
		t.Fatal("offline reconnect not saved", stored)
	}
	if _, exists, err := i.Read("claude-desktop", c.ID); err != nil || !exists {
		t.Fatal("offline client addition failed", err)
	}
	captureFlow(t, w, "applied")
	// Explain the actual unsaved combination, without installing either mode.
	codex.SetChecked(true)
	var chatgpt *widget.Check
	var explain *widget.Button
	walkFlow(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Check); ok && v.Text == "ChatGPT Desktop" {
			chatgpt = v
		}
		if v, ok := o.(*widget.Button); ok && v.Text == "How to connect" {
			explain = v
		}
	})
	if chatgpt == nil || explain == nil {
		t.Fatal("missing contextual help")
	}
	chatgpt.SetChecked(true)
	captureFlow(t, w, "selected-modes")
	var hovered *hintCheck
	walkFlow(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*hintCheck); ok && v.Text == "ChatGPT Desktop" {
			hovered = v
		}
	})
	if hovered == nil {
		t.Fatal("missing hover target")
	}
	hovered.hint.show(hovered)
	if hovered.hint.popup == nil || len(w.Canvas().Overlays().List()) != 0 {
		t.Fatal("hover hint missing or intercepting input")
	}
	captureFlow(t, w, "hover-hint")
	position := a.Driver().AbsolutePositionForObject(hovered)
	test.TapCanvas(w.Canvas(), position.Add(fyne.NewPos(10, hovered.Size().Height/2)))
	if hovered.Checked {
		t.Fatal("click did not toggle while hint was visible")
	}
	hovered.SetChecked(true)
	hovered.MouseOut()
	if hovered.hint.popup != nil {
		t.Fatal("hover hint remained after mouse exit")
	}
	if !hovered.Checked {
		t.Fatal("hover changed client selection")
	}
	test.Tap(explain)
	if len(w.Canvas().Overlays().List()) == 0 {
		t.Fatal("selection explanation did not open")
	}
	captureFlow(t, w, "selection-explained")

}

func captureFlow(t *testing.T, w fyne.Window, name string) {
	t.Helper()
	root := os.Getenv("MCP_CONNECT_UI_CAPTURE")
	if root == "" {
		return
	}
	path := filepath.Join(root, name+".png")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	// A test driver has no native window resize loop; explicitly refresh layout.
	w.Content().Refresh()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, w.Canvas().Capture())
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
}
