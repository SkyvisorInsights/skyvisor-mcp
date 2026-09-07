package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUnauthenticatedRequestAdvertisesResourceMetadata covers the header that
// actually starts one-click connect. Without it an MCP client reports an auth
// error instead of opening a browser, so the whole OAuth flow never begins.
func TestUnauthenticatedRequestAdvertisesResourceMetadata(t *testing.T) {
	t.Parallel()
	handler := requireBearerOrFallback(false, "https://mcp.skyvisor.test",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Fatal("unauthenticated request reached the MCP handler")
		}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	challenge := rec.Header().Get("WWW-Authenticate")
	want := `Bearer resource_metadata="https://mcp.skyvisor.test/.well-known/oauth-protected-resource"`
	if challenge != want {
		t.Fatalf("WWW-Authenticate = %q, want %q", challenge, want)
	}
}

// TestFallbackTokenSkipsChallenge keeps the smoke-test path working: when a
// fallback token is configured the server must not demand a per-request bearer.
func TestFallbackTokenSkipsChallenge(t *testing.T) {
	t.Parallel()
	var reached bool
	handler := requireBearerOrFallback(true, "https://mcp.skyvisor.test",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if !reached || rec.Code != http.StatusOK {
		t.Fatalf("fallback path blocked: reached=%v status=%d", reached, rec.Code)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	protectedResourceMetadata("https://mcp.skyvisor.test", "https://api.skyvisor.test")(
		rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if meta.Resource != "https://mcp.skyvisor.test" {
		t.Fatalf("resource = %q", meta.Resource)
	}
	if len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != "https://api.skyvisor.test" {
		t.Fatalf("authorization_servers = %v", meta.AuthorizationServers)
	}
	if strings.Join(meta.ScopesSupported, " ") != "skyvisor:read skyvisor:act" {
		t.Fatalf("scopes_supported = %v", meta.ScopesSupported)
	}
}

func TestPortFromAddr(t *testing.T) {
	t.Parallel()
	for addr, want := range map[string]string{
		"0.0.0.0:8087":   "8087",
		"127.0.0.1:9000": "9000",
		"garbage":        "8087",
		"":               "8087",
	} {
		if got := portFromAddr(addr); got != want {
			t.Fatalf("portFromAddr(%q) = %q, want %q", addr, got, want)
		}
	}
}
