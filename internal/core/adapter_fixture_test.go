package core

import "context"

// Integration fixtures construct adapters explicitly, keeping test paths out of
// the coordinator. Tests can replace execution seams after construction.
type integrationFixture struct {
	DisplayName, Home, Executable, ClaudePath, CodexPath, ClaudeDesktopPath string
	Run                                                                     func(context.Context, string, ...string) error
	Output                                                                  func(context.Context, string, ...string) ([]byte, error)
	Find                                                                    func(string) string
}

func fixtureIntegrator(f integrationFixture) *Integrator {
	i := &Integrator{DisplayName: f.DisplayName, Home: f.Home, Executable: f.Executable, Run: f.Run, Output: f.Output, Find: f.Find}
	codex := NewCodexAdapter(i, f.CodexPath)
	chat := NewChatGPTAdapter(i, f.CodexPath)
	codex.shared = chat.Read
	i.Clients, _ = NewClientRegistry(
		ClientBinding{codex, "Codex", func(c Connection) bool { return c.Codex }},
		ClientBinding{chat, "ChatGPT Desktop", func(c Connection) bool { return c.ChatGPT }},
		ClientBinding{NewClaudeAdapter(i, f.ClaudePath), "Claude Code", func(c Connection) bool { return c.Claude }},
		ClientBinding{NewClaudeDesktopAdapter(i, f.ClaudeDesktopPath), "Claude Desktop", func(c Connection) bool { return c.ClaudeDesktop }},
	)
	return i
}
func fixturePath(i *Integrator, client string) string {
	a, _ := i.adapter(client)
	switch a := a.(type) {
	case *CodexAdapter:
		return a.configPath
	case *ClaudeAdapter:
		return a.configPath
	case *ClaudeDesktopAdapter:
		return a.configPath
	}
	return ""
}
func fixtureChatGPT(i *Integrator) *ChatGPTAdapter {
	a, _ := i.adapter("chatgpt")
	return a.(*ChatGPTAdapter)
}
