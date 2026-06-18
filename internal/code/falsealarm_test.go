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
