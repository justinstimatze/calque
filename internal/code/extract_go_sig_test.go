package code

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractGoSig pins the Type-4 signature-rarity channel (Sig) for the Go
// extractor: multi-return and grouped param names must flatten correctly, a
// stdlib-qualified param (context.Context) must render as the bare lowercase
// package name (so it fails signatureInformative's capitalized-type check),
// and a same-module qualified domain type must render qualified (so it passes).
func TestExtractGoSig(t *testing.T) {
	dir := t.TempDir()
	src := `package p

import (
	"context"

	"example.com/mod/geom"
)

func Grouped(x, y int) (string, error) { return "", nil }

func WithContext(ctx context.Context, id string) (*geom.Road, error) { return nil, nil }

func NoParams() {}
`
	f := filepath.Join(dir, "sig.go")
	if err := os.WriteFile(f, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sigs, err := extractGoBatch([]string{f}, dir)
	if err != nil {
		t.Fatalf("extractGoBatch: %v", err)
	}

	grouped := sigByQual(sigs, "Grouped")
	if grouped == nil {
		t.Fatalf("Grouped not extracted; got %d sigs", len(sigs))
	}
	if want := "(int,int)=>(string,error)"; grouped.Sig != want {
		t.Errorf("Grouped.Sig = %q, want %q", grouped.Sig, want)
	}

	withCtx := sigByQual(sigs, "WithContext")
	if withCtx == nil {
		t.Fatalf("WithContext not extracted; got %d sigs", len(sigs))
	}
	// context.Context is stdlib -> renders as bare "context", not "context.Context".
	if want := "(context,string)=>(*geom.Road,error)"; withCtx.Sig != want {
		t.Errorf("WithContext.Sig = %q, want %q", withCtx.Sig, want)
	}

	noParams := sigByQual(sigs, "NoParams")
	if noParams == nil {
		t.Fatalf("NoParams not extracted; got %d sigs", len(sigs))
	}
	if want := "()=>()"; noParams.Sig != want {
		t.Errorf("NoParams.Sig = %q, want %q", noParams.Sig, want)
	}
}

// TestExtractGoSigDomainType pins the informativeness side of the Go fork in
// isolation: a same-module qualified domain type (geom.Road) must register as
// informative, distinguishing it from a bare stdlib package name.
func TestExtractGoSigDomainType(t *testing.T) {
	if !signatureInformative("(context,string)=>*geom.Road") {
		t.Error("a same-module qualified domain type (geom.Road) must register as informative")
	}
	if signatureInformative("(context)=>context") {
		t.Error("a stdlib-only signature (bare lowercased package name) must not register as informative")
	}
}
