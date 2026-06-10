"""calque Python extractor — emits one FuncSig JSON record per function/method.

Invoked by the Go scorer as:  python3 -c <this> <root>   with newline-separated
file paths on stdin. Output: a JSON array of {file,qualname,name,line,n_lines,
strings,writes,ret_keys,calls,delegates} — the same interchange the go/ast
extractor produces. Extraction only: the Go side computes the name stem and
scores, so the signal logic lives in one place per concern.

Reused from the original Python calque — Python's own ast is the
robust way to parse modern Python (match, walrus, type params), which is why the
Go binary shells out here for .py targets rather than embedding a parser.
"""

import ast
import json
import os
import sys

# Wrapper attributes marking a method as forwarding to a wrapped impl
# (self._engine.step(...)) — an adapter, not a reimplementation. (Mirrors the Go
# side's delegationRoots; kept small + stable.)
_DELEGATION_ROOTS = {
    "_engine", "_impl", "_inner", "_delegate",
    "_wrapped", "_backend", "_real", "_target",
}


def _attr_path(node):
    parts = []
    cur = node
    while isinstance(cur, ast.Attribute):
        parts.append(cur.attr)
        cur = cur.value
    if not isinstance(cur, ast.Name):
        return None
    parts.reverse()
    return ".".join(parts) if parts else None


class _BodyVisitor(ast.NodeVisitor):
    def __init__(self):
        self.strings = set()
        self.writes = set()
        self.ret_keys = set()
        self.calls = set()
        self.delegates = False

    def visit_Constant(self, node):
        if isinstance(node.value, str) and len(node.value.strip()) >= 4:
            self.strings.add(node.value.strip())
        self.generic_visit(node)

    def _record_target(self, tgt):
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

    def visit_Assign(self, node):
        for t in node.targets:
            self._record_target(t)
        self.generic_visit(node)

    def visit_AnnAssign(self, node):
        self._record_target(node.target)
        self.generic_visit(node)

    def visit_AugAssign(self, node):
        self._record_target(node.target)
        self.generic_visit(node)

    def visit_Return(self, node):
        if isinstance(node.value, ast.Dict):
            for k in node.value.keys:
                if isinstance(k, ast.Constant) and isinstance(k.value, str):
                    self.ret_keys.add(k.value)
        self.generic_visit(node)

    def visit_Call(self, node):
        f = node.func
        if isinstance(f, ast.Attribute):
            self.calls.add(f.attr)
            p = _attr_path(f)
            if p and p.split(".")[0] in _DELEGATION_ROOTS:
                self.delegates = True
        elif isinstance(f, ast.Name):
            self.calls.add(f.id)
        self.generic_visit(node)


def _extract(path, root):
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            tree = ast.parse(fh.read())
    except (SyntaxError, OSError, ValueError):
        return []
    try:
        rel = os.path.relpath(path, root)
    except ValueError:
        rel = path
    out = []

    def walk(node, prefix):
        for child in ast.iter_child_nodes(node):
            if isinstance(child, ast.ClassDef):
                walk(child, prefix + child.name + ".")
            elif isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                bv = _BodyVisitor()
                for stmt in child.body:
                    bv.visit(stmt)
                end = getattr(child, "end_lineno", child.lineno) or child.lineno
                out.append({
                    "file": rel,
                    "qualname": prefix + child.name,
                    "name": child.name,
                    "line": child.lineno,
                    "n_lines": end - child.lineno + 1,
                    "strings": sorted(bv.strings),
                    "writes": sorted(bv.writes),
                    "ret_keys": sorted(bv.ret_keys),
                    "calls": sorted(bv.calls),
                    "delegates": bv.delegates,
                })
                # nested defs are usually closures — skip

    walk(tree, "")
    return out


def _string_keys(node):
    """For a dict literal: its string-constant keys (+ string-constant values).
    For a set/list/tuple literal: its string-constant elements. The cross-substrate
    axis treats these key SETS as the entity's footprint (-> ret_keys)."""
    keys, vals = [], []
    if isinstance(node, ast.Dict):
        for k, v in zip(node.keys, node.values):
            if isinstance(k, ast.Constant) and isinstance(k.value, str):
                keys.append(k.value)
                if isinstance(v, ast.Constant) and isinstance(v.value, str):
                    vals.append(v.value)
    elif isinstance(node, (ast.Set, ast.List, ast.Tuple)):
        for e in node.elts:
            if isinstance(e, ast.Constant) and isinstance(e.value, str):
                keys.append(e.value)
    return keys, vals


def _extract_symbols(path, root, min_keys=3, max_keys=400):
    """Emit one 'table' FuncSig record per MODULE-LEVEL dict/set/list constant
    (the cross-substrate axis's non-function entity). ret_keys = the key set; the
    judge then pairs e.g. engine.py::HANDLERS with input_agent.py::_VERB_TEMPLATES."""
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            tree = ast.parse(fh.read())
    except (SyntaxError, OSError, ValueError):
        return []
    try:
        rel = os.path.relpath(path, root)
    except ValueError:
        rel = path
    out = []
    for node in tree.body:  # module level only — not nested in funcs/classes
        if isinstance(node, ast.Assign):
            targets = [t for t in node.targets if isinstance(t, ast.Name)]
            value = node.value
        elif isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
            targets, value = [node.target], node.value
        else:
            continue
        if not targets or value is None:
            continue
        keys, vals = _string_keys(value)
        if not keys:
            continue
        end = getattr(node, "end_lineno", node.lineno) or node.lineno
        for tgt in targets:
            name = tgt.id
            # Noise control: a SCREAMING_SNAKE named table, or any map with enough
            # keys to be a real registry (vs an incidental 1-2 element literal).
            if not (name.isupper() or len(keys) >= min_keys):
                continue
            out.append({
                "file": rel,
                "qualname": name,
                "name": name,
                "kind": "table",
                "line": node.lineno,
                "n_lines": end - node.lineno + 1,
                "strings": sorted(set(vals))[:max_keys],
                "writes": [],
                "ret_keys": sorted(set(keys))[:max_keys],
                "calls": [],
                "delegates": False,
            })
    return out


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    mode = sys.argv[2] if len(sys.argv) > 2 else "functions"
    extract = _extract_symbols if mode == "symbols" else _extract
    paths = sys.stdin.read().split()
    out = []
    for p in paths:
        out.extend(extract(p, root))
    json.dump(out, sys.stdout)


main()
