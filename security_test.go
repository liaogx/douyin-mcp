package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/douyin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeOps struct {
	calls int
	fail  error
	qr    []byte
}

func (f *fakeOps) CheckLoginStatus(context.Context) (*douyin.LoginResult, error) {
	f.calls++
	return &douyin.LoginResult{Phase: douyin.PhaseLoggedIn, Success: true}, f.fail
}
func (f *fakeOps) GetLoginQRCode(context.Context) (*douyin.LoginResult, error) {
	f.calls++
	return &douyin.LoginResult{Phase: douyin.PhaseQRCode, QRPNG: f.qr}, f.fail
}
func (f *fakeOps) DeleteCookies(context.Context) error { f.calls++; return f.fail }
func (f *fakeOps) PublishVideo(context.Context, *douyin.PublishRequest) (*douyin.PublishResult, error) {
	f.calls++
	return &douyin.PublishResult{Stage: "ready"}, f.fail
}
func (f *fakeOps) PublishImageText(c context.Context, r *douyin.PublishRequest) (*douyin.PublishResult, error) {
	return f.PublishVideo(c, r)
}

const testToken = "synthetic-test-token-not-a-real-secret-1234"

func testServer(f *fakeOps) *http.Server {
	return NewAppServer(f, configs.Config{Host: "127.0.0.1", Port: 18070, OperationTimeout: time.Minute}, testToken)
}

func TestAllBusinessRoutesRequireAuth(t *testing.T) {
	f := &fakeOps{}
	server := testServer(f)
	for _, route := range []struct{ method, path string }{{"GET", "/api/v1/login/status"}, {"POST", "/api/v1/login/qrcode"}, {"DELETE", "/api/v1/cookies"}, {"POST", "/api/v1/publish/video"}, {"POST", "/api/v1/publish/image-text"}, {"POST", "/mcp"}} {
		r := httptest.NewRequest(route.method, "http://127.0.0.1:18070"+route.path, nil)
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s %s => %d", route.method, route.path, w.Code)
		}
	}
	if f.calls != 0 {
		t.Fatal("unauthenticated request executed business code")
	}
}

func TestHostOriginContentType(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, contentType, fetch string
		want                                   int
	}{
		{"host", "evil.test:18070", "", "application/json", "", 403},
		{"origin", "127.0.0.1:18070", "https://evil.test", "application/json", "", 403},
		{"null-origin", "127.0.0.1:18070", "null", "application/json", "", 403},
		{"fetch", "127.0.0.1:18070", "", "application/json", "cross-site", 403},
		{"plain-json", "127.0.0.1:18070", "", "text/plain", "", 415},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeOps{}
			r := httptest.NewRequest("POST", "http://127.0.0.1:18070/mcp", strings.NewReader(`{}`))
			r.Host = tc.host
			r.Header.Set("Authorization", "Bearer "+testToken)
			r.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.fetch != "" {
				r.Header.Set("Sec-Fetch-Site", tc.fetch)
			}
			w := httptest.NewRecorder()
			testServer(f).Handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("want %d got %d", tc.want, w.Code)
			}
			if f.calls != 0 {
				t.Fatal("rejected request executed")
			}
		})
	}
}

func TestRESTRejectsUnsupportedFeaturesAndReportsFailure(t *testing.T) {
	f := &fakeOps{}
	server := testServer(f)
	for _, body := range []string{`{"title":"x","video_path":"a.mp4","schedule_at":"tomorrow"}`, `{} {}`, `{"image_paths":[1]}`} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:18070/api/v1/publish/video", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+testToken)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("bad JSON accepted %d", w.Code)
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid request reached publisher")
	}
	f.fail = &douyin.Error{Code: "needs_attention", Message: "人工验证", Status: 409}
	r := httptest.NewRequest("GET", "http://127.0.0.1:18070/api/v1/login/status", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("failure returned HTTP %d", w.Code)
	}
}

func rpc(t *testing.T, s *http.Server, body string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "http://127.0.0.1:18070/mcp", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+testToken)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	if strings.Contains(body, "io.modelcontextprotocol/protocolVersion") {
		r.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		r.Header.Set("Mcp-Method", "server/discover")
	}
	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("MCP HTTP %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	return got
}

func TestMCPModernDiscoveryAndLegacyInitialization(t *testing.T) {
	s := testServer(&fakeOps{})
	modern := rpc(t, s, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`)
	b, _ := json.Marshal(modern)
	if !strings.Contains(string(b), "2026-07-28") {
		t.Fatalf("modern discovery: %s", b)
	}
	legacy := rpc(t, s, `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	b, _ = json.Marshal(legacy)
	if !strings.Contains(string(b), "2025-11-25") {
		t.Fatalf("legacy initialization: %s", b)
	}
	list := rpc(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	result, ok := list["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list failed: %v", list)
	}
	if tools, ok := result["tools"].([]any); !ok || len(tools) != 17 {
		t.Fatalf("expected exactly seventeen tools: %v", result)
	}
	b, _ = json.Marshal(list)
	for _, forbidden := range []string{"sms_code", "phone_number", "search_video", "like_video", "schedule_at"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("unexpected feature %s", forbidden)
		}
	}
}

func TestMCPPanicAndBusinessError(t *testing.T) {
	r, _, err := withPanicRecoveryResult("test", func() (*mcp.CallToolResult, any, error) { panic("synthetic") })
	if err != nil || r == nil || !r.IsError {
		t.Fatal("panic swallowed as success")
	}
	r, _, _ = toolResult(nil, errors.New("synthetic failure"))
	if !r.IsError {
		t.Fatal("business failure not flagged")
	}
}

func TestMCPQRCodeEncodingAndNoSMS(t *testing.T) {
	pngBytes := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	s := testServer(&fakeOps{qr: pngBytes})
	r := rpc(t, s, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_login_qrcode","arguments":{}}}`)
	result, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("tool result missing: %v", r)
	}
	contents, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("content missing: %v", r)
	}
	images := 0
	for _, item := range contents {
		c := item.(map[string]any)
		if c["type"] == "image" {
			images++
			raw, err := base64.StdEncoding.DecodeString(c["data"].(string))
			if err != nil || !bytes.Equal(raw, pngBytes) {
				t.Fatal("PNG was double-encoded")
			}
		}
	}
	if images != 1 {
		t.Fatal("expected exactly one QR image")
	}
	bad := rpc(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_login_qrcode","arguments":{"phone_number":"synthetic-not-a-number"}}}`)
	b, _ := json.Marshal(bad)
	if !strings.Contains(string(b), `"isError":true`) && bad["error"] == nil {
		t.Fatalf("unsupported input accepted: %s", b)
	}
}

func TestExecutorRejectsConcurrencyAndCancels(t *testing.T) {
	s := NewDouyinService(configs.Config{OperationTimeout: time.Minute})
	ctx, done, err := s.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.begin(context.Background()); err == nil {
		t.Fatal("overlapping operation accepted")
	}
	s.cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel operation")
	}
	done()
}
