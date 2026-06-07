"""calque CLI -- scan a repo for dual-path suspects across a boundary.

    python -m calque scan --repo <path> --left "engine*.py" --right "testing.py"

Emits a ranked markdown report of suspect pairs for an agent/human to adjudicate.
The tool supplies recall; you supply the verdict; the registry holds the memory.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import core
from .core import FuncSig, extract_file, missing_twins, rank


def _collect(repo: Path, globs: list[str]) -> list[FuncSig]:
    files: list[Path] = []
    for g in globs:
        files.extend(sorted(repo.glob(g)))
    sigs: list[FuncSig] = []
    seen: set[Path] = set()
    for f in files:
        if f.suffix == ".py" and f not in seen:
            seen.add(f)
            sigs.extend(extract_file(f, repo))
    return sigs


def _report(suspects: list, left_globs: list[str], right_globs: list[str]) -> str:
    lines = [
        "# calque -- dual-path suspects",
        "",
        f"boundary: `{' '.join(left_globs)}`  ×  `{' '.join(right_globs)}`",
        f"suspect pairs: {len(suspects)}",
        "",
        "Each row is a candidate, ranked by contract-invariant signal overlap. "
        "calque is recall-only -- adjudicate each as drift / contracted-twin-ok / "
        "false-alarm, then record the verdict in the registry.",
        "",
    ]
    for i, s in enumerate(suspects, 1):
        lines.append(
            f"## {i}. {s.score:.2f}  "
            f"`{s.left.qualname}` ({s.left.file}:{s.left.lineno})  ≟  "
            f"`{s.right.qualname}` ({s.right.file}:{s.right.lineno})"
        )
        lines.append(f"- {s.reason()}")
        lines.append("")
    return "\n".join(lines)


def _missing_report(missing: list[FuncSig]) -> str:
    """Section for left-side contracts with no right-side twin at all."""
    if not missing:
        return ""
    lines = [
        "",
        "## missing twins (left contracts with no right-side counterpart)",
        "",
        "Role-named left functions (handler/resolver/etc.) whose role IS twinned "
        "elsewhere on this boundary, but which have NO match on the right side -- "
        "a counterpart that was never written or was deleted. Pair-ranking can't "
        "surface these. Verify each: is a right-side twin expected?",
        "",
    ]
    for f in sorted(missing, key=lambda f: (f.file, f.lineno)):
        lines.append(f"- `{f.qualname}` ({f.file}:{f.lineno})")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="calque")
    sub = p.add_subparsers(dest="cmd", required=True)
    sc = sub.add_parser("scan", help="rank dual-path suspects across a boundary")
    sc.add_argument("--repo", required=True, type=Path)
    sc.add_argument("--left", nargs="+", required=True, help="glob(s) for the A side")
    sc.add_argument("--right", nargs="+", required=True, help="glob(s) for the B side")
    sc.add_argument("--top", type=int, default=30)
    sc.add_argument("--min-score", type=float, default=0.18)
    sc.add_argument("--min-lines", type=int, default=4)
    sc.add_argument(
        "--missing",
        action="store_true",
        help="also report role-named left fns with no right-side twin (coverage gaps)",
    )
    sc.add_argument(
        "--missing-corpus",
        nargs="+",
        metavar="GLOB",
        help="glob(s) of usage/test files; their command-string vocabulary gates "
        "out missing-twins reachable via a generic dispatcher (e.g. step('verb'))",
    )
    sc.add_argument(
        "--role-prefixes",
        help="comma-separated extra name role-prefixes to strip (project-specific; "
        "extends the built-in handle/resolve/check/... set)",
    )
    sc.add_argument(
        "--delegation-roots",
        help="comma-separated extra attribute roots that mark a forwarding adapter "
        "(extends the built-in _engine/_impl/... set, e.g. _harness)",
    )
    sc.add_argument(
        "--dispatchers",
        help="comma-separated extra generic-dispatcher method names for the "
        "--missing-corpus reachability gate (extends step/do/run/... e.g. play)",
    )
    sc.add_argument("--out", type=Path, help="write markdown report here")
    args = p.parse_args(argv)

    repo = args.repo.expanduser().resolve()
    if not repo.is_dir():
        print(f"calque: --repo not a directory: {repo}", file=sys.stderr)
        return 2

    # Per-project config: extend the (stope-shaped) defaults before extraction so
    # calque generalizes to other naming conventions / wrapper attributes.
    if args.role_prefixes:
        core._ROLE_PREFIXES = core._ROLE_PREFIXES + tuple(
            t.strip().lower() for t in args.role_prefixes.split(",") if t.strip()
        )
    if args.delegation_roots:
        core._DELEGATION_ROOTS = core._DELEGATION_ROOTS | {
            t.strip() for t in args.delegation_roots.split(",") if t.strip()
        }

    left = _collect(repo, args.left)
    right = _collect(repo, args.right)
    if not left or not right:
        print(
            f"calque: no functions found (left={len(left)}, right={len(right)}). "
            "Check globs are relative to --repo.",
            file=sys.stderr,
        )
        return 1

    suspects = rank(
        left,
        right,
        min_lines=args.min_lines,
        min_score=args.min_score,
        top=args.top,
    )
    reachable: frozenset[str] = frozenset()
    if args.missing and args.missing_corpus:
        dispatchers = core._DEFAULT_DISPATCHERS
        if args.dispatchers:
            dispatchers = dispatchers | {
                t.strip() for t in args.dispatchers.split(",") if t.strip()
            }
        terms: set[str] = set()
        seen: set[Path] = set()
        for g in args.missing_corpus:
            for f in sorted(repo.glob(g)):
                if f.suffix == ".py" and f not in seen:
                    seen.add(f)
                    terms |= core.extract_command_terms(f, dispatchers)
        reachable = frozenset(terms)
    missing = (
        missing_twins(
            left,
            right,
            min_lines=args.min_lines,
            min_score=args.min_score,
            reachable_terms=reachable,
        )
        if args.missing
        else []
    )
    report = _report(suspects, args.left, args.right) + _missing_report(missing)
    if args.out:
        args.out.write_text(report, encoding="utf-8")
        tail = f" + {len(missing)} missing twins" if args.missing else ""
        print(f"calque: {len(suspects)} suspects{tail} -> {args.out}")
    else:
        print(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
