package core

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSDKClientThroughStdioBridge(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "SDK", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "Echo"}, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})
	s := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Bridge(ctx, s.URL, Credentials{Login: "user", Password: "secret"}, inR, outW) }()
	changed := make(chan struct{}, 1)
	client := mcp.NewClient(&mcp.Implementation{Name: "SDK client", Version: "1"}, &mcp.ClientOptions{ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
		select {
		case changed <- struct{}{}:
		default:
		}
	}})
	session, e := client.Connect(ctx, &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	list, e := session.ListTools(ctx, nil)
	if e != nil || len(list.Tools) != 1 {
		t.Fatal("list", e)
	}
	result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{}})
	if e != nil || result.IsError {
		t.Fatal("call", e)
	}
	mcp.AddTool(server, &mcp.Tool{Name: "another", Description: "Changes list"}, func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{}, nil, nil
	})
	select {
	case <-changed:
	case <-ctx.Done():
		t.Fatal("Standalone server notification did not reach the stdio client")
	}
	session.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Bridge did not exit")
	}
}
func TestProbeTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			w.WriteHeader(503)
		}
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e := probe(ctx, s.URL, Credentials{Login: "user", Password: "secret"}); e == nil {
		t.Fatal("timeout expected")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout not enforced")
	}
}
func TestMalformedStdio(t *testing.T) {
	s, _ := mockMCP(t)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, w := io.Pipe()
	outR, outW := io.Pipe()
	defer outR.Close()
	done := make(chan error, 1)
	go func() { done <- Bridge(ctx, s.URL, Credentials{Login: "user", Password: "sentinel-secret"}, r, outW) }()
	io.WriteString(w, "this is not JSON\n")
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("Malformed JSON must fail")
		}
	case <-ctx.Done():
		t.Fatal("adapter did not reject malformed stdin")
	}
}
