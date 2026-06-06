"""calque CLI -- scan a repo for dual-path suspects across a boundary.

    python -m calque scan --repo <path> --left "engine*.py" --right "testing.py"

Emits a ranked markdown report of suspect pairs for an agent/human to adjudicate.
The tool supplies recall; you supply the verdict; the registry holds the memory.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from .core import FuncSig, extract_file, rank


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
    sc.add_argument("--out", type=Path, help="write markdown report here")
    args = p.parse_args(argv)

    repo = args.repo.expanduser().resolve()
    if not repo.is_dir():
        print(f"calque: --repo not a directory: {repo}", file=sys.stderr)
        return 2

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
    report = _report(suspects, args.left, args.right)
    if args.out:
        args.out.write_text(report, encoding="utf-8")
        print(f"calque: {len(suspects)} suspects -> {args.out}")
    else:
        print(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
