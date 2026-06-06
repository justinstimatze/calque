"""calque core smoke tests -- the ranker must catch a planted behavioral twin and
ignore an unrelated pair."""

from __future__ import annotations

from pathlib import Path

from calque.core import extract_file, missing_twins, rank, _stem_tokens


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


def test_pure_delegator_named_after_engine_not_flagged(tmp_path: Path):
    """A harness adapter that just forwards to self._engine.<x>(...) is NAMED
    after what it wraps, so name alone matches the engine method — but it can't
    drift. With only a name match (no shared surface/effect) it must be gated out;
    a genuine reimplementation sharing emitted strings must still surface. This is
    the fix for the 21/30 false-positive adapters in stope's first run."""
    _write(
        tmp_path,
        "engine.py",
        "class E:\n"
        "    def _handle_radio(self, cmd):\n"
        "        self.world.station = cmd\n"
        "        self.shell.emit_text('static crackles over the band')\n"
        "        self.shell.emit_text('a voice resolves out of the noise')\n"
        "        return {'ok': True}\n"
        "    def _handle_dos(self, cmd):\n"
        "        self.world.terminal = cmd\n"
        "        self.shell.emit_text('the CRT flickers green')\n"
        "        return {'ok': True}\n",
    )
    _write(
        tmp_path,
        "testing.py",
        "class T:\n"
        "    def radio(self, cmd):\n"  # PURE delegator -> must NOT be flagged vs _handle_radio
        "        return self._engine.step(f'radio {cmd}')\n"
        "    def dos(self, cmd):\n"  # genuine reimpl sharing strings -> MUST surface
        "        self.world.terminal = cmd\n"
        "        self.shell.emit_text('the CRT flickers green')\n"
        "        return {'ok': True}\n",
    )
    left = extract_file(tmp_path / "engine.py", tmp_path)
    right = extract_file(tmp_path / "testing.py", tmp_path)
    suspects = rank(left, right, min_lines=1, min_score=0.18, top=20)
    pairs = {(s.left.name, s.right.name) for s in suspects}
    assert ("_handle_radio", "radio") not in pairs, f"pure delegator flagged: {pairs}"
    assert ("_handle_dos", "dos") in pairs, f"genuine reimpl missed: {pairs}"


def test_missing_twin_reported(tmp_path: Path):
    """A role-named left fn whose role IS twinned on the boundary but which has no
    right counterpart is reported; a same-role fn that HAS a twin, and an engine
    helper with no role prefix, are both excluded -- the lift from maturity_check,
    generalized so engine internals don't flood the report."""
    _write(
        tmp_path,
        "engine.py",
        "class E:\n"
        "    def _handle_alpha(self):\n"
        "        self.world.alpha_done = True\n"
        "        self.shell.emit_text('alpha authored narrative line one')\n"
        "        return {'status': 'alpha'}\n"
        "    def _handle_beta(self):\n"  # role 'handle' is twinned (via alpha), no right twin
        "        self.ctx.beta_count += 1\n"
        "        self.audio.play('beta-cue-sound')\n"
        "        return {'result': 'beta'}\n"
        "    def _internal_warm(self):\n"  # no role prefix -> engine-internal, excluded
        "        self.world.cache_built = True\n"
        "        return {'warmed': True}\n",
    )
    _write(
        tmp_path,
        "testing.py",
        "class T:\n"
        "    def alpha(self):\n"
        "        self.world.alpha_done = True\n"
        "        self.shell.emit_text('alpha authored narrative line one')\n"
        "        return {'status': 'alpha'}\n",
    )
    left = extract_file(tmp_path / "engine.py", tmp_path)
    right = extract_file(tmp_path / "testing.py", tmp_path)
    missing = missing_twins(left, right, min_lines=1, min_score=0.18)
    names = {f.name for f in missing}
    assert "_handle_beta" in names, names  # twinned role, no counterpart
    assert "_handle_alpha" not in names  # it has a twin
    assert "_internal_warm" not in names  # no role prefix -> not a missing-twin candidate
