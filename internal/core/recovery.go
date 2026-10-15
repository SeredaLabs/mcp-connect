package core

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"sync"
	"time"
)

// recoveringRemote restores the handshake after a broken HTTP session. It never
// replays an application request. Outstanding calls fail with an unknown outcome.
// Subsequent requests can use the new session; SSE streams resume independently.
type recoveringRemote struct {
	ctx         context.Context
	endpoint    string
	http        *http.Client
	auth        *authTransport
	mu          sync.RWMutex
	conn        mcp.Connection
	initialize  jsonrpc.Message
	initialized bool
	closed      bool
	onReset     func()
}

func newRemote(ctx context.Context, endpoint string, h *http.Client, a *authTransport) (*recoveringRemote, error) {
	r := &recoveringRemote{ctx: ctx, endpoint: endpoint, http: h, auth: a}
	c, e := r.connect()
	r.conn = c
	return r, e
}
func (r *recoveringRemote) connect() (mcp.Connection, error) {
	return (&mcp.StreamableClientTransport{Endpoint: r.endpoint, HTTPClient: r.http, DisableStandaloneSSE: true, MaxRetries: 3, MaxEventSize: MaxMessage}).Connect(r.ctx)
}
func (r *recoveringRemote) Write(ctx context.Context, msg jsonrpc.Message) error {
	r.mu.Lock()
	if q, ok := msg.(*jsonrpc.Request); ok {
		if q.Method == "initialize" || q.Method == "server/discover" {
			r.initialize = msg
		}
		if q.Method == "notifications/initialized" {
			r.initialized = true
		}
	}
	c := r.conn
	r.mu.Unlock()
	return c.Write(ctx, msg)
}
func (r *recoveringRemote) Read(ctx context.Context) (jsonrpc.Message, error) {
	for {
		r.mu.RLock()
		c := r.conn
		r.mu.RUnlock()
		msg, e := c.Read(ctx)
		if e == nil {
			return msg, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e = r.recover(c); e != nil {
			return nil, e
		}
	}
}
func (r *recoveringRemote) recover(old mcp.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != old {
		return nil
	}
	if r.closed || r.initialize == nil {
		return errors.New("Cannot recover an uninitialized connection")
	}
	if r.onReset != nil {
		r.onReset()
	}
	old.Close()
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
		}
		c, e := r.connect()
		if e != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
		e = c.Write(ctx, r.initialize)
		if e == nil {
			var msg jsonrpc.Message
			msg, e = c.Read(ctx)
			if e == nil {
				response, ok := msg.(*jsonrpc.Response)
				if !ok || response.Error != nil {
					e = errors.New("Recovery initialization rejected")
				} else {
					var init struct {
						ProtocolVersion string `json:"protocolVersion"`
					}
					if json.Unmarshal(response.Result, &init) == nil && init.ProtocolVersion != "" {
						prior, _ := r.auth.version.Load().(string)
						if prior != "" && prior != init.ProtocolVersion {
							e = errors.New("Protocol changed during recovery")
						} else {
							r.auth.version.Store(init.ProtocolVersion)
						}
					}
				}
			}
		}
		if e == nil && r.initialized {
			n, _ := jsonrpc.DecodeMessage([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
			e = c.Write(ctx, n)
		}
		cancel()
		if e == nil {
			r.conn = c
			return nil
		}
		c.Close()
	}
	return errors.New("Session recovery failed; restart the MCP client")
}
func (r *recoveringRemote) SessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn.SessionID()
}
func (r *recoveringRemote) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.conn.Close()
}
