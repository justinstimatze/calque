// calque TypeScript extractor — emits one FuncSig JSON record per function/method.
//
// Invoked by the Go scorer as:  node extract_ts.mjs <root>   with newline-separated
// file paths on stdin. Output: a JSON array of {file,qualname,name,line,n_lines,
// strings,writes,ret_keys,calls,delegates} — the same interchange the go/ast and
// python3 extractors produce. Extraction only: the Go side computes the name stem
// and scores, so the signal logic lives in one place per concern.
//
// TypeScript's own compiler API is the robust way to parse modern TS/TSX (generics,
// decorators, JSX), which is why the Go binary shells out here for .ts/.tsx targets
// rather than embedding a parser — mirroring the python3 path for .py.

import { createRequire } from 'node:module';
import * as path from 'node:path';

// Wrapper roots marking a call as forwarding to a wrapped impl (this._engine.step())
// — an adapter, not a reimplementation. Mirrors extract.py's _DELEGATION_ROOTS and
// the Go side's delegationRoots; kept small + stable.
const DELEGATION_ROOTS = new Set([
  '_engine', '_impl', '_inner', '_delegate',
  '_wrapped', '_backend', '_real', '_target',
]);

// resolveTypescript finds the `typescript` module without bundling it (it's large
// and version-sensitive): an explicit CALQUE_TS path wins, then the SCANNED repo's
// node_modules (a TS project almost always depends on typescript), then a global /
// NODE_PATH install. A clear error otherwise — never a silent half-parse.
function resolveTypescript(root) {
  const req = createRequire(import.meta.url);
  const tryPaths = [];
  if (process.env.CALQUE_TS) {
    // May be the package dir or its parent node_modules — try both.
    tryPaths.push(process.env.CALQUE_TS);
  }
  // require.resolve with explicit paths searches each dir's node_modules upward.
  try {
    const resolved = req.resolve('typescript', { paths: [path.resolve(root), process.cwd()] });
    return req(resolved);
  } catch { /* fall through */ }
  for (const p of tryPaths) {
    try { return req(p); } catch { /* try next */ }
    try { return req(path.join(p, 'node_modules', 'typescript')); } catch { /* try next */ }
  }
  try { return req('typescript'); } catch { /* fall through */ }
  process.stderr.write(
    'calque: TypeScript compiler not found. Install it (npm i -g typescript), run ' +
    '`npm install` in the scanned repo, or set CALQUE_TS to a typescript module path.\n');
  process.exit(3);
}

function main() {
  const root = process.argv[2] || '.';
  const ts = resolveTypescript(root);

  const input = readStdin();
  const paths = input.split('\n').map((s) => s.trim()).filter(Boolean);

  const out = [];
  for (const p of paths) {
    try {
      out.push(...extractFile(ts, p, root));
    } catch {
      // A single unparseable file must not abort the batch (mirrors extract.py's
      // per-file try/except) — skip it and keep going.
    }
  }
  process.stdout.write(JSON.stringify(out));
}

function readStdin() {
  try {
    return require_fs().readFileSync(0, 'utf8');
  } catch {
    return '';
  }
}

function require_fs() {
  const req = createRequire(import.meta.url);
  return req('node:fs');
}

function extractFile(ts, filePath, root) {
  const fs = require_fs();
  let src;
  try {
    src = fs.readFileSync(filePath, 'utf8');
  } catch {
    return [];
  }
  const isTSX = filePath.endsWith('.tsx') || filePath.endsWith('.jsx');
  const sf = ts.createSourceFile(
    filePath, src, ts.ScriptTarget.Latest, /*setParentNodes*/ true,
    isTSX ? ts.ScriptKind.TSX : ts.ScriptKind.TS);

  let rel;
  try {
    rel = path.relative(root, filePath) || filePath;
  } catch {
    rel = filePath;
  }

  const out = [];

  // line is 1-based to match go/ast + python ast.
  const lineOf = (pos) => sf.getLineAndCharacterOfPosition(pos).line + 1;
  const nLines = (node) => lineOf(node.getEnd()) - lineOf(node.getStart(sf)) + 1;

  const emit = (node, qualname, name) => {
    const body = node.body;
    if (!body) return; // overload signature / ambient decl — no body to scan
    const bv = new BodyVisitor(ts);
    bv.visitBody(body);
    out.push({
      file: rel,
      qualname,
      name,
      line: lineOf(node.getStart(sf)),
      n_lines: nLines(node),
      strings: bv.sorted(bv.strings),
      writes: bv.sorted(bv.writes),
      ret_keys: bv.sorted(bv.retKeys),
      calls: bv.sorted(bv.calls),
      delegates: bv.delegates,
    });
  };

  // Walk top-level + one class level deep, mirroring extract.py (classes prefix
  // qualname; nested functions are usually closures and are skipped).
  const walk = (node, prefix) => {
    ts.forEachChild(node, (child) => {
      if (ts.isClassDeclaration(child) || ts.isClassExpression(child)) {
        const cname = child.name ? child.name.text : '(anon)';
        for (const m of child.members) {
          if ((ts.isMethodDeclaration(m) || ts.isConstructorDeclaration(m) ||
               ts.isGetAccessorDeclaration(m) || ts.isSetAccessorDeclaration(m)) && m.body) {
            const mname = memberName(ts, m);
            emit(m, prefix + cname + '.' + mname, mname);
          }
        }
      } else if (ts.isFunctionDeclaration(child) && child.name) {
        emit(child, prefix + child.name.text, child.name.text);
      } else if (ts.isVariableStatement(child)) {
        // const f = (…) => {…}  /  const f = function(){…}
        for (const decl of child.declarationList.declarations) {
          if (ts.isIdentifier(decl.name) && decl.initializer &&
              (ts.isArrowFunction(decl.initializer) || ts.isFunctionExpression(decl.initializer)) &&
              decl.initializer.body) {
            emit(decl.initializer, prefix + decl.name.text, decl.name.text);
          }
        }
      }
    });
  };

  walk(sf, '');
  return out;
}

function memberName(ts, m) {
  if (ts.isConstructorDeclaration(m)) return 'constructor';
  const n = m.name;
  if (!n) return '(anon)';
  if (ts.isIdentifier(n) || ts.isPrivateIdentifier(n)) return n.text;
  if (ts.isStringLiteral(n) || ts.isNumericLiteral(n)) return n.text;
  return n.getText ? n.getText() : '(computed)';
}

// BodyVisitor accumulates the four signal channels over a function body, recursing
// into all descendants EXCEPT nested function/class bodies (their statements belong
// to a different FuncSig).
class BodyVisitor {
  constructor(ts) {
    this.ts = ts;
    this.strings = new Set();
    this.writes = new Set();
    this.retKeys = new Set();
    this.calls = new Set();
    this.delegates = false;
  }

  sorted(s) { return [...s].sort(); }

  visitBody(body) {
    const ts = this.ts;
    if (ts.isBlock(body)) {
      for (const stmt of body.statements) this.visit(stmt);
    } else {
      // Arrow with an expression body: const f = () => ({a:1})
      this.visit(body);
    }
  }

  visit(node) {
    const ts = this.ts;

    // String literals (≥4 chars trimmed) — includes un-substituted templates.
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      const v = (node.text || '').trim();
      if (v.length >= 4) this.strings.add(v);
    }

    // Assignment write targets:  a.b.c = …   /   a.b[i] = …
    if (ts.isBinaryExpression(node) && isAssignmentOp(ts, node.operatorToken.kind)) {
      this.recordTarget(node.left);
    }

    // return { … }  → ret_keys
    if (ts.isReturnStatement(node) && node.expression && ts.isObjectLiteralExpression(node.expression)) {
      for (const prop of node.expression.properties) {
        const key = propKey(ts, prop);
        if (key != null) this.retKeys.add(key);
      }
    }

    // Calls → leaf name; delegation when the call's root object is a wrapper field.
    if (ts.isCallExpression(node)) {
      const callee = node.expression;
      if (ts.isPropertyAccessExpression(callee)) {
        this.calls.add(callee.name.text);
        const rootName = accessRoot(ts, callee);
        if (rootName && DELEGATION_ROOTS.has(rootName)) this.delegates = true;
      } else if (ts.isIdentifier(callee)) {
        this.calls.add(callee.text);
      }
    }

    // Recurse, but NOT into nested function/class bodies.
    if (isFunctionLike(ts, node) || ts.isClassDeclaration(node) || ts.isClassExpression(node)) {
      return;
    }
    ts.forEachChild(node, (c) => this.visit(c));
  }

  recordTarget(target) {
    const ts = this.ts;
    if (ts.isPropertyAccessExpression(target)) {
      const p = accessPath(ts, target);
      if (p) this.writes.add(p);
    } else if (ts.isElementAccessExpression(target)) {
      const p = accessPath(ts, target.expression);
      if (p) this.writes.add(p + '[]');
    }
  }
}

function isAssignmentOp(ts, kind) {
  // = and all compound assignments (+=, &&=, ??=, …) live between the first/last
  // CompoundAssignment tokens; EqualsToken is plain `=`.
  return kind === ts.SyntaxKind.EqualsToken ||
    (kind >= ts.SyntaxKind.FirstCompoundAssignment && kind <= ts.SyntaxKind.LastCompoundAssignment);
}

function isFunctionLike(ts, node) {
  return ts.isFunctionDeclaration(node) || ts.isFunctionExpression(node) ||
    ts.isArrowFunction(node) || ts.isMethodDeclaration(node) ||
    ts.isConstructorDeclaration(node) || ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node);
}

// accessPath turns a.b.c into "a.b.c" (dotted), or null if it bottoms out on a
// non-identifier (e.g. a call result). Mirrors extract.py's _attr_path.
function accessPath(ts, node) {
  const parts = [];
  let cur = node;
  while (ts.isPropertyAccessExpression(cur)) {
    parts.push(cur.name.text);
    cur = cur.expression;
  }
  if (cur.kind === ts.SyntaxKind.ThisKeyword) {
    parts.push('this');
  } else if (ts.isIdentifier(cur)) {
    parts.push(cur.text);
  } else {
    return null;
  }
  parts.reverse();
  return parts.join('.');
}

// accessRoot returns the delegation-root segment of a property-access chain: the
// first non-`this` identifier. For this._engine.foo() it returns "_engine"; for
// _backend.run() it returns "_backend" — so the delegation check sees the wrapper
// field whether or not the call goes through `this`.
function accessRoot(ts, node) {
  const path = accessPath(ts, node);
  if (!path) return null;
  const segs = path.split('.');
  if (segs[0] === 'this' && segs.length > 1) return segs[1];
  return segs[0];
}

function propKey(ts, prop) {
  if (ts.isPropertyAssignment(prop) || ts.isShorthandPropertyAssignment(prop) ||
      ts.isMethodDeclaration(prop)) {
    const n = prop.name;
    if (!n) return null;
    if (ts.isIdentifier(n)) return n.text;
    if (ts.isStringLiteral(n) || ts.isNumericLiteral(n)) return n.text;
  }
  return null;
}

main();
