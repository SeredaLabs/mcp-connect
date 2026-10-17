package core

import "fmt"

// Explicit composition is the only place that binds adapters to stored profile
// flags and UI names. Adding a client does not change the coordinator.
const chatGPTClientID = "chatgpt"

type clientDefinition struct {
	id, name string
	selected func(Connection) bool
}

var clientDefinitions = []clientDefinition{
	{"codex", "Codex", func(c Connection) bool { return c.Codex }},
	{chatGPTClientID, "ChatGPT Desktop", func(c Connection) bool { return c.ChatGPT }},
	{"claude", "Claude Code", func(c Connection) bool { return c.Claude }},
	{"claude-desktop", "Claude Desktop", func(c Connection) bool { return c.ClaudeDesktop }},
}
var ClientIDs = func() []string {
	var ids []string
	for _, d := range clientDefinitions {
		ids = append(ids, d.id)
	}
	return ids
}()

func ClientName(id string) string {
	for _, d := range clientDefinitions {
		if d.id == id {
			return d.name
		}
	}
	return id
}
func (c Connection) WantsClient(id string) bool {
	for _, d := range clientDefinitions {
		if d.id == id {
			return d.selected(c)
		}
	}
	return false
}

func defaultClients(i *Integrator) *ClientRegistry {
	codex := NewCodexAdapter(i, "")
	chatgpt := NewChatGPTAdapter(i, "")
	codex.shared = chatgpt.Read
	adapters := []ClientAdapter{codex, chatgpt, NewClaudeAdapter(i, ""), NewClaudeDesktopAdapter(i, "")}
	registry, err := bindClientAdapters(clientDefinitions, adapters)
	if err != nil {
		panic(err)
	} // static programmer error
	return registry
}

// FindClient remains a discovery facade for existing GUI callers.
func FindClient(id string) string {
	i := &Integrator{}
	a, err := defaultClients(i).Lookup(id)
	if err != nil {
		return ""
	}
	return a.Adapter.Detect()
}

// Definitions determine reporting order. IDs, never parallel slice positions,
// bind display names and persisted selections to concrete adapters.
func bindClientAdapters(definitions []clientDefinition, adapters []ClientAdapter) (*ClientRegistry, error) {
	byID := make(map[string]ClientAdapter, len(adapters))
	for _, a := range adapters {
		if a == nil || a.ID() == "" {
			return nil, fmt.Errorf("Invalid client adapter")
		}
		if _, exists := byID[a.ID()]; exists {
			return nil, fmt.Errorf("Duplicate client adapter: %s", a.ID())
		}
		byID[a.ID()] = a
	}
	seen := make(map[string]bool, len(definitions))
	var bindings []ClientBinding
	for _, d := range definitions {
		if d.id == "" || d.name == "" || d.selected == nil {
			return nil, fmt.Errorf("Invalid client definition")
		}
		if seen[d.id] {
			return nil, fmt.Errorf("Duplicate client definition: %s", d.id)
		}
		seen[d.id] = true
		a, ok := byID[d.id]
		if !ok {
			return nil, fmt.Errorf("Missing client adapter: %s", d.id)
		}
		bindings = append(bindings, ClientBinding{Adapter: a, Name: d.name, Selected: d.selected})
		delete(byID, d.id)
	}
	if len(byID) != 0 {
		return nil, fmt.Errorf("Client adapter has no definition")
	}
	return NewClientRegistry(bindings...)
}
