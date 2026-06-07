package main

// MCP stdio server — the inline-agent surface for calque's gates. Speaks
// JSON-RPC 2.0 on newline-delimited stdio per the Model Context Protocol spec,
// so an agent editing code or prose can ask "did my change introduce new drift?"
// without shelling out, and get the same report `calque check` / `vocab-check`
// print on the CLI. Read-only: the MCP path never writes the fire log or exits
// the process (that stays the CLI/hook telemetry path).
//
// Exposes two tools — the two gates:
//   - calque_check        code axis: scan + diff vs the registry; new/known/stale
//   - calque_vocab_check  prose axis: hyphenated compounds (freq >= min) not in
//                         the allow-list (file + optional --seed-cmd seeder)
//
// Framing (the JSON-RPC plumbing) lifted from the sibling Go project hindcast
// (cmd/hindcast/cmd_mcp.go) — a zero-dependency stdlib implementation, which
// keeps calque dependency-free.

import (
	"bufio"
	"encoding/json"
	"os"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func runMCP(_ []string) {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		handleMCP(enc, req)
	}
}

func handleMCP(enc *json.Encoder, req mcpRequest) {
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		_ = enc.Encode(mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{
					"name":    "calque",
					"version": buildVersion(),
				},
				"instructions": mcpInstructions,
			},
		})

	case "notifications/initialized", "notifications/cancelled":
		// No response for notifications.

	case "tools/list":
		_ = enc.Encode(mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": []any{checkToolDefinition(), vocabCheckToolDefinition()}},
		})

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			_ = enc.Encode(errResp(req.ID, -32602, "invalid params"))
			return
		}
		switch p.Name {
		case "calque_check":
			_ = enc.Encode(toolText(req.ID, mcpCheck(p.Arguments)))
		case "calque_vocab_check":
			_ = enc.Encode(toolText(req.ID, mcpVocabCheck(p.Arguments)))
		default:
			_ = enc.Encode(errResp(req.ID, -32601, "unknown tool: "+p.Name))
		}

	default:
		if !isNotification {
			_ = enc.Encode(errResp(req.ID, -32601, "method not found: "+req.Method))
		}
	}
}

// mcpCheck runs the code-axis gate from MCP arguments and returns the report
// text. Read-only (no fire log, no exit). repo defaults to the cwd.
func mcpCheck(rawArgs json.RawMessage) string {
	var a struct {
		Repo              string  `json:"repo"`
		Left              string  `json:"left"`
		Right             string  `json:"right"`
		Exclude           string  `json:"exclude"`
		MinScore          float64 `json:"min_score"`
		MinLines          int     `json:"min_lines"`
		ClusterMinMembers int     `json:"cluster_min_members"`
		ClusterMaxFanout  int     `json:"cluster_max_fanout"`
		Registry          string  `json:"registry"`
	}
	// Defaults match the CLI flag defaults in check.go.
	a.Repo, a.MinScore, a.MinLines = ".", 0.18, 4
	a.ClusterMinMembers, a.ClusterMaxFanout = 3, 8
	a.Registry = ".calque/registry.md"
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &a); err != nil {
			return "calque_check: invalid arguments: " + err.Error()
		}
	}
	if a.Repo == "" {
		a.Repo = "."
	}
	f, err := computeCheck(a.Repo, a.Left, a.Right, a.Exclude, a.MinScore, a.MinLines, a.ClusterMinMembers, a.ClusterMaxFanout, a.Registry)
	if err != nil {
		return "calque_check: " + err.Error()
	}
	return renderCheck(f, a.Registry)
}

// mcpVocabCheck runs the prose-axis gate from MCP arguments and returns the
// report text. dir defaults to the cwd.
func mcpVocabCheck(rawArgs json.RawMessage) string {
	var a struct {
		Dir       string `json:"dir"`
		Ext       string `json:"ext"`
		Exclude   string `json:"exclude"`
		Allowlist string `json:"allowlist"`
		SeedCmd   string `json:"seed_cmd"`
		Min       int    `json:"min"`
		Locs      int    `json:"locs"`
	}
	a.Dir, a.Allowlist, a.Min, a.Locs = ".", ".calque/vocab-allowlist.txt", 5, 2
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &a); err != nil {
			return "calque_vocab_check: invalid arguments: " + err.Error()
		}
	}
	if a.Dir == "" {
		a.Dir = "."
	}
	f, err := computeVocabCheck(a.Dir, a.Ext, a.Exclude, a.Allowlist, a.SeedCmd, a.Min, a.Locs)
	if err != nil {
		return "calque_vocab_check: " + err.Error()
	}
	return renderVocabCheck(f, a.Allowlist)
}

// toolText wraps a plain-text tool result in the MCP content envelope.
func toolText(id json.RawMessage, text string) mcpResponse {
	return mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}},
	}
}

func errResp(id json.RawMessage, code int, msg string) mcpResponse {
	return mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: msg},
	}
}

// mcpInstructions is the behavioral contract delivered in the initialize
// response — it travels with the tool to whatever MCP client mounts it, not
// just Claude Code.
const mcpInstructions = `calque is a substrate-general drift nose: it surfaces
"one contract/concept/value defined in N places that silently drift" across
code and prose. The two MCP tools are calque's GATES — call them to verify a
change did not introduce NEW drift, then adjudicate anything new into the
registry.

WHEN TO USE:

  calque_check        After editing code, ask whether the change introduced a
                      new dual-path / behavioral-twin pair or an N-ary
                      touchpoint cluster (a private seam inlined into several
                      functions). Reports new vs already-adjudicated (known) vs
                      stale (registry entries whose code is gone).
  calque_vocab_check  After editing prose/docs, ask whether a hyphenated
                      compound at freq >= min appeared that is not in the
                      allow-list (the prose registry) — invented vocabulary
                      proliferating before it entrenches.

These are READ-ONLY: they do not write the fire log or block anything. NEW
findings are not bugs — they are un-adjudicated suspects. Resolve each by (a)
fixing real drift, or (b) recording it in the registry (.calque/registry.md for
code, .calque/vocab-allowlist.txt for prose) as contracted-twin-ok. A clean
gate means zero NEW.

TOOLS:

  calque_check([repo], [left], [right], [exclude], [min_score], [min_lines],
               [cluster_min_members], [cluster_max_fanout], [registry])
    repo defaults to the cwd. left/right are glob boundaries (default
    self-scan: all source x all source). exclude is comma-separated path globs.

  calque_vocab_check([dir], [ext], [exclude], [allowlist], [seed_cmd], [min],
                     [locs])
    dir defaults to the cwd. seed_cmd runs a project's own catalog->slug seeder
    (the seeder contract: print slugs to stdout) and merges it into the
    allow-list. min is the frequency threshold (default 5).`

func checkToolDefinition() map[string]any {
	return map[string]any{
		"name":        "calque_check",
		"description": "Code-axis drift gate: scan a repo for dual-path / behavioral-twin (Type-4) suspect pairs and N-ary touchpoint clusters, diff against the registry, and report only NEW (un-adjudicated) suspects plus known and stale ones. Call after editing code to verify no new drift slipped in. Read-only (no fire log, no exit).",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":                map[string]any{"type": "string", "description": "Repo root to scan. Defaults to the current working directory."},
				"left":                map[string]any{"type": "string", "description": "Optional left-side boundary glob (e.g. engine*.py). Default: self-scan (all source)."},
				"right":               map[string]any{"type": "string", "description": "Optional right-side boundary glob. Default: self-scan."},
				"exclude":             map[string]any{"type": "string", "description": "Optional comma-separated path globs to skip (e.g. legacy/**,vendor/**)."},
				"min_score":           map[string]any{"type": "number", "description": "Minimum suspicion score to consider (default 0.18)."},
				"min_lines":           map[string]any{"type": "number", "description": "Ignore functions shorter than this many lines (default 4)."},
				"cluster_min_members": map[string]any{"type": "number", "description": "Smallest N-ary cluster to consider (default 3)."},
				"cluster_max_fanout":  map[string]any{"type": "number", "description": "A private symbol touched by more than this is plumbing, not a seam (default 8)."},
				"registry":            map[string]any{"type": "string", "description": "Registry file of adjudicated pairs/clusters (default .calque/registry.md)."},
			},
		},
	}
}

func vocabCheckToolDefinition() map[string]any {
	return map[string]any{
		"name":        "calque_vocab_check",
		"description": "Prose-axis drift gate: flag hyphenated compounds at frequency >= min that are not in the allow-list (the prose registry) — invented vocabulary noun-stacks proliferating before they entrench. Call after editing prose/docs. Read-only.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir":       map[string]any{"type": "string", "description": "Repo root to walk for prose. Defaults to the current working directory."},
				"ext":       map[string]any{"type": "string", "description": "Comma-separated prose extensions (default: md,markdown,mdx,txt,rst)."},
				"exclude":   map[string]any{"type": "string", "description": "Comma-separated path globs to skip (e.g. refs/**,theory/working/**)."},
				"allowlist": map[string]any{"type": "string", "description": "Allow-list file, one slug per line (default .calque/vocab-allowlist.txt)."},
				"seed_cmd":  map[string]any{"type": "string", "description": "Optional shell command whose stdout (one slug per line) is merged into the allow-list — the seeder contract, for project-specific catalog->slug logic."},
				"min":       map[string]any{"type": "number", "description": "Minimum frequency to flag a missing compound (default 5)."},
				"locs":      map[string]any{"type": "number", "description": "Max example file:line cites per flagged compound (default 2)."},
			},
		},
	}
}
