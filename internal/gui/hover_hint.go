package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Hover hints are non-modal overlays and never change a control's selection.
type hoverHint struct {
	text     func() string
	timer    *time.Timer
	revision uint64
	popup    fyne.CanvasObject
	layer    *fyne.Container
}

// Unlike Canvas.Overlays, this visual layer does not intercept input to the
// underlying controls. Its children keep their measured size and position.
type hintLayerLayout struct{}

func (*hintLayerLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }
func (*hintLayerLayout) Layout([]fyne.CanvasObject, fyne.Size) {}

func (h *hoverHint) enter(owner fyne.CanvasObject) {
	h.leave()
	revision := h.revision
	h.timer = time.AfterFunc(450*time.Millisecond, func() {
		fyne.Do(func() {
			if h.revision == revision {
				h.show(owner)
			}
		})
	})
}

func (h *hoverHint) leave() {
	h.revision++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	if h.popup != nil {
		h.layer.Remove(h.popup)
		h.popup = nil
	}
}

func (h *hoverHint) show(owner fyne.CanvasObject) {
	if h.text == nil || h.popup != nil || h.layer == nil {
		return
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(owner)
	if c == nil {
		return
	}
	label := widget.NewLabel(h.text())
	label.Wrapping = fyne.TextWrapWord
	bg := canvas.NewRectangle(fyne.CurrentApp().Settings().Theme().Color(theme.ColorNameOverlayBackground, fyne.CurrentApp().Settings().ThemeVariant()))
	bg.CornerRadius = 8
	bg.StrokeColor = neutralTone
	bg.StrokeWidth = 1
	tip := container.NewStack(bg, pad(label, 10))
	width := float32(380)
	if width > c.Size().Width-16 {
		width = c.Size().Width - 16
	}
	tip.Resize(fyne.NewSize(width, 100))
	tip.Resize(fyne.NewSize(width, tip.MinSize().Height))
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(owner)
	pos.Y += owner.Size().Height + 6
	if pos.X+width > c.Size().Width-8 {
		pos.X = c.Size().Width - width - 8
	}
	if pos.Y+tip.Size().Height > c.Size().Height-8 {
		pos.Y = fyne.CurrentApp().Driver().AbsolutePositionForObject(owner).Y - tip.Size().Height - 6
	}
	if pos.Y < 8 {
		pos.Y = 8
	}
	tip.Move(pos)
	h.popup = tip
	h.layer.Add(tip)
}

type hintCheck struct {
	widget.Check
	hint hoverHint
}

func newHintCheck(text string, hint func() string) *hintCheck {
	c := &hintCheck{hint: hoverHint{text: hint}}
	c.Text = text
	c.ExtendBaseWidget(c)
	return c
}
func (c *hintCheck) MouseIn(e *desktop.MouseEvent) { c.Check.MouseIn(e); c.hint.enter(c) }
func (c *hintCheck) MouseOut()                     { c.Check.MouseOut(); c.hint.leave() }
func (c *hintCheck) Tapped(e *fyne.PointEvent)     { c.hint.leave(); c.Check.Tapped(e) }
func (c *hintCheck) Disable()                      { c.hint.leave(); c.Check.Disable() }

type hintButton struct {
	widget.Button
	hint hoverHint
}

func newHintButton(text string, icon fyne.Resource, tap func(), hint func() string) *hintButton {
	b := &hintButton{hint: hoverHint{text: hint}}
	b.Text, b.Icon, b.OnTapped = text, icon, tap
	b.ExtendBaseWidget(b)
	return b
}
func (b *hintButton) MouseIn(e *desktop.MouseEvent) { b.Button.MouseIn(e); b.hint.enter(b) }
func (b *hintButton) MouseOut()                     { b.Button.MouseOut(); b.hint.leave() }
func (b *hintButton) Tapped(e *fyne.PointEvent)     { b.hint.leave(); b.Button.Tapped(e) }
func (b *hintButton) Disable()                      { b.hint.leave(); b.Button.Disable() }
