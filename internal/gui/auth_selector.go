package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

// A mutually exclusive, equal-width switch with no popup or hidden choices.
type authSelector struct {
	widget.BaseWidget
	Selected  string
	OnChanged func(string)
	buttons   []*hintButton
	options   []string
}

func newAuthSelector(options []string) *authSelector {
	s := &authSelector{options: options}
	for _, option := range options {
		value := option
		var icon fyne.Resource
		switch value {
		case "Login & password":
			icon = loginPasswordIcon
		case "Bearer token":
			icon = bearerTokenIcon
		case "API key":
			icon = apiKeyIcon
		}
		button := newHintButton(value, icon, func() { s.SetSelected(value) }, func() string {
			switch value {
			case "Bearer token":
				return authenticationHint("bearer")
			case "API key":
				return authenticationHint("api-key")
			default:
				return authenticationHint("basic")
			}
		})
		button.Importance = widget.LowImportance
		s.buttons = append(s.buttons, button)
	}
	s.ExtendBaseWidget(s)
	return s
}
func (s *authSelector) SetSelected(value string) {
	if s.Selected == value {
		return
	}
	found := false
	for _, option := range s.options {
		if value == option {
			found = true
		}
	}
	if !found {
		return
	}
	s.Selected = value
	for n, button := range s.buttons {
		button.Importance = widget.LowImportance
		if s.options[n] == value {
			button.Importance = widget.HighImportance
		}
		button.Refresh()
	}
	if s.OnChanged != nil {
		s.OnChanged(value)
	}
}
func (s *authSelector) Disable() {
	for _, b := range s.buttons {
		b.Disable()
	}
}
func (s *authSelector) Enable() {
	for _, b := range s.buttons {
		b.Enable()
	}
}
func (s *authSelector) CreateRenderer() fyne.WidgetRenderer {
	objects := make([]fyne.CanvasObject, len(s.buttons))
	for n, button := range s.buttons {
		objects[n] = button
	}
	track := canvas.NewRectangle(color.NRGBA{R: 241, G: 245, B: 251, A: 255})
	track.CornerRadius = 10
	return widget.NewSimpleRenderer(container.NewStack(track, pad(container.NewGridWithColumns(len(objects), objects...), 3)))
}
