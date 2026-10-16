package core

import (
	"context"
	"errors"
	"strings"
)

// ClientIDs is the order used for applying and reporting independent integrations.
var ClientIDs = []string{"codex", "chatgpt", "claude", "claude-desktop"}

func ClientName(id string) string {
	switch id {
	case "codex":
		return "Codex"
	case "chatgpt":
		return "ChatGPT Desktop"
	case "claude":
		return "Claude Code"
	case "claude-desktop":
		return "Claude Desktop"
	}
	return id
}

func (c Connection) WantsClient(id string) bool {
	switch id {
	case "codex":
		return c.Codex
	case "chatgpt":
		return c.ChatGPT
	case "claude":
		return c.Claude
	case "claude-desktop":
		return c.ClaudeDesktop
	}
	return false
}

type ClientState struct {
	Registered bool
	Shared     bool
	Disabled   bool
	Repair     bool
	Err        error
}

// Inspect reports registration, not a live connection or tool availability.
func (i *Integrator) Inspect(client, id string) ClientState {
	r, exists, err := i.Read(client, id)
	s := ClientState{Err: err}
	if err != nil {
		return s
	}
	if client == "codex" {
		_, shared, sharedErr := i.readChatGPT(id)
		if sharedErr == nil {
			s.Shared = shared
		}
	}
	if !exists {
		return s
	}
	if !owned(r, id) {
		s.Err = errors.New("Name conflict: this entry belongs to another integration")
		return s
	}
	s.Registered = true
	s.Disabled = r.Enabled != nil && !*r.Enabled
	s.Repair = !samePath(r.Command, i.Executable)
	return s
}

type ClientResult struct {
	Client string
	On     bool
	Err    error
}

type ApplyReport []ClientResult

func (r ApplyReport) FailedClients() []string {
	var failed []string
	for _, result := range r {
		if result.Err != nil {
			failed = append(failed, result.Client)
		}
	}
	return failed
}

func (r ApplyReport) Err() error {
	var messages []string
	for _, result := range r {
		if result.Err != nil {
			messages = append(messages, ClientName(result.Client)+": "+result.Err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(messages, "\n"))
}

// Apply retries only the requested clients when a subset is supplied. A plugin
// failure never removes or prevents an independent direct Codex registration.
func (i *Integrator) Apply(ctx context.Context, c Connection, clients ...string) ApplyReport {
	if len(clients) == 0 {
		clients = ClientIDs
	}
	copyIntegrator := *i
	copyIntegrator.DisplayName = c.Name
	var report ApplyReport
	for _, client := range clients {
		if client != "codex" && client != "chatgpt" && client != "claude" && client != "claude-desktop" {
			report = append(report, ClientResult{Client: client, Err: errors.New("Unknown client")})
			continue
		}
		on := c.Enabled && c.WantsClient(client)
		report = append(report, ClientResult{Client: client, On: on, Err: copyIntegrator.Set(ctx, client, c.ID, on)})
	}
	return report
}
