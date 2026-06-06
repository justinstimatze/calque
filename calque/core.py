"""calque.core -- extract divergence-robust signatures from Python functions and
rank cross-boundary pairs by how much they smell like the *same contract*.

This is the RECALL half of calque. It is deliberately dumb and high-recall: its
job is to hand a short ranked list of suspect pairs to an LLM oracle (the agent),
which makes the actual equivalence call. It never claims two functions ARE
equivalent -- only that they share enough contract-invariant signal to be worth a
human/agent's judgment.

Why these signals: a dual path (two implementations of one contract that have
drifted) is dissimilar *by construction* in token/AST shape -- that is the bug.
So we index the things that stay invariant when the body is rewritten:

  - emitted string literals   (what the function SAYS -- surface output)
  - attribute write-targets   (what state it MUTATES -- effect signature)
  - returned dict keys         (the shape of what it HANDS BACK)
  - called function names      (what downstream it leans on)
  - name stem                  (the ROLE -- names track role even when bodies don't)

None of these care that the bodies look nothing alike. That is the whole point.
"""

from __future__ import annotations

import ast
import re
from dataclasses import dataclass, field
from pathlib import Path

# Role-prefixes stripped when normalizing a name to its "stem" (the contract it
# fills). `_handle_leave_town` and `leave_town` must collapse to the same stem.
_ROLE_PREFIXES = (
    "handle",
    "do",
    "try",
    "resolve",
    "check",
    "run",
    "apply",
    "process",
    "on",
    "maybe",
    "build",
    "make",
    "get",
    "compute",
)
_STOPWORDS = frozenset({"the", "a", "an", "to", "for", "of", "and", "or"})

# Attribute names that signal a method *forwards to a wrapped implementation*
# rather than reimplementing it (the adapter pattern: `self._engine.step(...)`).
# A forwarding method is NAMED AFTER what it wraps, so its name-stem match against
# the real method is a guaranteed false positive -- stope's first run was 21/30
# such adapters. We don't drop them (they can still drift in the bit of glue they
# DO own), but a name match alone can no longer anchor a delegating pair. Extend
# per-repo as needed; these cover the common wrapper-attribute names.
_DELEGATION_ROOTS = frozenset(
    {"_engine", "_impl", "_inner", "_delegate", "_wrapped", "_backend", "_real", "_target"}
)


def _norm_tokens(name: str) -> list[str]:
    """Lowercase token list of a name: strip leading underscores, camelCase->snake,
    split on underscores."""
    n = name.strip().lstrip("_")
    n = re.sub(r"(?<=[a-z0-9])(?=[A-Z])", "_", n).lower()
    return [t for t in n.split("_") if t]


def _stem_tokens(name: str) -> frozenset[str]:
    """Role tokens of a function name: strip leading underscores + role prefixes,
    split on underscores/camelCase, drop stopwords."""
    toks = _norm_tokens(name)
    # peel one leading role prefix (handle_leave_town -> leave_town)
    while toks and toks[0] in _ROLE_PREFIXES and len(toks) > 1:
        toks = toks[1:]
    return frozenset(t for t in toks if t and t not in _STOPWORDS)


def _role_prefix(name: str) -> str | None:
    """The single leading role-prefix of a name (`_handle_leave_town` -> 'handle'),
    or None. Mirrors the peel in `_stem_tokens` so 'twinned roles' can be learned
    from the pairs that matched on a boundary."""
    toks = _norm_tokens(name)
    if toks and toks[0] in _ROLE_PREFIXES and len(toks) > 1:
        return toks[0]
    return None


@dataclass(frozen=True)
class FuncSig:
    file: str
    qualname: str  # "Class.method" or "func"
    name: str
    lineno: int
    n_lines: int
    strings: frozenset[str]
    writes: frozenset[str]  # dotted attribute targets, e.g. "ctx.pending_confirm"
    ret_keys: frozenset[str]  # keys of any returned dict literal
    calls: frozenset[str]  # called function/method leaf names
    stem: frozenset[str] = field(default_factory=frozenset)
    delegates: bool = False  # body forwards to a wrapped impl (self._engine.*)

    @property
    def key(self) -> str:
        return f"{self.file}::{self.qualname}"


def _attr_path(node: ast.AST) -> str | None:
    """Dotted suffix of an attribute chain, dropping the root identifier.
    self.ctx.pending_confirm -> 'ctx.pending_confirm'; self.x -> 'x'."""
    parts: list[str] = []
    cur = node
    while isinstance(cur, ast.Attribute):
        parts.append(cur.attr)
        cur = cur.value
    if not isinstance(cur, ast.Name):
        return None
    parts.reverse()
    return ".".join(parts) if parts else None


class _BodyVisitor(ast.NodeVisitor):
    def __init__(self) -> None:
        self.strings: set[str] = set()
        self.writes: set[str] = set()
        self.ret_keys: set[str] = set()
        self.calls: set[str] = set()
        self.delegates: bool = False

    def visit_Constant(self, node: ast.Constant) -> None:
        if isinstance(node.value, str) and len(node.value.strip()) >= 4:
            self.strings.add(node.value.strip())
        self.generic_visit(node)

    def _record_target(self, tgt: ast.AST) -> None:
        if isinstance(tgt, ast.Attribute):
            p = _attr_path(tgt)
            if p:
                self.writes.add(p)
        elif isinstance(tgt, ast.Subscript):
            p = _attr_path(tgt.value)
            if p:
                self.writes.add(p + "[]")
        elif isinstance(tgt, (ast.Tuple, ast.List)):
            for e in tgt.elts:
                self._record_target(e)

    def visit_Assign(self, node: ast.Assign) -> None:
        for t in node.targets:
            self._record_target(t)
        self.generic_visit(node)

    def visit_AnnAssign(self, node: ast.AnnAssign) -> None:
        self._record_target(node.target)
        self.generic_visit(node)

    def visit_AugAssign(self, node: ast.AugAssign) -> None:
        self._record_target(node.target)
        self.generic_visit(node)

    def visit_Return(self, node: ast.Return) -> None:
        if isinstance(node.value, ast.Dict):
            for k in node.value.keys:
                if isinstance(k, ast.Constant) and isinstance(k.value, str):
                    self.ret_keys.add(k.value)
        self.generic_visit(node)

    def visit_Call(self, node: ast.Call) -> None:
        f = node.func
        if isinstance(f, ast.Attribute):
            self.calls.add(f.attr)
            # Forwarding to a wrapped impl (self._engine.step(...)) marks this as
            # an adapter, not a reimplementation -- recorded so scoring can stop
            # the wrapper's name match from being a false-positive anchor.
            p = _attr_path(f)
            if p and p.split(".")[0] in _DELEGATION_ROOTS:
                self.delegates = True
        elif isinstance(f, ast.Name):
            self.calls.add(f.id)
        self.generic_visit(node)


def extract_file(path: Path, root: Path) -> list[FuncSig]:
    """All function/method signatures in one Python file."""
    try:
        tree = ast.parse(path.read_text(encoding="utf-8", errors="replace"))
    except SyntaxError:
        return []
    rel = str(path.relative_to(root)) if path.is_relative_to(root) else str(path)
    out: list[FuncSig] = []

    def walk(node: ast.AST, prefix: str) -> None:
        for child in ast.iter_child_nodes(node):
            if isinstance(child, ast.ClassDef):
                walk(child, f"{prefix}{child.name}.")
            elif isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                bv = _BodyVisitor()
                for stmt in child.body:
                    bv.visit(stmt)
                end = getattr(child, "end_lineno", child.lineno) or child.lineno
                out.append(
                    FuncSig(
                        file=rel,
                        qualname=f"{prefix}{child.name}",
                        name=child.name,
                        lineno=child.lineno,
                        n_lines=end - child.lineno + 1,
                        strings=frozenset(bv.strings),
                        writes=frozenset(bv.writes),
                        ret_keys=frozenset(bv.ret_keys),
                        calls=frozenset(bv.calls),
                        stem=_stem_tokens(child.name),
                        delegates=bv.delegates,
                    )
                )
                # nested defs inside this function are usually closures -- skip

    walk(tree, "")
    return out


def _jaccard(a: frozenset[str], b: frozenset[str]) -> float:
    if not a and not b:
        return 0.0
    inter = len(a & b)
    union = len(a | b)
    return inter / union if union else 0.0


# Signal weights. Surface (strings) + effect (writes) + role (name) carry the most
# because they survive a full rewrite; calls/ret_keys are corroborating.
_WEIGHTS = {"strings": 0.30, "writes": 0.30, "name": 0.22, "calls": 0.10, "ret": 0.08}


@dataclass(frozen=True)
class Suspicion:
    left: FuncSig
    right: FuncSig
    score: float
    signals: dict[str, float]

    def reason(self) -> str:
        fired = sorted(
            ((k, v) for k, v in self.signals.items() if v > 0),
            key=lambda kv: -kv[1],
        )
        bits = []
        for k, v in fired:
            if k == "name":
                shared = sorted(self.left.stem & self.right.stem)
                bits.append(f"name~{v:.2f}({'+'.join(shared)})")
            elif k == "strings":
                n = len(self.left.strings & self.right.strings)
                bits.append(f"shared-strings={n}")
            elif k == "writes":
                w = sorted(self.left.writes & self.right.writes)
                bits.append(f"shared-writes={w}")
            elif k == "ret":
                r = sorted(self.left.ret_keys & self.right.ret_keys)
                bits.append(f"shared-ret-keys={r}")
            elif k == "calls":
                n = len(self.left.calls & self.right.calls)
                bits.append(f"shared-calls={n}")
        return "; ".join(bits)


def score_pair(a: FuncSig, b: FuncSig) -> Suspicion | None:
    # When either side forwards to a wrapped impl, the adapter is named after what
    # it wraps -- so a name match is a guaranteed false positive. Keep a sliver of
    # weight (still weak evidence) but bar name from anchoring the pair on its own.
    delegating = a.delegates or b.delegates
    name_raw = 1.0 if a.stem and a.stem == b.stem else _jaccard(a.stem, b.stem)
    name = name_raw * 0.2 if delegating else name_raw
    sig = {
        "strings": _jaccard(a.strings, b.strings),
        "writes": _jaccard(a.writes, b.writes),
        "name": name,
        "calls": _jaccard(a.calls, b.calls),
        "ret": _jaccard(a.ret_keys, b.ret_keys),
    }
    # Renormalize over signals that are *available* (both sides have data), so a
    # pair isn't penalized for, e.g., neither emitting strings.
    avail = {
        "strings": bool(a.strings or b.strings),
        "writes": bool(a.writes or b.writes),
        "name": bool(a.stem or b.stem),
        "calls": bool(a.calls or b.calls),
        "ret": bool(a.ret_keys or b.ret_keys),
    }
    wsum = sum(_WEIGHTS[k] for k, ok in avail.items() if ok) or 1.0
    score = sum(_WEIGHTS[k] * sig[k] for k, ok in avail.items() if ok) / wsum

    # Gate out noise: require a real role-overlap OR a concrete surface/effect
    # overlap. A pair that only shares generic call names is junk. For a
    # delegating pair, name can't be the anchor (it's named after what it wraps),
    # so a pure forwarder with no shared surface/effect drops out entirely.
    has_anchor = (
        (name >= 0.34 and not delegating)
        or sig["strings"] > 0
        or sig["writes"] > 0
        or sig["ret"] > 0
    )
    if not has_anchor:
        return None
    return Suspicion(left=a, right=b, score=score, signals=sig)


def rank(
    left: list[FuncSig],
    right: list[FuncSig],
    *,
    min_lines: int = 4,
    min_score: float = 0.18,
    top: int = 30,
) -> list[Suspicion]:
    """Score every left x right pair; return the top suspects."""
    L = [f for f in left if f.n_lines >= min_lines and not f.name.startswith("__")]
    R = [f for f in right if f.n_lines >= min_lines and not f.name.startswith("__")]
    out: list[Suspicion] = []
    for a in L:
        for b in R:
            if a.key == b.key:
                continue
            s = score_pair(a, b)
            if s and s.score >= min_score:
                out.append(s)
    out.sort(key=lambda s: -s.score)
    # Deduplicate: keep each left function's single best match, then each right's,
    # so the report isn't dominated by one promiscuous function.
    seen_pairs: set[tuple[str, str]] = set()
    deduped: list[Suspicion] = []
    for s in out:
        pk = (s.left.key, s.right.key)
        if pk in seen_pairs:
            continue
        seen_pairs.add(pk)
        deduped.append(s)
    return deduped[:top]


def missing_twins(
    left: list[FuncSig],
    right: list[FuncSig],
    *,
    min_lines: int = 4,
    min_score: float = 0.18,
) -> list[FuncSig]:
    """Left-boundary functions that *look* like they should have a twin but have
    NONE on the right side -- the 'missing twin' case pair-ranking is structurally
    blind to (a contract whose counterpart was never written, or was deleted,
    produces zero pairs and falls off `rank()` entirely).

    Generalized from stope's `maturity_check._check_dual_path_drift`, which flags
    `_handle_X` handlers absent from the test side. Instead of hardcoding one
    prefix, we *learn* which role prefixes are twinned on this boundary (those
    that produced a real match) and report only the gaps within those roles -- so
    engine-internal helpers (`_check_*`, `_build_*` with no counterpart by design)
    don't flood the list. A recall aid, not a verdict, exactly like `rank()`.
    """
    R = [f for f in right if f.n_lines >= min_lines and not f.name.startswith("__")]
    L = [f for f in left if f.n_lines >= min_lines and not f.name.startswith("__")]
    matched: set[str] = set()
    twinned: set[str] = set()  # role prefixes that have at least one real twin here
    for a in L:
        pfx = _role_prefix(a.name)
        for b in R:
            if a.key == b.key:
                continue
            s = score_pair(a, b)
            if s and s.score >= min_score:
                matched.add(a.key)
                if pfx:
                    twinned.add(pfx)
                break
    return [
        a
        for a in L
        if a.key not in matched
        and _role_prefix(a.name) in twinned
        and (a.strings or a.writes or a.ret_keys)  # carries real contract signal
    ]
