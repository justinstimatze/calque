"""calque core smoke tests -- the ranker must catch a planted behavioral twin and
ignore an unrelated pair."""

from __future__ import annotations

from pathlib import Path

from calque.core import extract_file, rank, _stem_tokens


def _write(tmp: Path, name: str, body: str) -> Path:
    p = tmp / name
    p.write_text(body)
    return p


def test_stem_collapses_role_prefix():
    assert _stem_tokens("_handle_leave_town") == _stem_tokens("leave_town")
    assert _stem_tokens("_resolve_confirm") == frozenset({"confirm"})


def test_ranker_flags_planted_twin(tmp_path: Path):
    # prod side: a handler that mutates ruin and emits a known line
    _write(
        tmp_path,
        "engine.py",
        "class E:\n"
        "    def _handle_leave_town(self):\n"
        "        self.world.end_reason = 'leave'\n"
        "        self.shell.emit_text('You drive. The switchbacks unwind.')\n"
        "        return {'action': 'leave', 'end_reason': 'leave_town'}\n",
    )
    # test side: a differently-shaped reimplementation of the SAME contract
    _write(
        tmp_path,
        "testing.py",
        "class T:\n"
        "    def leave_town(self):\n"
        "        outcome = 'leave'\n"
        "        self.world.end_reason = outcome\n"
        "        self.shell.emit_text('You drive. The switchbacks unwind.')\n"
        "        return {'action': 'leave', 'end_reason': 'leave_town'}\n"
        "    def unrelated_helper(self):\n"
        "        return sum(range(10))\n",
    )
    left = extract_file(tmp_path / "engine.py", tmp_path)
    right = extract_file(tmp_path / "testing.py", tmp_path)
    suspects = rank(left, right, min_lines=1, min_score=0.18, top=10)

    pairs = {(s.left.name, s.right.name) for s in suspects}
    assert ("_handle_leave_town", "leave_town") in pairs, pairs
    # the unrelated helper must NOT be flagged against the handler
    assert ("_handle_leave_town", "unrelated_helper") not in pairs, pairs


def test_collapsed_pair_scores_lower(tmp_path: Path):
    """A pair where the 'test' side just delegates shares far less surface/effect,
    so it ranks below a genuine reimplementation -- the property that makes the
    suspect list shrink as you single-path."""
    _write(
        tmp_path,
        "engine.py",
        "class E:\n"
        "    def _handle_thing(self):\n"
        "        self.world.x = 1\n"
        "        self.shell.emit_text('a long authored narrative line here')\n"
        "        self.shell.emit_text('another distinct authored sentence')\n"
        "        return {'ok': True}\n",
    )
    _write(
        tmp_path,
        "testing.py",
        "class T:\n"
        "    def thing_reimpl(self):\n"
        "        self.world.x = 1\n"
        "        self.shell.emit_text('a long authored narrative line here')\n"
        "        self.shell.emit_text('another distinct authored sentence')\n"
        "        return {'ok': True}\n"
        "    def thing_delegating(self):\n"
        "        return self._engine._handle_thing()\n",
    )
    left = extract_file(tmp_path / "engine.py", tmp_path)
    right = extract_file(tmp_path / "testing.py", tmp_path)
    suspects = rank(left, right, min_lines=1, min_score=0.0, top=10)
    by_right = {s.right.name: s.score for s in suspects}
    assert by_right["thing_reimpl"] > by_right.get("thing_delegating", 0.0)
