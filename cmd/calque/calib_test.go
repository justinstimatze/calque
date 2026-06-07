package main

import (
	"path/filepath"
	"testing"
)

func TestFireIDStable(t *testing.T) {
	a := fireID("pair", "x|y")
	if a != fireID("pair", "x|y") {
		t.Fatal("fireID must be stable across calls")
	}
	if fireID("cluster", "x|y") == a {
		t.Fatal("kind must affect the id")
	}
	if len(a) != 10 {
		t.Fatalf("id length = %d, want 10", len(a))
	}
}

func TestVerdictLabel(t *testing.T) {
	for in, want := range map[string]string{
		"drift":              "useful",
		"false-alarm":        "not-useful",
		"contracted-twin-ok": "mixed",
		"":                   "",
		"unknown":            "",
	} {
		if got := verdictLabel(in); got != want {
			t.Errorf("verdictLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFireTagRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fire-tags.jsonl")
	mustAppend := func(tag fireTag) {
		if err := appendJSONL(path, tag); err != nil {
			t.Fatal(err)
		}
	}
	mustAppend(fireTag{ID: "abc", Verdict: "useful"})
	mustAppend(fireTag{ID: "abc", Verdict: "not-useful"}) // latest wins
	mustAppend(fireTag{ID: "def", Verdict: "mixed"})
	mustAppend(fireTag{ID: "ghi", Verdict: "bogus"}) // invalid → ignored

	tags := loadFireTags(path)
	if tags["abc"] != "not-useful" {
		t.Errorf("latest tag should win, got %q", tags["abc"])
	}
	if tags["def"] != "mixed" {
		t.Errorf("def = %q, want mixed", tags["def"])
	}
	if _, ok := tags["ghi"]; ok {
		t.Error("invalid verdict must be ignored")
	}
}
