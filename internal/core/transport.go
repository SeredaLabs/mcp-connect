package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const MaxMessage = 16 * 1024 * 1024

var errStreamRead = errors.New("SSE stream interrupted")

var ErrAuth = errors.New("Authorization failed (401): check credentials or replace an expired token")
var ErrPermission = errors.New("Access denied (403): check server permissions or token scope")

type authTransport struct {
	base     http.RoundTripper
	cred     Credentials
	version  atomic.Value
	origin   string
	endpoint string
}

func (t *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.cred.Endpoint != "" && t.cred.Endpoint != t.endpoint {
		return nil, errors.New("Credentials are bound to a different endpoint")
	}
	if r.URL.Scheme+"://"+r.URL.Host != t.origin {
		return nil, errors.New("Cross-origin request blocked")
	}
	q := r.Clone(r.Context())
	q.Header = r.Header.Clone()
	if err := t.cred.applyAuth(q.Header); err != nil {
		return nil, err
	}
	if v, _ := t.version.Load().(string); v != "" && q.Header.Get("MCP-Protocol-Version") == "" {
		q.Header.Set("MCP-Protocol-Version", v)
	}
	resp, e := t.base.RoundTrip(q)
	if e != nil {
		if r.Context().Err() != nil {
			return nil, r.Context().Err()
		}
		return nil, errors.New("Network or TLS connection failed")
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		resp.Body.Close()
		if resp.StatusCode == 403 {
			return nil, ErrPermission
		}
		return nil, ErrAuth
	}
	// Bound each non-streaming response, including decompressed response bodies.
	// SSE is bounded per event instead, because a stream may remain open.
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]))
	if contentType != "text/event-stream" {
		if resp.ContentLength > MaxMessage {
			resp.Body.Close()
			return nil, errors.New("MCP response exceeds size limit")
		}
		resp.Body = &boundedBody{ReadCloser: resp.Body, remaining: MaxMessage}
	}
	return resp, nil
}

type boundedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *boundedBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.remaining == 0 {
		var extra [1]byte
		n, err := b.ReadCloser.Read(extra[:])
		if n > 0 {
			return 0, errors.New("MCP response exceeds size limit")
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}
func newHTTP(endpoint string, cred Credentials) (*http.Client, *authTransport) {
	parts := strings.SplitN(endpoint, "/", 4)
	origin := parts[0] + "//" + parts[2]
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.ResponseHeaderTimeout = 30 * time.Second
	base.TLSHandshakeTimeout = 10 * time.Second
	a := &authTransport{base: base, cred: cred, origin: origin, endpoint: endpoint}
	return &http.Client{Transport: a, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Redirect blocked; enter the final HTTPS endpoint")
	}}, a
}

// SafeError deliberately never includes remote response bodies, URLs or credentials.
func SafeError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, ErrPermission) || strings.Contains(e.Error(), ErrPermission.Error()) {
		return ErrPermission
	}
	if errors.Is(e, ErrAuth) || strings.Contains(e.Error(), ErrAuth.Error()) {
		return ErrAuth
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return errors.New("Request timed out; its outcome may be unknown. It was not replayed")
	}
	return errors.New("MCP connection failed: check the HTTPS endpoint, server availability and transport compatibility. The request was not replayed")
}

type ProbeResult struct {
	Protocol string
	Server   string
	Tools    []string
}

func Probe(ctx context.Context, endpoint string, cred Credentials) (ProbeResult, error) {
	if e := ValidateURL(endpoint); e != nil {
		return ProbeResult{}, e
	}
	return probe(ctx, endpoint, cred)
}
func probe(ctx context.Context, endpoint string, cred Credentials) (ProbeResult, error) {
	type result struct {
		value ProbeResult
		err   error
	}
	ready := make(chan result)
	go func() {
		r, e := probeSDK(ctx, endpoint, cred)
		select {
		case ready <- result{r, e}:
		case <-ctx.Done():
		}
	}()
	select {
	case r := <-ready:
		return r.value, r.err
	case <-ctx.Done():
		return ProbeResult{}, SafeError(ctx.Err())
	}
}

// The outer deadline also bounds SDK session cleanup, whose best-effort DELETE
// may otherwise delay a timed-out connection attempt. Cleanup still completes.
func probeSDK(ctx context.Context, endpoint string, cred Credentials) (ProbeResult, error) {
	h, _ := newHTTP(endpoint, cred)
	defer h.CloseIdleConnections()
	client := mcp.NewClient(&mcp.Implementation{Name: "MCP Connect", Version: Version}, nil)
	session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: h, DisableStandaloneSSE: true, MaxEventSize: MaxMessage}, nil)
	if e != nil {
		return ProbeResult{}, SafeError(e)
	}
	defer session.Close()
	init := session.InitializeResult()
	if init == nil || init.ServerInfo == nil {
		return ProbeResult{}, errors.New("Server returned an incomplete MCP initialization response")
	}
	result := ProbeResult{Protocol: init.ProtocolVersion, Server: init.ServerInfo.Name}
	cursor := ""
	seen := map[string]bool{}
	for {
		r, e := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if e != nil {
			return result, SafeError(e)
		}
		for _, tool := range r.Tools {
			result.Tools = append(result.Tools, tool.Name)
		}
		if r.NextCursor == "" {
			break
		}
		if seen[r.NextCursor] {
			return result, errors.New("Server returned a repeated tools cursor")
		}
		seen[r.NextCursor] = true
		cursor = r.NextCursor
	}
	return result, nil
}

type pendingCall struct {
	cancel context.CancelFunc
	timer  *time.Timer
}

// Bridge forwards JSON-RPC without rebuilding the tool/resource/prompt schemas.
// SDK handles POST JSON/SSE, session IDs and DELETE. No tools/call is replayed.
func Bridge(ctx context.Context, endpoint string, cred Credentials, in io.ReadCloser, out io.WriteCloser) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h, auth := newHTTP(endpoint, cred)
	defer h.CloseIdleConnections()
	remote, e := newRemote(ctx, endpoint, h, auth)
	if e != nil {
		return SafeError(e)
	}
	defer remote.Close()
	local, e := (&mcp.IOTransport{Reader: in, Writer: out, MaxLineLength: MaxMessage}).Connect(ctx)
	if e != nil {
		return e
	}
	defer local.Close()
	var mu sync.Mutex
	pending := map[jsonrpc.ID]pendingCall{}
	var sseOnce sync.Once
	done := make(chan error, 3)
	finish := func(id jsonrpc.ID) bool {
		mu.Lock()
		defer mu.Unlock()
		p, ok := pending[id]
		if ok {
			delete(pending, id)
			p.timer.Stop()
			p.cancel()
		}
		return ok
	}
	failCall := func(id jsonrpc.ID, err error) {
		if finish(id) {
			_ = local.Write(ctx, &jsonrpc.Response{ID: id, Error: &jsonrpc.Error{Code: -32000, Message: SafeError(err).Error()}})
		}
	}
	remote.onReset = func() {
		mu.Lock()
		ids := make([]jsonrpc.ID, 0, len(pending))
		for id := range pending {
			ids = append(ids, id)
		}
		mu.Unlock()
		for _, id := range ids {
			failCall(id, errors.New("Session lost; request outcome unknown"))
		}
	}
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, p := range pending {
			p.timer.Stop()
			p.cancel()
		}
	}()
	go func() {
		for {
			msg, e := remote.Read(ctx)
			if e != nil {
				done <- SafeError(e)
				return
			}
			if r, ok := msg.(*jsonrpc.Response); ok {
				if !finish(r.ID) {
					continue
				}
				var init struct {
					ProtocolVersion string `json:"protocolVersion"`
				}
				if json.Unmarshal(r.Result, &init) == nil && init.ProtocolVersion != "" {
					auth.version.Store(init.ProtocolVersion)
				}
			}
			if e = local.Write(ctx, msg); e != nil {
				done <- e
				return
			}
		}
	}()
	go func() {
		for {
			msg, e := local.Read(ctx)
			if e != nil {
				if errors.Is(e, io.EOF) {
					done <- nil
				} else {
					done <- errors.New("Invalid or oversized stdio JSON-RPC message")
				}
				return
			}
			r, ok := msg.(*jsonrpc.Request)
			if ok && r.IsCall() {
				callCtx, callCancel := context.WithCancel(ctx)
				mu.Lock()
				if _, exists := pending[r.ID]; exists {
					mu.Unlock()
					callCancel()
					done <- errors.New("Duplicate active JSON-RPC request ID")
					return
				}
				id := r.ID
				p := pendingCall{cancel: callCancel}
				p.timer = time.AfterFunc(60*time.Second, func() { failCall(id, context.DeadlineExceeded) })
				pending[id] = p
				mu.Unlock()
				go func() {
					if e := remote.Write(callCtx, msg); e != nil {
						failCall(id, e)
					}
				}()
				continue
			}
			if ok && r.Method == "notifications/cancelled" {
				var p struct {
					RequestID any `json:"requestId"`
				}
				if json.Unmarshal(r.Params, &p) == nil {
					if id, e := jsonrpc.MakeID(p.RequestID); e == nil {
						finish(id)
					}
				}
			}
			if e = remote.Write(ctx, msg); e != nil {
				done <- SafeError(e)
				return
			}
			if ok && r.Method == "notifications/initialized" {
				sseOnce.Do(func() {
					go func() {
						if e := listenSSECurrent(ctx, h, endpoint, remote.SessionID, local); e != nil {
							done <- SafeError(e)
						}
					}()
				})
			}
		}
	}()
	select {
	case e := <-done:
		cancel()
		return e
	case <-ctx.Done():
		return nil
	}
}

// Standalone SSE is optional. A 405 is the normal response of the 1C backend.
// Reconnect uses Last-Event-ID and never replays a POST or tool call.
func listenSSECurrent(ctx context.Context, h *http.Client, endpoint string, sessionID func() string, local mcp.Connection) error {
	last := ""
	lastSession := ""
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if e != nil {
			return e
		}
		req.Header.Set("Accept", "text/event-stream")
		session := sessionID()
		if session != lastSession {
			last = ""
			lastSession = session
		}
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		if last != "" {
			req.Header.Set("Last-Event-ID", last)
		}
		resp, e := h.Do(req)
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		if resp.StatusCode == 405 {
			resp.Body.Close()
			return nil
		}
		if resp.StatusCode == 404 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			resp.Body.Close()
			continue
		}
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			resp.Body.Close()
			return errors.New("Unexpected standalone SSE response")
		}
		e = readEvents(ctx, resp.Body, local, &last)
		resp.Body.Close()
		if ctx.Err() != nil {
			return nil
		}
		if e != nil && !errors.Is(e, io.EOF) && !errors.Is(e, errStreamRead) {
			return e
		}
	}
	return errors.New("SSE reconnection attempts exhausted")
}
func readEvents(ctx context.Context, r io.Reader, local mcp.Connection, last *string) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), MaxMessage)
	data := ""
	eventID := ""
	for s.Scan() {
		line := s.Text()
		if line == "" {
			if eventID != "" {
				*last = eventID
			}
			if data != "" {
				msg, e := jsonrpc.DecodeMessage([]byte(strings.TrimSuffix(data, "\n")))
				if e != nil {
					return errors.New("Malformed SSE message")
				}
				if e = local.Write(ctx, msg); e != nil {
					return e
				}
			}
			data = ""
			eventID = ""
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ") + "\n"
			if len(data) > MaxMessage {
				return fmt.Errorf("SSE event too large")
			}
		}
		if strings.HasPrefix(line, "id:") {
			eventID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
	}
	if s.Err() != nil {
		return errStreamRead
	}
	return io.EOF
}
