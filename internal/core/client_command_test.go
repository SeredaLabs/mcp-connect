package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestNPMLauncherWithoutShell(t *testing.T) {
	if _, e := exec.LookPath("node"); e != nil {
		t.Skip("Synthetic npm launcher test requires Node")
	}
	dir := filepath.Join(t.TempDir(), "Developer & 100% Space")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"claude", "codex"} {
		launcher := filepath.Join(dir, name+".cmd")
		if e := os.WriteFile(launcher, []byte("@echo this launcher must not be executed\r\nexit /b 99\r\n"), 0600); e != nil {
			t.Fatal(e)
		}
		var entry string
		if name == "codex" {
			entry = filepath.Join(dir, "node_modules", "@openai", "codex", "bin", "codex.js")
		} else {
			entry = filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-code", "cli.js")
		}
		if e := os.MkdirAll(filepath.Dir(entry), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(entry, []byte("process.stdout.write(JSON.stringify(process.argv.slice(2)))"), 0600); e != nil {
			t.Fatal(e)
		}
		args := []string{"mcp", "add", "literal & argument", "%TEMP%", "$(should-not-run)", `C:\Developer With Spaces\Connect.exe`}
		bin, got, e := clientCommand(launcher, args, func() string { relative, _ := filepath.Rel(filepath.Join(dir, "node_modules"), entry); return relative }())
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, e := exec.CommandContext(ctx, bin, got...).CombinedOutput()
		contextErr := ctx.Err()
		cancel()
		if e != nil {
			t.Fatalf("Synthetic Node entrypoint failed: %v; context: %v; output: %s", e, contextErr, out)
		}
		var actual []string
		if e = json.Unmarshal(out, &actual); e != nil || !reflect.DeepEqual(actual, args) {
			t.Fatal("Arguments were expanded or split", e)
		}
	}
}
func TestBrokenNPMLauncherFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if _, _, e := clientCommand(filepath.Join(dir, "codex.cmd"), []string{"mcp", "list"}, filepath.FromSlash("@openai/codex/bin/codex.js")); e == nil {
		t.Fatal("Broken client installation must not fall through to a shell")
	}
}
