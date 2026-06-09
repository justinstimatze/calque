package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinstimatze/calque/internal/code"
	"github.com/justinstimatze/calque/internal/registry"
)

// pSig builds a prepared FuncSig for the cluster pass (NLines must clear MinLines=4
// for a function to enter the clustering pool).
func pSig(file, name string, nlines int, calls, strs []string) *code.FuncSig {
	f := &code.FuncSig{File: file, Qualname: name, Name: name, NLines: nlines, Calls: calls, Strings: strs}
	f.Prepare()
	return f
}

// threeSharingCall: three real functions sharing the rare private call `_seam`,
// the minimal shape that forms a default cluster (N>=3, fanout 3, score 0.53>=0.40).
func threeSharingCall() []*code.FuncSig {
	return []*code.FuncSig{
		pSig("a.go", "Af", 6, []string{"_seam", "x"}, nil),
		pSig("b.go", "Bf", 6, []string{"_seam", "y"}, nil),
		pSig("c.go", "Cf", 6, []string{"_seam", "z"}, nil),
	}
}

func emptyReg(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.Load(filepath.Join(t.TempDir(), "none.md"))
	if err != nil {
		t.Fatalf("load empty registry: %v", err)
	}
	return reg
}

func loadRegistry(t *testing.T, body string) *registry.Registry {
	t.Helper()
	p := filepath.Join(t.TempDir(), "registry.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	reg, err := registry.Load(p)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return reg
}

func TestComputeProposals_CallsSynth(t *testing.T) {
	sigs := threeSharingCall()
	clusters := code.ClusterByTouchpoint(sigs, code.DefaultClusterOptions())
	if len(clusters) != 1 {
		t.Fatalf("want exactly 1 cluster, got %d", len(clusters))
	}
	props := computeProposals(sigs, clusters, emptyReg(t))
	if len(props) != 1 {
		t.Fatalf("want 1 proposal, got %d", len(props))
	}
	p := props[0]
	if p.Predicate != "calls:_seam" {
		t.Fatalf("want predicate calls:_seam, got %q", p.Predicate)
	}
	if p.Approx {
		t.Fatalf("a fully-covering calls seam must not be approximate")
	}
	if p.Name != "seam" {
		t.Fatalf("want kebab name seam (from _seam), got %q", p.Name)
	}
	if len(p.Baseline) != 3 {
		t.Fatalf("want 3 baseline members, got %v", p.Baseline)
	}
	if len(p.Extra) != 0 || len(p.Missing) != 0 {
		t.Fatalf("exact self-verify expected, got extra=%v missing=%v", p.Extra, p.Missing)
	}
}

func TestComputeProposals_BroadPredicate(t *testing.T) {
	sigs := threeSharingCall()
	// A short 4th function also calls _seam: below MinLines so it's excluded from the
	// cluster pool, but Implementers (no length filter) matches it — so the synthesized
	// predicate is too broad and the verify must surface the non-member.
	sigs = append(sigs, pSig("d.go", "Df", 2, []string{"_seam"}, nil))
	clusters := code.ClusterByTouchpoint(sigs, code.DefaultClusterOptions())
	if len(clusters) != 1 || len(clusters[0].Members) != 3 {
		t.Fatalf("want 1 cluster of 3 (short Df excluded), got %d clusters", len(clusters))
	}
	props := computeProposals(sigs, clusters, emptyReg(t))
	if len(props) != 1 {
		t.Fatalf("want 1 proposal, got %d", len(props))
	}
	if len(props[0].Extra) != 1 || props[0].Extra[0] != "d.go::Df" {
		t.Fatalf("predicate breadth must surface d.go::Df as extra, got %v", props[0].Extra)
	}
}

func TestComputeProposals_EmitsFallback(t *testing.T) {
	// Seam appears only as an emitted string, not a call → synthesize an emits: term.
	sigs := []*code.FuncSig{
		pSig("a.go", "Af", 6, nil, []string{"_seam"}),
		pSig("b.go", "Bf", 6, nil, []string{"_seam"}),
		pSig("c.go", "Cf", 6, nil, []string{"_seam"}),
	}
	clusters := code.ClusterByTouchpoint(sigs, code.DefaultClusterOptions())
	if len(clusters) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(clusters))
	}
	props := computeProposals(sigs, clusters, emptyReg(t))
	if len(props) != 1 || props[0].Predicate != "emits:_seam" {
		t.Fatalf("want emits:_seam fallback, got %+v", props)
	}
	if props[0].Approx {
		t.Fatalf("a fully-covering emits seam must not be approximate")
	}
}

func TestComputeProposals_DedupCluster(t *testing.T) {
	sigs := threeSharingCall()
	clusters := code.ClusterByTouchpoint(sigs, code.DefaultClusterOptions())
	reg := loadRegistry(t, "## c1\n- cluster: a.go::Af | b.go::Bf | c.go::Cf\n- verdict: false-alarm\n")
	props := computeProposals(sigs, clusters, reg)
	if len(props) != 0 {
		t.Fatalf("an adjudicated cluster must not be re-proposed, got %d", len(props))
	}
}

func TestComputeProposals_DedupDeclaredRole(t *testing.T) {
	sigs := threeSharingCall()
	clusters := code.ClusterByTouchpoint(sigs, code.DefaultClusterOptions())
	// A declared role whose predicate already selects the same member set.
	reg := loadRegistry(t, "## role: existing\n- role: existing\n- predicate: calls:_seam\n- expected: 1\n")
	props := computeProposals(sigs, clusters, reg)
	if len(props) != 0 {
		t.Fatalf("a cluster already covered by a declared role must not be re-proposed, got %d", len(props))
	}
}
