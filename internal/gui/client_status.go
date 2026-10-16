package gui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/SeredaLabs/mcp-connect/internal/core"
)

func clientStatus(s core.ClientState, chosen, pending, failed bool) (string, color.NRGBA) {
	if pending {
		if chosen {
			return "Will be added", amberTone
		}
		return "Will be removed", amberTone
	}
	if failed {
		return "Needs attention", redTone
	}
	if s.Err != nil {
		return "Check settings", redTone
	}
	if s.Registered {
		if !chosen {
			return "Registered · will remove", amberTone
		}
		if s.Disabled {
			return "Disabled in client", amberTone
		}
		if s.Repair {
			return "Repair needed", amberTone
		}
		return "Registered", blueTone
	}
	if s.Shared {
		return "Via OpenAI plugin", blueTone
	}
	if chosen {
		return "Not registered", amberTone
	}
	return "Not registered", neutralTone
}

func integrationSummary(results map[string]core.ClientResult) (string, int) {
	var lines []string
	failed := 0
	for _, client := range core.ClientIDs {
		r, ok := results[client]
		if !ok {
			continue
		}
		message := "Entry removed / not registered"
		if r.On {
			message = "Registered · restart the client"
			if client == "chatgpt" {
				message = "Plugin installed · enable in a local-execution chat"
			}
		}
		if r.Err != nil {
			failed++
			message = r.Err.Error() + " · use Retry"
		}
		lines = append(lines, core.ClientName(client)+": "+message)
	}
	lines = append(lines, "Registration does not confirm that the client has loaded its tools.")
	if codex, ok := results["codex"]; ok && codex.On {
		if plugin, ok := results["chatgpt"]; ok && plugin.On {
			lines = append(lines, "OpenAI: the plugin may also appear in Codex. If both entries appear, enable only one in that Codex session.")
		}
	}
	if failed > 0 {
		lines = append([]string{fmt.Sprintf("%d client action(s) need attention. Successful actions are retained.", failed)}, lines...)
	}
	return strings.Join(lines, "\n"), failed
}
