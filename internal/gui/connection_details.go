package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/SeredaLabs/mcp-connect/internal/core"
)

func connectionDetailsContent(status, detail string, result *core.ProbeResult, checkedAt time.Time) fyne.CanvasObject {
	wrapped := func(text string) *widget.Label {
		label := widget.NewLabel(text)
		label.Wrapping = fyne.TextWrapWord
		return label
	}
	if result == nil {
		heading := wrapped(status)
		heading.TextStyle.Bold = true
		return container.NewVScroll(container.NewVBox(heading, widget.NewSeparator(), wrapped(detail)))
	}
	field := func(title, value string) fyne.CanvasObject {
		label := canvas.NewText(title, neutralTone)
		label.TextSize = 12
		valueLabel := wrapped(value)
		valueLabel.TextStyle.Bold = true
		return container.NewVBox(label, valueLabel)
	}
	server := result.Server
	if server == "" {
		server = "Not provided by server"
	}
	summary := panel(container.NewVBox(
		container.NewBorder(nil, nil, nil, newIndicator("Authorized", greenTone), field("Server", server)),
		container.NewGridWithColumns(2, field("MCP protocol", result.Protocol), field("Available tools", fmt.Sprint(len(result.Tools)))),
	), 10)
	heading := widget.NewLabel("Available tools")
	heading.TextStyle.Bold = true
	tools := container.NewGridWithColumns(2)
	for _, name := range result.Tools {
		label := wrapped(name)
		label.TextStyle.Monospace = true
		tools.Add(container.NewBorder(nil, nil, widget.NewIcon(theme.DocumentIcon()), nil, label))
	}
	if len(result.Tools) == 0 {
		tools.Add(wrapped("No tools advertised by this server."))
	}
	body := container.NewVBox(summary, spacer(4), heading, panel(tools, 6))
	if detail != "" {
		body.Add(wrapped(detail))
	}
	footer := container.NewVBox(widget.NewSeparator(), wrapped("Last checked · "+checkedAt.Format("02 Jan 2006, 15:04")))
	note := canvas.NewText("Snapshot of the last check · no live monitoring", neutralTone)
	note.TextSize = 12
	footer.Add(note)
	return container.NewBorder(nil, footer, nil, nil, container.NewVScroll(body))
}
