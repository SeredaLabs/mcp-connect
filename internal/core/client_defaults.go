package core

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
	chatgpt := NewChatGPTAdapter(i, codex)
	codex.shared = chatgpt.Read
	adapters := []ClientAdapter{codex, chatgpt, NewClaudeAdapter(i, ""), NewClaudeDesktopAdapter(i, "")}
	var bindings []ClientBinding
	for n, a := range adapters {
		d := clientDefinitions[n]
		bindings = append(bindings, ClientBinding{Adapter: a, Name: d.name, Selected: d.selected})
	}
	registry, err := NewClientRegistry(bindings...)
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
