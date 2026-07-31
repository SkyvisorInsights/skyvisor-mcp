package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registeredToolNames builds the real server the way main.go does and reads
// back its tool list over an in-memory MCP session. This is the single
// source of truth for "what tools does this server expose" — there is no
// second, hand-maintained list to drift from reality.
//
// newMCPServer(nil) is safe here: mcp.AddTool only stores the handler
// closure, it never invokes it, so a nil *apiclient.Client is fine as long
// as no tool is actually called.
func registeredToolNames(t *testing.T) map[string]bool {
	t.Helper()
	ctx := context.Background()

	server := newMCPServer(nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	names := map[string]bool{}
	for tool, err := range clientSession.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names[tool.Name] = true
	}
	return names
}

// The agent surface is deliberately asymmetric: an agent may revoke a public
// trust report but may not publish one, because publishing exposes customer
// data on an unauthenticated URL.
func TestTrustShareToolsAreAsymmetric(t *testing.T) {
	names := registeredToolNames(t)
	for _, want := range []string{"list_trust_shares", "revoke_trust_share", "get_decision_trust"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
	if names["create_trust_share"] {
		t.Error("create_trust_share must not be exposed over MCP: publishing a trust report is a human-approved action")
	}
}
