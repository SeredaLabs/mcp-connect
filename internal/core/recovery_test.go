package core

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryDoesNotReplayTool(t *testing.T) {
	var inits, calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&q)
		if q.Method == "tools/call" {
			calls.Add(1)
			w.WriteHeader(404)
			return
		}
		if len(q.ID) == 0 {
			w.WriteHeader(204)
			return
		}
		var result any
		if q.Method == "initialize" {
			inits.Add(1)
			w.Header().Set("Mcp-Session-Id", "recover-session")
			result = map[string]any{"protocolVersion": "2025-03-26", "serverInfo": map[string]string{"name": "recover", "version": "1"}, "capabilities": map[string]any{}}
		} else {
			result = map[string]any{"tools": []any{}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Bridge(ctx, s.URL, Credentials{Login: "user", Password: "secret"}, inR, outW) }()
	c, _ := (&mcp.IOTransport{Reader: outR, Writer: inW}).Connect(ctx)
	defer c.Close()
	send := func(id int, method string) {
		t.Helper()
		msg, _ := jsonrpc.DecodeMessage([]byte(fmtRequest(id, method, `{}`)))
		if e := c.Write(ctx, msg); e != nil {
			t.Fatal(e)
		}
	}
	read := func() *jsonrpc.Response {
		t.Helper()
		m, e := c.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		return m.(*jsonrpc.Response)
	}
	send(1, "initialize")
	if read().Error != nil {
		t.Fatal("init failed")
	}
	n, _ := jsonrpc.DecodeMessage([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	c.Write(ctx, n)
	send(2, "tools/call")
	if read().Error == nil {
		t.Fatal("broken call must fail")
	}
	deadline := time.Now().Add(3 * time.Second)
	for inits.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if inits.Load() != 2 {
		t.Fatal("session was not reinitialized")
	}
	send(3, "tools/list")
	if read().Error != nil {
		t.Fatal("subsequent call failed")
	}
	if calls.Load() != 1 {
		t.Fatal("dangerous tool replay", calls.Load())
	}
	inW.Close()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("EOF timeout")
	}
}
