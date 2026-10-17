package gui

import (
	"strings"
	"testing"
)

func TestHoverHintExplainsSharedOpenAIAndClientScope(t *testing.T) {
	for client, phrase := range map[string]string{
		"codex":          "enable only one entry",
		"chatgpt":        "even when Codex is unchecked",
		"claude":         "across this user's projects",
		"claude-desktop": "does not connect Cowork",
	} {
		if text := clientHoverHint(client); !strings.Contains(text, phrase) {
			t.Fatal(client, text)
		}
	}
}
