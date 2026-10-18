package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryVault struct {
	items map[string]Credentials
	fail  bool
}

func (v *memoryVault) Get(id string) (Credentials, error) {
	if v.fail {
		return Credentials{}, errors.New("Locked")
	}
	c, ok := v.items[id]
	if !ok {
		return c, ErrSecretMissing
	}
	return c, nil
}
func (v *memoryVault) Set(id string, c Credentials) error {
	if v.fail {
		return errors.New("Locked")
	}
	v.items[id] = c
	return nil
}
func (v *memoryVault) Delete(id string) error { delete(v.items, id); return nil }
func TestStore(t *testing.T) {
	v := &memoryVault{items: map[string]Credentials{}}
	s := &Store{Dir: t.TempDir(), Vault: v}
	c := Connection{ID: NewID(), Name: "Test", URL: "https://example.com/mcp", Enabled: true}
	cred := Credentials{Login: "Ігор", Password: " secret-:密碼\n"}
	if e := s.Save(c, cred); e != nil {
		t.Fatal(e)
	}
	got, e := v.Get(c.ID)
	if e != nil || got.Login != cred.Login || got.Password != cred.Password || got.Endpoint != c.URL {
		t.Fatal("credential roundtrip", e)
	}
	c.Name = "Edited"
	if e = s.Save(c, cred); e != nil {
		t.Fatal(e)
	}
	cs, e := s.Load()
	if e != nil || len(cs) != 1 || cs[0].Name != "Edited" {
		t.Fatal(cs, e)
	}
	b, _ := os.ReadFile(filepath.Join(s.Dir, "connections.json"))
	for _, secret := range []string{cred.Login, cred.Password, base64.StdEncoding.EncodeToString([]byte(cred.Login + ":" + cred.Password))} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("secret on disk")
		}
	}
	v.fail = true
	if e = s.Save(c, cred); e == nil {
		t.Fatal("must fail closed")
	}
	v.fail = false
	if e = s.Remove(c.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = v.Get(c.ID); !errors.Is(e, ErrSecretMissing) {
		t.Fatal(e)
	}
}
func TestValidate(t *testing.T) {
	for _, u := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com?password=x", "https://example.com/#x", "bad", "file:///x"} {
		if ValidateURL(u) == nil {
			t.Fatal(u)
		}
	}
	if ValidateURL("https://example.com:443/a/") != nil {
		t.Fatal("valid URL")
	}
}
func mockMCP(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "sentinel-secret" {
			w.WriteHeader(401)
			return
		}
		if r.Method == "GET" {
			w.WriteHeader(405)
			return
		}
		if r.Method == "DELETE" {
			w.WriteHeader(204)
			return
		}
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			w.WriteHeader(400)
			return
		}
		if q.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "fixture-session")
		} else if q.Method != "server/discover" && r.Header.Get("Mcp-Session-Id") != "fixture-session" {
			t.Error("Missing session header")
		}
		if len(q.ID) == 0 {
			w.WriteHeader(204)
			return
		}
		if q.Method != "initialize" && q.Method != "server/discover" && r.Header.Get("MCP-Protocol-Version") != "2025-03-26" {
			t.Error("Missing negotiated protocol header", r.Header)
		}
		var result any
		switch q.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-03-26", "serverInfo": map[string]string{"name": "fixture", "version": "1"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "fixture", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			calls.Add(1)
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "hello"}}}
		case "resources/list":
			result = map[string]any{"resources": []any{}}
		case "drop":
			c, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				c.Close()
			}
			return
		case "transient":
			w.WriteHeader(503)
			return
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32601, "message": "Unsupported"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	})
	return httptest.NewServer(h), &calls
}
func TestProbe(t *testing.T) {
	s, _ := mockMCP(t)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, e := probe(ctx, s.URL, Credentials{Login: "user", Password: "sentinel-secret"})
	if e != nil {
		t.Fatal(e)
	}
	if r.Protocol != "2025-03-26" || len(r.Tools) != 1 || r.Tools[0] != "echo" {
		t.Fatal(r)
	}
	if _, e = probe(ctx, s.URL, Credentials{Login: "user", Password: "bad"}); e == nil || !strings.Contains(e.Error(), "Authorization") {
		t.Fatal(e)
	}
}
func TestUnavailableAndTLS(t *testing.T) {
	s, _ := mockMCP(t)
	s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e := probe(ctx, s.URL, Credentials{Login: "user", Password: "sentinel-secret"}); e == nil {
		t.Fatal("must fail")
	}
	tls := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("untrusted TLS was accepted") }))
	defer tls.Close()
	if _, e := Probe(ctx, tls.URL, Credentials{Login: "user", Password: "sentinel-secret"}); e == nil {
		t.Fatal("TLS must verify")
	}
}
func TestRedirectNoCredentialLeak(t *testing.T) {
	var got atomic.Int32
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got.Add(1) }))
	defer dest.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, http.StatusFound) }))
	defer src.Close()
	if _, e := probe(context.Background(), src.URL, Credentials{Login: "user", Password: "sentinel-secret"}); e == nil {
		t.Fatal("redirect should fail")
	}
	if got.Load() != 0 {
		t.Fatal("redirect followed")
	}
}
func TestBridge(t *testing.T) {
	s, calls := mockMCP(t)
	defer s.Close()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Bridge(ctx, s.URL, Credentials{Login: "user", Password: "sentinel-secret"}, inR, outW) }()
	conn, e := (&mcp.IOTransport{Reader: outR, Writer: inW}).Connect(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	request := func(id int, method string, params string) *jsonrpc.Response {
		t.Helper()
		msg, e := jsonrpc.DecodeMessage([]byte(fmtRequest(id, method, params)))
		if e != nil {
			t.Fatal(e)
		}
		if e = conn.Write(ctx, msg); e != nil {
			t.Fatal(e)
		}
		msg, e = conn.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		resp, ok := msg.(*jsonrpc.Response)
		if !ok {
			t.Fatal("Expected JSON-RPC response")
		}
		return resp
	}
	init := request(1, "initialize", `{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}`)
	if init.Error != nil {
		t.Fatal(init.Error)
	}
	n, _ := jsonrpc.DecodeMessage([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if e = conn.Write(ctx, n); e != nil {
		t.Fatal(e)
	}
	if r := request(2, "tools/list", `{}`); r.Error != nil || !bytes.Contains(r.Result, []byte("echo")) {
		t.Fatal(r)
	}
	if r := request(3, "tools/call", `{"name":"echo","arguments":{}}`); r.Error != nil || !bytes.Contains(r.Result, []byte("hello")) {
		t.Fatal(r)
	}
	if r := request(4, "resources/list", `{}`); r.Error != nil {
		t.Fatal(r)
	}
	if r := request(5, "transient", `{}`); r.Error == nil || strings.Contains(r.Error.Error(), "sentinel-secret") {
		t.Fatal(r)
	}
	if r := request(6, "tools/list", `{}`); r.Error != nil {
		t.Fatal("Did not recover from transient error", r.Error)
	}
	if r := request(7, "drop", `{}`); r.Error == nil {
		t.Fatal("Dropped request must fail")
	}
	if r := request(8, "tools/list", `{}`); r.Error != nil {
		t.Fatal("Did not recover after network disconnect", r.Error)
	}
	if calls.Load() != 1 {
		t.Fatal("Tool was replayed")
	}
	inW.Close()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("adapter did not stop on EOF")
	}
}
func fmtRequest(id int, method, params string) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": json.RawMessage(params)})
	return string(b)
}
func TestRealSDKServer(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "official-sdk-fixture", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "Echo"}, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{JSONResponse: false})
	s := httptest.NewServer(handler)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, e := probe(ctx, s.URL, Credentials{Login: "user", Password: "sentinel-secret"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Tools) != 1 {
		t.Fatal(r)
	}
}

type recordingConn struct {
	mu       sync.Mutex
	messages []jsonrpc.Message
}

func (c *recordingConn) Read(context.Context) (jsonrpc.Message, error) { return nil, io.EOF }
func (c *recordingConn) Write(_ context.Context, m jsonrpc.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, m)
	return nil
}
func (c *recordingConn) Close() error      { return nil }
func (c *recordingConn) SessionID() string { return "" }
func TestSSEReconnect(t *testing.T) {
	var gets atomic.Int32
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := gets.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			io.WriteString(w, "id: resume-1\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\"}\n\n")
			return
		}
		if r.Header.Get("Last-Event-ID") != "resume-1" {
			t.Error("resume ID missing")
		}
		w.WriteHeader(405)
	}))
	defer h.Close()
	client, _ := newHTTP(h.URL, Credentials{Login: "user", Password: "secret"})
	c := &recordingConn{}
	if e := listenSSECurrent(context.Background(), client, h.URL, func() string { return "" }, c); e != nil {
		t.Fatal(e)
	}
	if gets.Load() != 2 || len(c.messages) != 1 {
		t.Fatal("reconnect failed")
	}
}
func TestErrorRedaction(t *testing.T) {
	err := SafeError(errors.New("https://user:sentinel-secret@host: response sentinel-secret"))
	if strings.Contains(err.Error(), "sentinel-secret") {
		t.Fatal("secret leaked")
	}
}
func TestNativeVault(t *testing.T) {
	if os.Getenv("ONECMCP_TEST_NATIVE_VAULT") != "1" {
		t.Skip("Explicit native-vault test opt-in required")
	}
	v := OSVault{}
	id := NewID()
	defer v.Delete(id)
	for _, password := range []string{"synthetic-only-密碼\n ", "updated-synthetic"} {
		want := Credentials{Login: "test-user", Password: password}
		if e := v.Set(id, want); e != nil {
			t.Fatal(e)
		}
		got, e := v.Get(id)
		if e != nil || got != want {
			t.Fatal("Native roundtrip failed", e)
		}
	}
	for _, want := range []Credentials{
		{AuthType: AuthBearer, Token: "synthetic-bearer", Endpoint: "https://example.com/mcp"},
		{AuthType: AuthAPIKey, AuthHeader: "X-API-Key", Token: "synthetic-api-key", Endpoint: "https://example.com/mcp"},
	} {
		if e := v.Set(id, want); e != nil {
			t.Fatal(e)
		}
		got, e := v.Get(id)
		if e != nil || got != want {
			t.Fatal("Native token roundtrip failed", e)
		}
	}
	if e := v.Delete(id); e != nil {
		t.Fatal(e)
	}
	if _, e := v.Get(id); !errors.Is(e, ErrSecretMissing) {
		t.Fatal("Native deletion failed", e)
	}
}

func TestBrandCompatibility(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "MCP Connect")
	legacy := filepath.Join(root, "1C MCP Connect")
	if connectionDir(root) != current {
		t.Fatal("new installations should use the universal name")
	}
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "connections.json"), []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if connectionDir(root) != legacy {
		t.Fatal("existing profiles must remain accessible")
	}
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "connections.json"), []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if connectionDir(root) != current {
		t.Fatal("current profiles should take precedence")
	}
	for _, id := range []string{"onecmcp-0123456789abcdef", NewID()} {
		if err := Validate(Connection{ID: id, Name: "Profile", URL: "https://example.com/mcp"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRemoteResponseLimits(t *testing.T) {
	for _, tc := range []struct {
		size      int
		wantError bool
	}{{MaxMessage, false}, {MaxMessage + 1, true}} {
		body := &boundedBody{ReadCloser: io.NopCloser(strings.NewReader(strings.Repeat("x", tc.size))), remaining: MaxMessage}
		data, err := io.ReadAll(body)
		if (err != nil) != tc.wantError || len(data) > MaxMessage {
			t.Fatalf("size %d: bytes=%d err=%v", tc.size, len(data), err)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "16777217")
		w.WriteHeader(200)
	}))
	defer s.Close()
	client, _ := newHTTP(s.URL, Credentials{Login: "fixture", Password: "sentinel-secret"})
	defer client.CloseIdleConnections()
	if response, err := client.Get(s.URL); err == nil {
		response.Body.Close()
		t.Fatal("oversized content-length accepted")
	}
}

func TestCredentialsBoundToEndpoint(t *testing.T) {
	vault := &memoryVault{items: map[string]Credentials{}}
	store := &Store{Dir: t.TempDir(), Vault: vault}
	c := Connection{ID: NewID(), Name: "Fixture", URL: "https://example.com/mcp"}
	if err := store.Save(c, Credentials{Login: "user", Password: "sentinel-secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveCredentials(c, Credentials{}); err != nil {
		t.Fatal(err)
	}
	c.URL = "https://other.example.com/mcp"
	if _, err := store.ResolveCredentials(c, Credentials{}); err == nil {
		t.Fatal("reused saved password at changed URL")
	}
	if err := store.Write([]Connection{c}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveCredentials(c, Credentials{}); err == nil {
		t.Fatal("metadata tampering bypassed vault endpoint binding")
	}
	creds := vault.items[c.ID]
	client, _ := newHTTP(c.URL, creds)
	if response, err := client.Get(c.URL); err == nil {
		response.Body.Close()
		t.Fatal("transport ignored endpoint binding")
	}
	if _, err := store.ResolveCredentials(c, Credentials{Login: "user", Password: "explicit-new-secret"}); err != nil {
		t.Fatal(err)
	}
}
