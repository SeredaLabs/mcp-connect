package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestClientCommandDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		message string
		kind    error
	}{
		{"timeout", context.DeadlineExceeded, "timed out", context.DeadlineExceeded},
		{"canceled", context.Canceled, "canceled", context.Canceled},
		{"permission", &os.PathError{Op: "open", Path: "private-sentinel-path", Err: os.ErrPermission}, "permissions", os.ErrPermission},
		{"missing", &os.PathError{Op: "open", Path: "private-sentinel-path", Err: os.ErrNotExist}, "repair the client", exec.ErrNotFound},
		{"unknown-command", errors.New("unsupported --flag; private-sentinel-path"), "update MCP Connect", nil},
		{"launcher", clientLauncherError("Repair the Node installation"), "Repair the Node installation", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := clientCommandFailure(context.Background(), "claude", tc.err)
			if !strings.Contains(err.Error(), tc.message) || !strings.Contains(err.Error(), "Claude Code") || strings.Contains(err.Error(), "private-sentinel-path") {
				t.Fatal("unsafe or unhelpful diagnostic", err)
			}
			if tc.kind != nil && !errors.Is(err, tc.kind) {
				t.Fatal("safe failure classification lost", err)
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	// exec.CommandContext may report an ExitError after being killed rather than
	// returning context.DeadlineExceeded directly. Prefer the operation's context.
	if err := clientCommandFailure(ctx, "codex", errors.New("signal: killed")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
