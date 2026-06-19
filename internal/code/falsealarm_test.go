package code

import "testing"

func TestFalseAlarmHint(t *testing.T) {
	mk := func(qual string, reads, writes []string) *FuncSig {
		f := &FuncSig{Qualname: qual, Name: qual, Reads: reads, Writes: writes}
		f.Prepare()
		return f
	}

	// Two methods on the same receiver type — same-receiver (wins even though they
	// don't field-copy).
	if got := FalseAlarmHint(
		mk("Bucket.Summary", []string{"x"}, []string{"y"}),
		mk("Bucket.Project", []string{"x"}, []string{"y"}),
	); got != "same-receiver" {
		t.Errorf("two methods on Bucket: got %q, want same-receiver", got)
	}

	// Different receiver types — NOT same-receiver.
	if got := FalseAlarmHint(
		mk("Bucket.Summary", []string{"x"}, nil),
		mk("Request.Project", []string{"x"}, nil),
	); got == "same-receiver" {
		t.Error("different receiver types must not be tagged same-receiver")
	}

	// Both sides copy the fields they read straight to writes — field-copy.
	if got := FalseAlarmHint(
		mk("toDTO", []string{"width", "height", "depth"}, []string{"width", "height", "depth"}),
		mk("toRow", []string{"width", "height", "depth"}, []string{"width", "height", "depth"}),
	); got != "field-copy" {
		t.Errorf("two projection mappers: got %q, want field-copy", got)
	}

	// A genuine derivation pair — writes a NEW quantity, not a copy — gets no hint.
	if got := FalseAlarmHint(
		mk("buildA", []string{"road.width", "road.pieces"}, []string{"mesh"}),
		mk("buildB", []string{"road.width", "road.pieces"}, []string{"verts"}),
	); got != "" {
		t.Errorf("derivation pair must get no hint, got %q", got)
	}

	// One-sided copy (only A is a mapper) — not enough for the field-copy pair tag.
	if got := FalseAlarmHint(
		mk("toDTO", []string{"width", "height"}, []string{"width", "height"}),
		mk("buildB", []string{"width", "height"}, []string{"mesh", "verts"}),
	); got == "field-copy" {
		t.Error("only one side is a mapper — must not tag field-copy")
	}
}

func TestFalseAlarmHintSvelteKit(t *testing.T) {
	mk := func(file, name string) *FuncSig {
		f := &FuncSig{File: file, Qualname: name, Name: name}
		f.Prepare()
		return f
	}

	// Two POST endpoints on different routes — the SvelteKit route-handler shape.
	if got := FalseAlarmHint(
		mk("src/routes/conversation/+server.ts", "POST"),
		mk("src/routes/settings/+server.ts", "POST"),
	); got != "sveltekit-handler" {
		t.Errorf("two +server.ts POST handlers: got %q, want sveltekit-handler", got)
	}

	// A page load and a server load — both framework exports in +page(.server) modules.
	if got := FalseAlarmHint(
		mk("src/routes/a/+page.ts", "load"),
		mk("src/routes/b/+page.server.ts", "load"),
	); got != "sveltekit-handler" {
		t.Errorf("two load() exports: got %q, want sveltekit-handler", got)
	}

	// Only one side is a framework export — must NOT tag (the other is an ordinary
	// helper that merely happens to be named POST in a plain module).
	if got := FalseAlarmHint(
		mk("src/routes/conversation/+server.ts", "POST"),
		mk("src/lib/http.ts", "POST"),
	); got == "sveltekit-handler" {
		t.Error("only one side is a SvelteKit route module — must not tag sveltekit-handler")
	}

	// Right file, but the export name isn't a framework handler — must NOT tag.
	if got := FalseAlarmHint(
		mk("src/routes/a/+server.ts", "helper"),
		mk("src/routes/b/+server.ts", "helper"),
	); got == "sveltekit-handler" {
		t.Error("non-handler export name must not tag sveltekit-handler")
	}
}
