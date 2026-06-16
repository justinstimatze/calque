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
  const mode = process.argv[3] || 'functions';
  const ts = resolveTypescript(root);

  const input = readStdin();
  const paths = input.split('\n').map((s) => s.trim()).filter(Boolean);

  // mode "symbols" = module-level tables (the cross-substrate axis); else functions
  // (the code axis). Mirrors extract.py's mode dispatch so the .ts path matches .py.
  const extract = mode === 'symbols' ? extractSymbolsFile : extractFile;

  const out = [];
  for (const p of paths) {
    try {
      out.push(...extract(ts, p, root));
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

  // Module-scope declared SCREAMING_SNAKE constants (export const GRID = 60) — the
  // const analog of project-defined call names. The touchpoint pass gates the const
  // seam channel on these so library/global references (JSON, Math.PI) that are
  // referenced but never declared in-corpus don't form clusters. Repeated on each
  // function record; the Go side unions them across the corpus. `export const X` is
  // still a VariableStatement at module scope, so it is covered.
  const declConstSet = new Set();
  for (const stmt of sf.statements) {
    if (!ts.isVariableStatement(stmt)) continue;
    for (const decl of stmt.declarationList.declarations) {
      if (ts.isIdentifier(decl.name) && isDomainConst(decl.name.text)) {
        declConstSet.add(decl.name.text);
      }
    }
  }
  const declConsts = [...declConstSet].sort();

  // line is 1-based to match go/ast + python ast.
  const lineOf = (pos) => sf.getLineAndCharacterOfPosition(pos).line + 1;
  const nLines = (node) => lineOf(node.getEnd()) - lineOf(node.getStart(sf)) + 1;

  // signatureOf builds a normalized "(paramType,…)=>returnType" string — a
  // representation-independent invariant for the Type-4 recall pass. Declared types
  // only (no inference); an absent type is "?". Whitespace stripped so textually-
  // formatted-differently-but-identical signatures still match.
  const typeText = (t) => (t ? t.getText(sf).replace(/\s+/g, "") : "?");
  const signatureOf = (node) => {
    const params = (node.parameters || []).map((p) => typeText(p.type));
    const ret = node.type ? typeText(node.type) : "?";
    return `(${params.join(",")})=>${ret}`;
  };

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
      reads: bv.reads(),
      ret_keys: bv.sorted(bv.retKeys),
      calls: bv.sorted(bv.calls),
      consts: bv.sorted(bv.consts),
      decl_consts: declConsts,
      delegates: bv.delegates,
      sig: signatureOf(node),
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
    // readsRaw is every field-path seen in a value position; pureWrites is the
    // plain-`=` LHS. reads() returns readsRaw \ pureWrites (the derivation input
    // footprint), mirroring writes on the read side (same dotted convention).
    this.readsRaw = new Set();
    this.pureWrites = new Set();
    this.retKeys = new Set();
    this.calls = new Set();
    this.consts = new Set();
    this.delegates = false;
    // PropertyAccess nodes that are a call's callee (this.road.compute in
    // this.road.compute()); the read pass skips them so a call name does not
    // masquerade as a field read. Mirrors the Go calleeSkip map.
    this.calleeSkip = new Set();
  }

  sorted(s) { return [...s].sort(); }

  reads() { return this.sorted(new Set([...this.readsRaw].filter((r) => !this.pureWrites.has(r)))); }

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

    // Field-path reads (value positions). Plain-`=` LHS removed later via pureWrites;
    // call-receiver paths (this.road.width) appear too — acceptable recall noise. A
    // call's own callee (this.road.compute in this.road.compute()) is skipped — a
    // call name is not a field read — but its receiver is still visited, so the
    // domain object (this.road) still contributes.
    if (ts.isPropertyAccessExpression(node) && !this.calleeSkip.has(node)) {
      const p = accessPath(ts, node);
      if (p) this.readsRaw.add(p);
    }

    // Domain constants (SCREAMING_SNAKE): a bare V_BELOW (Identifier) or a qualified
    // mod.V_BELOW (the property leaf). Powers the const-set touchpoint channel.
    if (ts.isIdentifier(node) && isDomainConst(node.text)) {
      this.consts.add(node.text);
    } else if (ts.isPropertyAccessExpression(node) && isDomainConst(node.name.text)) {
      this.consts.add(node.name.text);
    }

    // Assignment write targets:  a.b.c = …   /   a.b[i] = …
    if (ts.isBinaryExpression(node) && isAssignmentOp(ts, node.operatorToken.kind)) {
      this.recordTarget(node.left);
      // Only a plain `=` LHS is a pure write (excluded from reads); compound (+=)
      // is a read-modify-write and stays in readsRaw.
      if (node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isPropertyAccessExpression(node.left)) {
        const p = accessPath(ts, node.left);
        if (p) this.pureWrites.add(p);
      }
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
        // The callee is a CALL, not a field read — skip it for reads (the
        // PropertyAccess branch checks calleeSkip before recursing). Its receiver
        // stays visited, so this.road.compute() still yields "this.road".
        this.calleeSkip.add(callee);
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

// extractSymbolsFile emits one 'table' record per MODULE-LEVEL const/let/var whose
// initializer is an object or array literal (the cross-substrate axis's non-function
// entity). ret_keys = the object's property names / the array's string elements; the
// existing key-set + judge machinery then pairs e.g. a TS HANDLERS table with a Python
// _VERB_TEMPLATES. Mirrors extract.py's _extract_symbols exactly: module level only,
// SCREAMING_SNAKE name OR >= minKeys, key/value sets deduped + sorted + capped.
function extractSymbolsFile(ts, filePath, root) {
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

  const lineOf = (pos) => sf.getLineAndCharacterOfPosition(pos).line + 1;
  const out = [];
  const minKeys = 3, maxKeys = 400;

  // Only top-level statements — not nested in functions/classes. `export const X`
  // is still a VariableStatement at module scope, so it is covered.
  for (const stmt of sf.statements) {
    if (!ts.isVariableStatement(stmt)) continue;
    for (const decl of stmt.declarationList.declarations) {
      if (!ts.isIdentifier(decl.name) || !decl.initializer) continue;
      const name = decl.name.text;
      const { keys, vals } = literalKeys(ts, decl.initializer);
      if (!keys.length) continue;
      // Noise control: an UPPER-cased table name (HANDLERS, _VERB_TEMPLATES), or any
      // literal with enough keys to be a real registry (vs an incidental 1-2 element
      // config). Mirrors extract.py's `name.isupper() or len(keys) >= min_keys`.
      if (!(isUpperName(name) || keys.length >= minKeys)) continue;
      const start = lineOf(decl.getStart(sf));
      const end = lineOf(decl.getEnd());
      out.push({
        file: rel,
        qualname: name,
        name,
        kind: 'table',
        line: start,
        n_lines: end - start + 1,
        strings: uniqSorted(vals).slice(0, maxKeys),
        writes: [],
        reads: [],
        ret_keys: uniqSorted(keys).slice(0, maxKeys),
        calls: [],
        delegates: false,
      });
    }
  }
  return out;
}

// literalKeys returns the string keys of an object literal (+ its string-literal
// values, for the strings channel) or the string elements of an array literal —
// the table's footprint. Mirrors extract.py's _string_keys.
function literalKeys(ts, node) {
  const keys = [], vals = [];
  if (ts.isObjectLiteralExpression(node)) {
    for (const prop of node.properties) {
      const k = propKey(ts, prop);
      if (k == null) continue;
      keys.push(k);
      if (ts.isPropertyAssignment(prop) && prop.initializer &&
          (ts.isStringLiteral(prop.initializer) ||
           ts.isNoSubstitutionTemplateLiteral(prop.initializer))) {
        const v = (prop.initializer.text || '').trim();
        if (v) vals.push(v);
      }
    }
  } else if (ts.isArrayLiteralExpression(node)) {
    for (const el of node.elements) {
      if (ts.isStringLiteral(el) || ts.isNoSubstitutionTemplateLiteral(el)) {
        const v = (el.text || '').trim();
        if (v) keys.push(v);
      }
    }
  }
  return { keys, vals };
}

// isUpperName mirrors Python str.isupper(): at least one cased letter and no
// lowercase — so HANDLERS and _VERB_TEMPLATES qualify, camelCase/handlers do not.
function isUpperName(name) {
  return /[A-Za-z]/.test(name) && name === name.toUpperCase();
}

// isDomainConst mirrors the Go/Python/Rust predicate: a SCREAMING_SNAKE name >= 3
// chars (V_BELOW, MAX_RETRIES, GRID). Keys the const-set touchpoint channel.
function isDomainConst(s) {
  return s.length >= 3 && /^[A-Z][A-Z0-9_]*$/.test(s);
}

function uniqSorted(arr) {
  return [...new Set(arr)].sort();
}

main();
