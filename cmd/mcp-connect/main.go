package main

import (
	"context"
	"fmt"
	"github.com/SeredaLabs/mcp-connect/internal/core"
	"github.com/SeredaLabs/mcp-connect/internal/gui"
	"github.com/SeredaLabs/mcp-connect/internal/updater"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	store, e := core.DefaultStore()
	if e != nil {
		return e
	}
	args := os.Args[1:]
	if len(args) == 0 {
		prepareDesktop()
		return gui.Run(store)
	}
	switch args[0] {
	case "version", "--version":
		fmt.Println(core.Version)
		return nil
	case "update":
		return runUpdate(args[1:])
	case "list":
		cs, e := store.Load()
		if e != nil {
			return e
		}
		for _, c := range cs {
			fmt.Printf("%s\t%s\t%s\tenabled=%t\n", c.ID, c.Name, c.URL, c.Enabled)
		}
		return nil
	case "serve", "test":
		if len(args) != 2 {
			return fmt.Errorf("Usage: mcp-connect %s <connection-id>", args[0])
		}
		c, e := store.Find(args[1])
		if e != nil {
			return e
		}
		if args[0] == "serve" && !c.Enabled {
			return fmt.Errorf("Connection disabled; reconnect in the app")
		}
		cred, e := store.Vault.Get(c.ID)
		if e != nil {
			return e
		}
		if cred.Endpoint == "" {
			return fmt.Errorf("Legacy credentials are not bound to a server; enter them again and use Save & Connect")
		}
		if e := core.ValidateCredentials(c, cred); e != nil {
			return e
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if args[0] == "serve" {
			return core.Bridge(ctx, c.URL, cred, os.Stdin, os.Stdout)
		}
		ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
		defer timeout()
		r, e := core.Probe(ctx, c.URL, cred)
		if e != nil {
			return e
		}
		fmt.Printf("Authorization OK · MCP %s · %s · %d tools\n", r.Protocol, r.Server, len(r.Tools))
		for _, tool := range r.Tools {
			fmt.Println(tool)
		}
		return nil
	default:
		return fmt.Errorf("Usage: mcp-connect [serve <id>|test <id>|list|version|update [--check|--download]]")
	}
}

func runUpdate(args []string) error {
	mode := ""
	if len(args) > 1 || (len(args) == 1 && args[0] != "--check" && args[0] != "--download") {
		return fmt.Errorf("Usage: mcp-connect update [--check|--download]")
	}
	if len(args) == 1 {
		mode = args[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	fmt.Println("Checking GitHub releases…")
	u, e := updater.Check(ctx, core.Version)
	if e != nil {
		return e
	}
	if u == nil {
		fmt.Println("MCP Connect " + core.Version + " is up to date.")
		return nil
	}
	fmt.Printf("Update available: %s → %s\n", core.Version, u.Version)
	if mode == "--check" {
		return nil
	}
	if mode != "--download" && !updater.Installed() {
		return fmt.Errorf("Portable copy: download a portable update from https://seredalabs.github.io/mcp-connect/ and replace it at the same location; use --download only if you want an installer")
	}
	fmt.Println("Downloading and verifying the installer…")
	path, e := updater.Download(ctx, u)
	if e != nil {
		return e
	}
	fmt.Println("Verified installer: " + path)
	if mode == "--download" {
		return nil
	}
	fmt.Println("Close MCP Connect and active AI-client adapter sessions. Complete the standard installer, then restart your clients. Profiles and credentials are preserved.")
	return updater.OpenInstaller(path)
}
