package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// drive runs one JSON-RPC request through handleMCP and returns the decoded
// response (nil for a notification that produces no output).
func drive(t *testing.T, method string, params any) *mcpResponse {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	handleMCP(enc, mcpRequest{JSONRPC: "2.0", Method: method, Params: raw, ID: json.RawMessage(`1`)})
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return nil
	}
	var resp mcpResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode response for %s: %v (raw: %s)", method, err, out)
	}
	return &resp
}

// initialize advertises the calque server and the tools capability.
func TestMCPInitialize(t *testing.T) {
	resp := drive(t, "initialize", map[string]any{})
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp)
	}
	res, _ := resp.Result.(map[string]any)
	si, _ := res["serverInfo"].(map[string]any)
	if si["name"] != "calque" {
		t.Errorf("serverInfo.name = %v, want calque", si["name"])
	}
}

// tools/list exposes exactly the two gate tools.
func TestMCPToolsList(t *testing.T) {
	resp := drive(t, "tools/list", map[string]any{})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp)
	}
	res, _ := resp.Result.(map[string]any)
	tools, _ := res["tools"].([]any)
	got := map[string]bool{}
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok {
			got[m["name"].(string)] = true
		}
	}
	for _, want := range []string{"calque_check", "calque_vocab_check"} {
		if !got[want] {
			t.Errorf("tools/list missing %q (got %v)", want, got)
		}
	}
}

// An unknown tool name is a clean JSON-RPC error, not a panic.
func TestMCPUnknownTool(t *testing.T) {
	resp := drive(t, "tools/call", map[string]any{"name": "calque_nope", "arguments": map[string]any{}})
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected an error for unknown tool, got %+v", resp)
	}
}

// A notification (no id) produces no response line.
func TestMCPNotificationSilent(t *testing.T) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	handleMCP(enc, mcpRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	if strings.TrimSpace(sb.String()) != "" {
		t.Errorf("notification should produce no output, got %q", sb.String())
	}
}
