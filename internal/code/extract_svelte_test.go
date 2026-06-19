package code

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractSvelteScript pins the .svelte path: calque masks everything outside the
// <script> block(s) and runs the existing TS extractor over the script content, at
// the script's true line offsets. It must (1) extract <script> functions with the
// full effect-footprint (writes/reads/ret_keys/calls), (2) cover BOTH the module and
// instance <script> blocks, (3) report line numbers relative to the whole .svelte
// file, and (4) NOT be confused by the surrounding template ({#if}, markup, mustache
// expressions) — those are out of scope by design.
func TestExtractSvelteScript(t *testing.T) {
	dir := t.TempDir()
	if !tsToolchainAvailable(t, dir) {
		t.Skip("node + typescript not available")
	}
	// A two-block component: a module <script> with a helper, an instance <script>
	// with the provenance-style stamp, and a template with an {#if} branch that must
	// be ignored. The instance stamp() sits on a known line so we can assert mapping.
	src := `<script module>
  export function fmtKey(convId: string, sha: string) {
    return ` + "`${convId}-${sha}`" + `;
  }
</script>

<script lang="ts" generics="T extends Record<string, unknown>">
  export function stamp(msg: any, results: any) {
    msg.webSearch = { query: "q", results: results, ok: true };
    audit.log("stamped");
    return { stamped: true, count: results.length };
  }
</script>

<div class="card">
  {#if msg.webSearch}
    <Results data={msg.webSearch} />
  {/if}
</div>
`
	f := filepath.Join(dir, "ChatMessage.svelte")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractTSBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractTSBatch(.svelte): %v", err)
	}

	// (2) both blocks covered.
	fmtKey := sigByQual(sigs, "fmtKey")
	if fmtKey == nil {
		t.Fatalf("module-block fmtKey not extracted; got %d sigs: %+v", len(sigs), sigs)
	}
	stamp := sigByQual(sigs, "stamp")
	if stamp == nil {
		t.Fatalf("instance-block stamp not extracted; got %d sigs", len(sigs))
	}

	// (1) full effect-footprint on the script function.
	assertHas(t, "stamp.writes", stamp.Writes, []string{"msg.webSearch"})
	assertHas(t, "stamp.ret_keys", stamp.RetKeys, []string{"stamped", "count"})
	assertHas(t, "stamp.calls", stamp.Calls, []string{"log"})

	// (3) line numbers map to the whole file: `stamp` is declared on line 8.
	if stamp.Line != 8 {
		t.Errorf("stamp.Line = %d, want 8 (line offset must survive the mask)", stamp.Line)
	}

	// (4) the template {#if}/markup must not surface as a function.
	if len(sigs) != 2 {
		t.Errorf("expected exactly 2 funcs (fmtKey, stamp); template leaked? got %d: %+v", len(sigs), sigs)
	}
}
