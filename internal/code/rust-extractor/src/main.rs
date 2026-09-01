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
use quote::ToTokens;
use serde::Serialize;
use std::collections::BTreeSet;
use std::io::{self, Read};
use std::path::Path;
use syn::visit::{self, Visit};
use syn::{
    Attribute, BinOp, Block, Expr, FnArg, ImplItem, Item, Lit, Member, Meta, ReturnType,
    Signature, Stmt, Type,
};

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
    // node_count is the number of AST nodes visited in the body — a more precise
    // substantiality proxy than n_lines (a one-line ternary and a ten-statement
    // one-liner both count as "1 line"). Mirrors the Go/Python/TS extractors'
    // per-node counters.
    node_count: usize,
    strings: Vec<String>,
    writes: Vec<String>,
    reads: Vec<String>,
    ret_keys: Vec<String>,
    calls: Vec<String>,
    consts: Vec<String>,
    // decl_consts: the file's module-scope SCREAMING_SNAKE const/static declarations
    // (repeated on each record). The touchpoint pass gates the const seam channel on
    // these project-DECLARED names so std/extern-crate constant references that are
    // never declared in-corpus don't form clusters — the const analog of requiring a
    // call seam to resolve to a project def.
    decl_consts: Vec<String>,
    delegates: bool,
    // test: this function is TEST code — it carries a #[test]-family attribute or
    // lives inside a #[cfg(test)] module (the dominant Rust unit-test shape, where
    // tests sit in the SAME .rs file as the production code they exercise, so no
    // file-path rule can see them). The Go side gates test↔test pairs on this.
    test: bool,
    // sig: the "(paramType,...)=>returnType" Type-4 signature-rarity string
    // (mirrors extract_ts.mjs's signatureOf / Go's goSignatureOf / Python's
    // _sig_of). Declared types only, receiver excluded, lifetimes canonicalized.
    sig: String,
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
    consts: BTreeSet<String>,
    delegates: bool,
    node_count: usize,
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
        self.node_count += 1;
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
            Expr::Path(p) => {
                // A referenced SCREAMING_SNAKE path leaf is a domain constant: bare
                // V_BELOW or qualified geom::V_BELOW (last segment). Rust's UPPER_SNAKE
                // const convention makes this channel strongest here. Powers the
                // const-set touchpoint channel.
                if let Some(seg) = p.path.segments.last() {
                    let id = seg.ident.to_string();
                    if is_domain_const(&id) {
                        self.consts.insert(id);
                    }
                }
            }
            _ => {}
        }
        // Recurse into children (closures/nested exprs are conflated into the
        // enclosing fn, matching extract.py's whole-body visitor).
        visit::visit_expr(self, node);
    }

    // syn's default visit_stmt/visit_local already delegate into visit_expr for
    // nested expressions, so overriding visit_expr alone misses only statement-
    // WRAPPER nodes (Local/Item/Macro) — this override closes that gap so
    // node_count matches "every AST node visited", not "every expression".
    fn visit_stmt(&mut self, node: &'ast Stmt) {
        self.node_count += 1;
        visit::visit_stmt(self, node);
    }
}

// is_domain_const mirrors the Go/Python/TS predicate: a SCREAMING_SNAKE name >= 3
// chars (V_BELOW, MAX_RETRIES, GRID). Keys the const-set touchpoint channel.
fn is_domain_const(s: &str) -> bool {
    s.len() >= 3
        && s.chars().next().map_or(false, |c| c.is_ascii_uppercase())
        && s.chars().all(|c| c.is_ascii_uppercase() || c.is_ascii_digit() || c == '_')
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

/// Whether c can continue a lifetime identifier ('a, 'b, '_, 'static, ...).
fn is_lifetime_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '_'
}

/// Replaces every `'ident` lifetime with a fixed `'_` placeholder, so
/// `fn foo<'a>(x: &'a str) -> &'a str` and `fn bar<'b>(x: &'b str) -> &'b str`
/// — the same contract — bucket together. Deliberately collapses "same
/// lifetime reused vs different lifetimes"; the loss is acceptable for a
/// Type-4 contract-matching purpose. Types never contain char literals, so
/// every `'` here is a lifetime, not a char-literal quote.
fn canonicalize_lifetimes(s: &str) -> String {
    let chars: Vec<char> = s.chars().collect();
    let mut out = String::with_capacity(s.len());
    let mut i = 0;
    while i < chars.len() {
        if chars[i] == '\'' {
            out.push_str("'_");
            i += 1;
            while i < chars.len() && is_lifetime_char(chars[i]) {
                i += 1;
            }
        } else {
            out.push(chars[i]);
            i += 1;
        }
    }
    out
}

/// Renders a type back to source text via quote's ToTokens (deterministic
/// spacing for structurally-identical types, unlike TS's raw-slice case) and
/// collapses whitespace RUNS to a single space — not a strip-to-zero, which
/// would glue `&` + `'_` into `&'_` with no boundary the way `& 'a str` needs
/// one (TS's strip-to-zero is safe only because JS/TS punctuation never
/// requires an inter-token space).
fn render_type(ty: &Type) -> String {
    let raw = canonicalize_lifetimes(&ty.to_token_stream().to_string());
    raw.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// signature_of builds the "(paramType,...)=>returnType" Type-4
/// signature-rarity string (mirrors extract_ts.mjs's signatureOf / Go's
/// goSignatureOf / Python's _sig_of) from declared types only. The receiver
/// (self/&self/&mut self) is excluded, matching how TS excludes `this` and Go
/// excludes the method receiver.
fn signature_of(sig: &Signature) -> String {
    let params: Vec<String> = sig
        .inputs
        .iter()
        .filter_map(|arg| match arg {
            FnArg::Receiver(_) => None,
            FnArg::Typed(pt) => Some(render_type(&pt.ty)),
        })
        .collect();
    let ret = match &sig.output {
        ReturnType::Default => "()".to_string(),
        ReturnType::Type(_, ty) => render_type(ty),
    };
    format!("({})=>{}", params.join(","), ret)
}

/// Bundles emit_fn's per-function inputs — grouped once the signature-rarity
/// axis pushed the parameter count past a plain positional list's readable
/// limit (Signature and Block both come from the same syn::Item, so passing
/// them as a group also mirrors how walk_items already holds them together).
struct EmitFnArgs<'a> {
    ident: &'a Ident,
    sig: &'a Signature,
    block: &'a Block,
    rel: &'a str,
    impl_type: Option<&'a str>,
    decl_consts: &'a [String],
    is_test: bool,
}

fn emit_fn(args: EmitFnArgs, out: &mut Vec<Record>) {
    let name = args.ident.to_string();
    let qualname = match args.impl_type {
        Some(t) => format!("{}.{}", t, name),
        None => name.clone(),
    };
    let line = args.ident.span().start().line;
    let close = args.block.brace_token.span.close().start().line;
    let n_lines = if close >= line { close - line + 1 } else { 1 };

    let mut body = Body::default();
    body.visit_block(args.block);
    // The trailing (semicolon-less) expression is Rust's implicit return.
    if let Some(Stmt::Expr(e, None)) = args.block.stmts.last() {
        body.collect_struct_keys(e);
    }

    out.push(Record {
        file: args.rel.to_string(),
        qualname,
        name,
        line,
        n_lines,
        node_count: body.node_count,
        reads: body.read_set(),
        strings: body.strings.into_iter().collect(),
        writes: body.writes.into_iter().collect(),
        ret_keys: body.ret_keys.into_iter().collect(),
        calls: body.calls.into_iter().collect(),
        consts: body.consts.into_iter().collect(),
        decl_consts: args.decl_consts.to_vec(),
        delegates: body.delegates,
        test: args.is_test,
        sig: signature_of(args.sig),
    });
}

/// has_cfg_test reports whether any attribute is `#[cfg(test)]` (or a cfg whose
/// predicate mentions `test`, e.g. `#[cfg(all(test, …))]`) — the marker on the
/// conventional `mod tests` unit-test module. Token-stream match keeps it robust to
/// nested cfg predicates without re-implementing cfg parsing.
fn has_cfg_test(attrs: &[Attribute]) -> bool {
    attrs.iter().any(|a| {
        a.path().is_ident("cfg")
            && matches!(&a.meta, Meta::List(l) if l.tokens.to_string().contains("test"))
    })
}

/// has_test_attr reports whether any attribute is a #[test]-family marker on the
/// function itself — bare `#[test]`, `#[tokio::test]`, `#[async_std::test]`, etc.
/// (last path segment == `test`). Backstops a #[test] fn not wrapped in a
/// #[cfg(test)] module.
fn has_test_attr(attrs: &[Attribute]) -> bool {
    attrs
        .iter()
        .any(|a| a.path().segments.last().map_or(false, |s| s.ident == "test"))
}

// collect_decl_consts gathers file-scope SCREAMING_SNAKE const/static declaration
// names, recursing into inline modules (mirroring walk_items' Mod descent). These
// are the project-DECLARED domain constants the touchpoint pass gates on.
fn collect_decl_consts(items: &[Item], out: &mut BTreeSet<String>) {
    for item in items {
        match item {
            Item::Const(c) => {
                let id = c.ident.to_string();
                if is_domain_const(&id) {
                    out.insert(id);
                }
            }
            Item::Static(s) => {
                let id = s.ident.to_string();
                if is_domain_const(&id) {
                    out.insert(id);
                }
            }
            Item::Mod(m) => {
                if let Some((_, items)) = &m.content {
                    collect_decl_consts(items, out);
                }
            }
            _ => {}
        }
    }
}

fn walk_items(items: &[Item], rel: &str, decl_consts: &[String], in_test: bool, out: &mut Vec<Record>) {
    for item in items {
        match item {
            Item::Fn(f) => {
                let t = in_test || has_test_attr(&f.attrs);
                emit_fn(
                    EmitFnArgs {
                        ident: &f.sig.ident,
                        sig: &f.sig,
                        block: &f.block,
                        rel,
                        impl_type: None,
                        decl_consts,
                        is_test: t,
                    },
                    out,
                )
            }
            Item::Impl(im) => {
                let ty = type_name(&im.self_ty);
                let impl_test = in_test || has_cfg_test(&im.attrs);
                for ii in &im.items {
                    if let ImplItem::Fn(m) = ii {
                        let t = impl_test || has_test_attr(&m.attrs);
                        emit_fn(
                            EmitFnArgs {
                                ident: &m.sig.ident,
                                sig: &m.sig,
                                block: &m.block,
                                rel,
                                impl_type: ty.as_deref(),
                                decl_consts,
                                is_test: t,
                            },
                            out,
                        );
                    }
                }
            }
            Item::Mod(m) => {
                if let Some((_, items)) = &m.content {
                    // A #[cfg(test)] module marks everything beneath it as test code.
                    walk_items(items, rel, decl_consts, in_test || has_cfg_test(&m.attrs), out);
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
        let mut decl_set = BTreeSet::new();
        collect_decl_consts(&file.items, &mut decl_set);
        let decl_consts: Vec<String> = decl_set.into_iter().collect();
        walk_items(&file.items, &rel, &decl_consts, false, &mut out);
    }

    serde_json::to_writer(io::stdout(), &out).ok();
}
