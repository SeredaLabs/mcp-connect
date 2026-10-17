package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Preserve actionable, locally generated launcher errors. Do not expose arbitrary
// CLI stderr or underlying errors, which can contain paths or configuration data.
type clientLauncherError string

func (e clientLauncherError) Error() string { return string(e) }

func clientCommandFailure(ctx context.Context, client string, err error) error {
	name := ClientName(client)
	var launcher clientLauncherError
	if errors.As(err, &launcher) {
		return fmt.Errorf("%s: %s", name, launcher)
	}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s setup timed out; close busy client processes and retry: %w", name, context.DeadlineExceeded)
	case errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled):
		return fmt.Errorf("%s setup was canceled: %w", name, context.Canceled)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("%s setup was denied; check executable and configuration permissions: %w", name, os.ErrPermission)
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%s setup executable is unavailable; repair the client installation and retry: %w", name, exec.ErrNotFound)
	default:
		return fmt.Errorf("%s setup command failed; check the client configuration and version, then update MCP Connect and the client if needed", name)
	}
}
