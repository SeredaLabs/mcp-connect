package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

var (
	neutralTone = color.NRGBA{R: 126, G: 140, B: 161, A: 255}
	blueTone    = color.NRGBA{R: 54, G: 96, B: 228, A: 255}
	greenTone   = color.NRGBA{R: 20, G: 148, B: 102, A: 255}
	amberTone   = color.NRGBA{R: 183, G: 121, B: 20, A: 255}
	redTone     = color.NRGBA{R: 201, G: 62, B: 81, A: 255}
)

// A visible label always accompanies the color, including for color-blind users.
type indicator struct {
	widget.BaseWidget
	label      *widget.Label
	dot        *canvas.Circle
	background *canvas.Rectangle
	accessory  fyne.CanvasObject
}

func newIndicator(text string, tone color.NRGBA) *indicator {
	i := &indicator{label: widget.NewLabel(text), dot: canvas.NewCircle(tone), background: canvas.NewRectangle(color.White)}
	i.label.Wrapping = fyne.TextWrapOff
	i.background.CornerRadius = 8
	i.ExtendBaseWidget(i)
	i.Set(text, tone)
	return i
}
func (i *indicator) Set(text string, tone color.NRGBA) {
	i.label.SetText(text)
	i.dot.FillColor = tone
	i.dot.Refresh()
	// Pale background keeps the signal readable without tinting the whole card.
	i.background.FillColor = color.NRGBA{R: uint8(239 + int(tone.R)/16), G: uint8(239 + int(tone.G)/16), B: uint8(239 + int(tone.B)/16), A: 255}
	i.background.Refresh()
	i.Refresh()
}
func (i *indicator) CreateRenderer() fyne.WidgetRenderer {
	dot := container.NewCenter(container.NewGridWrap(fyne.NewSize(9, 9), i.dot))
	content := container.NewBorder(nil, nil, dot, i.accessory, i.label)
	return widget.NewSimpleRenderer(container.NewStack(i.background, pad(content, 5)))
}

// Only client indicators are actions; server and feedback lights stay passive.
type indicatorAction struct {
	widget.BaseWidget
	indicator *indicator
	onTapped  func()
	outline   *canvas.Rectangle
	hovered   bool
	focused   bool
}

func newIndicatorAction(i *indicator, action func()) *indicatorAction {
	control := &indicatorAction{indicator: i, onTapped: action, outline: canvas.NewRectangle(color.Transparent)}
	control.outline.CornerRadius = 8
	control.outline.StrokeWidth = 1
	control.ExtendBaseWidget(control)
	arrow := canvas.NewText("›", neutralTone)
	arrow.TextSize = 16
	i.accessory = container.NewCenter(arrow)
	return control
}
func (i *indicatorAction) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(i.indicator, i.outline))
}
func (i *indicatorAction) Tapped(*fyne.PointEvent)        { i.onTapped() }
func (i *indicatorAction) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (i *indicatorAction) MouseIn(*desktop.MouseEvent)    { i.hovered = true; i.refreshOutline() }
func (i *indicatorAction) MouseMoved(*desktop.MouseEvent) {}
func (i *indicatorAction) MouseOut()                      { i.hovered = false; i.refreshOutline() }
func (i *indicatorAction) FocusGained()                   { i.focused = true; i.refreshOutline() }
func (i *indicatorAction) FocusLost()                     { i.focused = false; i.refreshOutline() }
func (i *indicatorAction) TypedRune(rune)                 {}
func (i *indicatorAction) TypedKey(event *fyne.KeyEvent) {
	if event.Name == fyne.KeyReturn || event.Name == fyne.KeySpace {
		i.onTapped()
	}
}
func (i *indicatorAction) refreshOutline() {
	i.outline.StrokeColor = color.Transparent
	if i.hovered {
		i.outline.StrokeColor = color.NRGBA{R: 54, G: 96, B: 228, A: 64}
	}
	if i.focused {
		i.outline.StrokeColor = blueTone
	}
	i.outline.Refresh()
}
