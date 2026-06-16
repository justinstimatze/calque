//! calque Rust extractor — emits one FuncSig JSON record per function/method.
//!
//! Invoked by the Go scorer as:  calque-rust-extractor <root>   with newline-
//! separated file paths on stdin. Output: a JSON array of {file,qualname,name,
//! line,n_lines,strings,writes,ret_keys,calls,delegates} — the SAME interchange
//! the go/ast and python3 (extract.py) extractors produce, so the Go side's
//! runJSONExtractor handles all three identically.
//!
//! syn is the de-facto Rust parser (what rustc-adjacent tooling builds on), so the
//! Go binary shells out to this built-once-cached helper for .rs targets rather
//! than hand-rolling a brace scanner — a real AST keeps the scoring gate's inputs
//! precise. Field semantics deliberately mirror extract.py:
//!   - `writes` strip the root object name: `self.foo = x` -> "foo" (extract.py's
//!     _attr_path drops the leading Name).
//!   - `delegates` is true when a method call forwards through a delegation-root
//!     field: `self.inner.foo()` / `self._engine.step()` (first attribute segment
//!     after the root is in DELEGATION_ROOTS).
//!   - `strings` are trimmed string literals with >= 4 chars; all sets sorted+unique.

use proc_macro2::Ident;
use serde::Serialize;
use std::collections::BTreeSet;
use std::io::{self, Read};
use std::path::Path;
use syn::visit::{self, Visit};
use syn::{BinOp, Block, Expr, ImplItem, Item, Lit, Member, Stmt, Type};

/// Field names marking a method as forwarding to a wrapped impl. Mirrors the Go
/// side's delegationRoots (funcsig.go) + extract.py's _DELEGATION_ROOTS, plus the
/// Rust idioms `self.inner` and `self.0` (newtype) that don't use a leading `_`.
const DELEGATION_ROOTS: &[&str] = &[
    "_engine", "_impl", "_inner", "_delegate", "_wrapped", "_backend", "_real",
    "_target", "inner", "0",
];

#[derive(Serialize)]
struct Record {
    file: String,
    qualname: String,
    name: String,
    line: usize,
    n_lines: usize,
    strings: Vec<String>,
    writes: Vec<String>,
    reads: Vec<String>,
    ret_keys: Vec<String>,
    calls: Vec<String>,
    delegates: bool,
}

#[derive(Default)]
struct Body {
    strings: BTreeSet<String>,
    writes: BTreeSet<String>,
    // reads_raw is every field-path seen in a value position; pure_writes is the
    // plain-`=` LHS. read_set() returns reads_raw \ pure_writes (the derivation
    // input footprint), mirroring writes on the read side.
    reads_raw: BTreeSet<String>,
    pure_writes: BTreeSet<String>,
    ret_keys: BTreeSet<String>,
    calls: BTreeSet<String>,
    delegates: bool,
}

impl Body {
    /// Record an assignment target the way extract.py does: attribute writes keep
    /// the dotted field path WITHOUT the root object (`self.a.b = x` -> "a.b"),
    /// index writes append "[]" (`self.grid[i] = x` -> "grid[]"). Bare local
    /// writes (no field access) are not recorded.
    fn record_target(&mut self, left: &Expr) {
        match left {
            Expr::Field(_) | Expr::Path(_) => {
                if let Some(m) = field_members(left) {
                    if !m.is_empty() {
                        self.writes.insert(m.join("."));
                    }
                }
            }
            Expr::Index(idx) => {
                if let Some(m) = field_members(&idx.expr) {
                    if !m.is_empty() {
                        self.writes.insert(format!("{}[]", m.join(".")));
                    }
                }
            }
            Expr::Tuple(t) => {
                for e in &t.elems {
                    self.record_target(e);
                }
            }
            Expr::Reference(r) => self.record_target(&r.expr),
            Expr::Paren(p) => self.record_target(&p.expr),
            _ => {}
        }
    }

    /// The derivation read-set: field-paths read in a value position, excluding
    /// those that appear only as a plain-`=` assignment target.
    fn read_set(&self) -> Vec<String> {
        self.reads_raw
            .iter()
            .filter(|r| !self.pure_writes.contains(*r))
            .cloned()
            .collect()
    }

    /// A returned struct literal contributes its field names as ret_keys (the
    /// analog of extract.py's returned-dict string keys).
    fn collect_struct_keys(&mut self, e: &Expr) {
        if let Expr::Struct(s) = e {
            for f in &s.fields {
                self.ret_keys.insert(member_str(&f.member));
            }
        }
    }
}

impl<'ast> Visit<'ast> for Body {
    fn visit_expr(&mut self, node: &'ast Expr) {
        match node {
            Expr::Lit(l) => {
                if let Lit::Str(s) = &l.lit {
                    let v = s.value();
                    let t = v.trim();
                    if t.chars().count() >= 4 {
                        self.strings.insert(t.to_string());
                    }
                }
            }
            Expr::Field(_) => {
                // A field-path read in a value position. The plain-`=` LHS is
                // removed later via pure_writes; compound-assign targets stay
                // (read-modify-write). Recursion also captures path prefixes,
                // matching the go/ast walk.
                if let Some(m) = field_members(node) {
                    if !m.is_empty() {
                        self.reads_raw.insert(m.join("."));
                    }
                }
            }
            Expr::Assign(a) => {
                self.record_target(&a.left);
                // Only a plain `=` LHS is a pure write (excluded from reads).
                if let Some(m) = field_members(&a.left) {
                    if !m.is_empty() {
                        self.pure_writes.insert(m.join("."));
                    }
                }
            }
            Expr::Binary(b) if is_assign_op(&b.op) => self.record_target(&b.left),
            Expr::MethodCall(mc) => {
                self.calls.insert(mc.method.to_string());
                if let Some(m) = field_members(&mc.receiver) {
                    if let Some(first) = m.first() {
                        if DELEGATION_ROOTS.contains(&first.as_str()) {
                            self.delegates = true;
                        }
                    }
                }
            }
            Expr::Call(c) => {
                if let Expr::Path(p) = &*c.func {
                    if let Some(seg) = p.path.segments.last() {
                        self.calls.insert(seg.ident.to_string());
                    }
                }
            }
            Expr::Return(r) => {
                if let Some(e) = &r.expr {
                    self.collect_struct_keys(e);
                }
            }
            _ => {}
        }
        // Recurse into children (closures/nested exprs are conflated into the
        // enclosing fn, matching extract.py's whole-body visitor).
        visit::visit_expr(self, node);
    }
}

fn is_assign_op(op: &BinOp) -> bool {
    matches!(
        op,
        BinOp::AddAssign(_)
            | BinOp::SubAssign(_)
            | BinOp::MulAssign(_)
            | BinOp::DivAssign(_)
            | BinOp::RemAssign(_)
            | BinOp::BitXorAssign(_)
            | BinOp::BitAndAssign(_)
            | BinOp::BitOrAssign(_)
            | BinOp::ShlAssign(_)
            | BinOp::ShrAssign(_)
    )
}

fn member_str(m: &Member) -> String {
    match m {
        Member::Named(i) => i.to_string(),
        Member::Unnamed(idx) => idx.index.to_string(),
    }
}

/// The dotted field path of `expr`, EXCLUDING the root object identifier:
/// `self.a.b` -> ["a","b"], `self` -> [], a non-field expr -> None. Mirrors
/// extract.py's _attr_path (which drops the leading Name).
fn field_members(expr: &Expr) -> Option<Vec<String>> {
    match expr {
        Expr::Path(p) if p.qself.is_none() => Some(vec![]),
        Expr::Field(f) => {
            let mut v = field_members(&f.base)?;
            v.push(member_str(&f.member));
            Some(v)
        }
        _ => None,
    }
}

fn type_name(ty: &Type) -> Option<String> {
    if let Type::Path(tp) = ty {
        tp.path.segments.last().map(|s| s.ident.to_string())
    } else {
        None
    }
}

fn emit_fn(ident: &Ident, block: &Block, rel: &str, impl_type: Option<&str>, out: &mut Vec<Record>) {
    let name = ident.to_string();
    let qualname = match impl_type {
        Some(t) => format!("{}.{}", t, name),
        None => name.clone(),
    };
    let line = ident.span().start().line;
    let close = block.brace_token.span.close().start().line;
    let n_lines = if close >= line { close - line + 1 } else { 1 };

    let mut body = Body::default();
    body.visit_block(block);
    // The trailing (semicolon-less) expression is Rust's implicit return.
    if let Some(Stmt::Expr(e, None)) = block.stmts.last() {
        body.collect_struct_keys(e);
    }

    out.push(Record {
        file: rel.to_string(),
        qualname,
        name,
        line,
        n_lines,
        reads: body.read_set(),
        strings: body.strings.into_iter().collect(),
        writes: body.writes.into_iter().collect(),
        ret_keys: body.ret_keys.into_iter().collect(),
        calls: body.calls.into_iter().collect(),
        delegates: body.delegates,
    });
}

fn walk_items(items: &[Item], rel: &str, out: &mut Vec<Record>) {
    for item in items {
        match item {
            Item::Fn(f) => emit_fn(&f.sig.ident, &f.block, rel, None, out),
            Item::Impl(im) => {
                let ty = type_name(&im.self_ty);
                for ii in &im.items {
                    if let ImplItem::Fn(m) = ii {
                        emit_fn(&m.sig.ident, &m.block, rel, ty.as_deref(), out);
                    }
                }
            }
            Item::Mod(m) => {
                if let Some((_, items)) = &m.content {
                    walk_items(items, rel, out);
                }
            }
            // Trait default methods are out of scope for v1.
            _ => {}
        }
    }
}

fn rel_path(path: &str, root: &str) -> String {
    Path::new(path)
        .strip_prefix(root)
        .map(|p| p.to_string_lossy().into_owned())
        .unwrap_or_else(|_| path.to_string())
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let root = args.get(1).cloned().unwrap_or_else(|| ".".to_string());

    let mut input = String::new();
    io::stdin().read_to_string(&mut input).ok();

    let mut out: Vec<Record> = Vec::new();
    for path in input.split_whitespace() {
        let src = match std::fs::read_to_string(path) {
            Ok(s) => s,
            Err(_) => continue,
        };
        let file = match syn::parse_file(&src) {
            Ok(f) => f,
            Err(_) => continue, // skip unparseable files (matches go/py)
        };
        let rel = rel_path(path, &root);
        walk_items(&file.items, &rel, &mut out);
    }

    serde_json::to_writer(io::stdout(), &out).ok();
}
