package gui

import "github.com/SeredaLabs/mcp-connect/internal/core"

func clientHoverHint(client string) string {
	switch client {
	case "codex":
		return "Direct MCP for the local Codex app, CLI and IDE. Restart the client after applying. If the ChatGPT plugin also appears in Codex, enable only one entry in that session."
	case "chatgpt":
		return "Plugin for ChatGPT Desktop chats with local execution. Restart ChatGPT and enable the plugin in the chat. It may also appear in Codex, even when Codex is unchecked here."
	case "claude":
		return "MCP for Claude Code across this user's projects. Start a new session after applying; use /mcp to check it. Claude Desktop chat is configured separately."
	case "claude-desktop":
		return "MCP for local Claude Desktop chat. Fully quit and reopen Claude after applying. This registration does not connect Cowork, claude.ai or mobile chats."
	}
	return ""
}

func authenticationHint(mode string) string {
	switch mode {
	case core.AuthBearer:
		return "Use a token issued by this MCP server. Paste the token only, without the Bearer prefix."
	case core.AuthAPIKey:
		return "Use the API key and header name required by this MCP server."
	default:
		return "Use the MCP server's username and password, not your ChatGPT or Claude account login."
	}
}
