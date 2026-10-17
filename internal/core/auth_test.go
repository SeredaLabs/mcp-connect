package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStaticAuthThroughMCPBridge(t *testing.T) {
	for _, mode := range []string{"", AuthBasic, AuthBearer, AuthAPIKey} {
		t.Run("mode-"+mode, func(t *testing.T) {
			cred := Credentials{AuthType: mode}
			if AuthMode(mode) == AuthBasic {
				cred.Login = "fixture"
				cred.Password = "sentinel-password"
			} else {
				cred.Token = "sentinel-token"
			}
			server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
			mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
			})
			handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{})
			var requests atomic.Int64
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch AuthMode(mode) {
				case AuthBasic:
					user, pass, ok := r.BasicAuth()
					if !ok || user != cred.Login || pass != cred.Password {
						t.Error("missing Basic auth")
						w.WriteHeader(401)
						return
					}
					if r.Header.Get("X-API-Key") != "" {
						t.Error("mixed auth")
					}
				case AuthBearer:
					if r.Header.Get("Authorization") != "Bearer sentinel-token" || r.Header.Get("X-API-Key") != "" {
						t.Error("incorrect Bearer auth")
						w.WriteHeader(401)
						return
					}
				case AuthAPIKey:
					if r.Header.Get("X-API-Key") != cred.Token || r.Header.Get("Authorization") != "" {
						t.Error("incorrect API key auth")
						w.WriteHeader(401)
						return
					}
				}
				handler.ServeHTTP(w, r)
			}))
			defer remote.Close()
			cred.Endpoint = remote.URL
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			inR, inW := io.Pipe()
			outR, outW := io.Pipe()
			defer inW.Close()
			defer outR.Close()
			done := make(chan error, 1)
			go func() { done <- Bridge(ctx, remote.URL, cred, inR, outW) }()
			client := mcp.NewClient(&mcp.Implementation{Name: "fixture-client", Version: "1"}, nil)
			session, err := client.Connect(ctx, &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
			if err != nil {
				t.Fatal(err)
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{}})
			if err != nil || result.IsError {
				t.Fatal("tool call failed", err)
			}
			session.Close()
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("bridge did not stop")
			}
			if requests.Load() < 2 {
				t.Fatal("MCP lifecycle not exercised")
			}
		})
	}
}

func TestTokenVaultBindingAndMetadata(t *testing.T) {
	for _, mode := range []string{AuthBearer, AuthAPIKey} {
		t.Run(mode, func(t *testing.T) {
			vault := &memoryVault{items: map[string]Credentials{}}
			store := &Store{Dir: t.TempDir(), Vault: vault}
			c := Connection{ID: NewID(), Name: "Fixture", URL: "https://example.com/mcp", AuthType: mode}
			cred := Credentials{AuthType: mode, Token: "sentinel-vault-token"}
			if err := store.Save(c, cred); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(store.Dir, "connections.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), cred.Token) || strings.Contains(string(b), `"token"`) {
				t.Fatal("token leaked to metadata")
			}
			got, err := store.ResolveCredentials(c, Credentials{AuthType: mode})
			if err != nil || got.Token != cred.Token {
				t.Fatal("saved token reuse", err)
			}
			changed := c
			changed.URL = "https://other.example.com/mcp"
			if _, err := store.ResolveCredentials(changed, Credentials{AuthType: mode}); err == nil {
				t.Fatal("reused token across endpoints")
			}
			changed = c
			changed.AuthType = AuthBasic
			if _, err := store.ResolveCredentials(changed, Credentials{}); err == nil {
				t.Fatal("reused token across modes")
			}
			if mode == AuthAPIKey {
				changed = c
				changed.AuthHeader = "Api-Key"
				if _, err := store.ResolveCredentials(changed, Credentials{AuthType: mode, AuthHeader: "Api-Key"}); err == nil {
					t.Fatal("reused token across headers")
				}
				// Updating metadata cannot bypass binding inside the vault record.
				if err := store.Write([]Connection{changed}); err != nil {
					t.Fatal(err)
				}
				if _, err := store.ResolveCredentials(changed, Credentials{AuthType: mode, AuthHeader: "Api-Key"}); err == nil {
					t.Fatal("metadata bypassed vault header binding")
				}
			}
		})
	}
}

func TestInvalidAuthNeverSent(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	cases := []Credentials{
		{AuthType: AuthBearer, Token: "secret\r\nX-Evil: injected"},
		{AuthType: AuthBearer, Token: "Bearer secret"},
		{AuthType: AuthAPIKey, Token: "secret\x00value"},
		{AuthType: AuthAPIKey, Token: "secret", AuthHeader: "Authorization"},
		{AuthType: AuthAPIKey, Token: "secret", AuthHeader: "Host"},
		{AuthType: AuthAPIKey, Token: "secret", AuthHeader: "Mcp-Session-Id"},
		{AuthType: "unknown", Token: "secret"},
		{AuthType: AuthBasic, Login: "user", Password: "pass", Token: "secret"},
	}
	for _, cred := range cases {
		client, _ := newHTTP(server.URL, cred)
		if response, err := client.Get(server.URL); err == nil {
			response.Body.Close()
			t.Fatal("invalid credentials reached server")
		}
		client.CloseIdleConnections()
	}
	if hits.Load() != 0 {
		t.Fatal("invalid header transmitted")
	}
}

func TestAPIKeyHeadersAndAuthErrors(t *testing.T) {
	for _, header := range []string{"X-API-Key", "Api-Key", "X-Auth-Token"} {
		t.Run(header, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(header) != "sentinel-key" || r.Header.Get("Authorization") != "" {
					t.Error("wrong authentication header")
				}
				w.WriteHeader(204)
			}))
			defer server.Close()
			client, _ := newHTTP(server.URL, Credentials{AuthType: AuthAPIKey, AuthHeader: header, Token: "sentinel-key"})
			request, _ := http.NewRequest("GET", server.URL, nil)
			request.Header.Set("Authorization", "Bearer stale-token")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			client.CloseIdleConnections()
		})
	}
	for _, status := range []int{401, 403} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, "remote-secret-body")
		}))
		client, _ := newHTTP(server.URL, Credentials{AuthType: AuthBearer, Token: "secret"})
		_, err := client.Get(server.URL)
		safe := SafeError(err)
		want := ErrAuth
		if status == 403 {
			want = ErrPermission
		}
		if !errors.Is(safe, want) || strings.Contains(safe.Error(), "remote-secret-body") {
			t.Fatal("incorrect safe authentication error")
		}
		client.CloseIdleConnections()
		server.Close()
	}
}

func TestTokenRedirectDoesNotLeak(t *testing.T) {
	var hits atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer redirect.Close()
	for _, mode := range []string{AuthBearer, AuthAPIKey} {
		client, _ := newHTTP(redirect.URL, Credentials{AuthType: mode, Token: "sentinel-secret", Endpoint: redirect.URL})
		if response, err := client.Get(redirect.URL); err == nil {
			response.Body.Close()
			t.Fatal("redirect followed")
		}
		if response, err := client.Get(destination.URL); err == nil {
			response.Body.Close()
			t.Fatal("cross-origin request allowed")
		}
		client.CloseIdleConnections()
	}
	if hits.Load() != 0 {
		t.Fatal("destination received token")
	}
}

func TestPortableVaultSizeLimit(t *testing.T) {
	c := Connection{ID: NewID(), Name: "Fixture", URL: "https://example.com/mcp", AuthType: AuthBearer}
	cred := Credentials{AuthType: AuthBearer, Token: strings.Repeat("x", MaxCredentialRecord)}
	vault := &memoryVault{items: map[string]Credentials{}}
	store := &Store{Dir: t.TempDir(), Vault: vault}
	if err := store.Save(c, cred); err == nil {
		t.Fatal("accepted a record that Windows cannot store")
	}
	if len(vault.items) != 0 {
		t.Fatal("oversized record reached vault")
	}
	cred.Token = strings.Repeat("x", 2000)
	if err := store.Save(c, cred); err != nil {
		t.Fatal("portable token rejected", err)
	}
}
func TestLegacyCredentialsRequireExplicitReentry(t *testing.T) {
	c := Connection{ID: NewID(), Name: "Fixture", URL: "https://example.com/mcp"}
	vault := &memoryVault{items: map[string]Credentials{c.ID: {Login: "user", Password: "sentinel"}}}
	store := &Store{Dir: t.TempDir(), Vault: vault}
	if err := store.Write([]Connection{c}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveCredentials(c, Credentials{}); err == nil {
		t.Fatal("unbound legacy secret was reused")
	}
	input := Credentials{Login: "user", Password: "sentinel"}
	got, err := store.ResolveCredentials(c, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(c, got); err != nil {
		t.Fatal(err)
	}
	if vault.items[c.ID].Endpoint != c.URL {
		t.Fatal("explicit resave did not bind endpoint")
	}
}
