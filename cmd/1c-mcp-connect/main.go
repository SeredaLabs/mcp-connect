package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"onecmcp/connect/internal/core"
	"onecmcp/connect/internal/gui"
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
		return fmt.Errorf("Usage: mcp-connect [serve <id>|test <id>|list|version]")
	}
}
