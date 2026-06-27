package code

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// exeSuffix is ".exe" on Windows, "" elsewhere — cargo names the built helper
// calque-rust-extractor.exe on Windows, and exec.Command needs the suffix to find
// and run it, so both the cache path and the cargo output path carry it.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// rustExtractorFS carries the syn-based Rust extractor crate's SOURCE (not its
// deps) inside the Go binary, so `go install …@latest` ships it. Unlike .py/.ts
// (run a script via an always-present interpreter), Rust needs a compiled helper,
// so the crate is built once and cached on the first .rs scan — see
// rustExtractorBin. Cargo.lock is embedded and the build is `--locked`, pinning
// the syn version for reproducible extraction.
//
//go:embed rust-extractor/Cargo.toml rust-extractor/Cargo.lock rust-extractor/src/main.rs
var rustExtractorFS embed.FS

// rustEmbedFiles is the deterministic file list (also the cache-key input).
var rustEmbedFiles = []string{
	"rust-extractor/Cargo.toml",
	"rust-extractor/Cargo.lock",
	"rust-extractor/src/main.rs",
}

var (
	rustBinOnce sync.Once
	rustBinPath string
	rustBinErr  error
)

// cargoBin is the Cargo build tool to run (CALQUE_CARGO override, else cargo).
func cargoBin() string {
	if c := os.Getenv("CALQUE_CARGO"); c != "" {
		return c
	}
	return "cargo"
}

// extractRustBatch extracts FUNCTIONS from .rs paths (the code axis) by shelling
// out to the cached syn helper, which emits the same FuncSig JSON the go/ast and
// python3 extractors produce — so runJSONExtractor handles all three identically.
func extractRustBatch(paths []string, root string) ([]*FuncSig, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	bin, err := rustExtractorBin()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, root)
	return runJSONExtractor(cmd, paths, bin, "Rust", "install rustup/cargo or set CALQUE_RUST_EXTRACTOR to a prebuilt binary")
}

// rustExtractorBin resolves the helper binary once per process: an explicit
// CALQUE_RUST_EXTRACTOR override (CI / power users supply a prebuilt binary and
// skip the build), else a cached build keyed by a hash of the embedded source
// (reused across runs), else a one-time `cargo build --release --locked`.
func rustExtractorBin() (string, error) {
	rustBinOnce.Do(func() {
		if e := os.Getenv("CALQUE_RUST_EXTRACTOR"); e != "" {
			rustBinPath = e
			return
		}
		hash, err := rustExtractorHash()
		if err != nil {
			rustBinErr = err
			return
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			rustBinErr = fmt.Errorf("locating user cache dir for the Rust extractor: %w", err)
			return
		}
		bin := filepath.Join(cache, "calque", "rust-extractor-"+hash[:16], "calque-rust-extractor"+exeSuffix())
		if _, err := os.Stat(bin); err == nil {
			rustBinPath = bin
			return
		}
		if err := buildRustExtractor(bin); err != nil {
			rustBinErr = err
			return
		}
		rustBinPath = bin
	})
	return rustBinPath, rustBinErr
}

// rustExtractorHash is the sha256 over the embedded crate files (names + bytes),
// so any change to Cargo.toml / Cargo.lock / main.rs invalidates the cached build.
func rustExtractorHash() (string, error) {
	h := sha256.New()
	for _, name := range rustEmbedFiles {
		b, err := rustExtractorFS.ReadFile(name)
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// buildRustExtractor materializes the embedded crate to a temp dir, builds it
// `--locked` (pinned syn), and installs the binary at binPath via a temp+rename
// so a concurrent first scan never sees a half-written binary.
func buildRustExtractor(binPath string) error {
	tmp, err := os.MkdirTemp("", "calque-rust-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := writeEmbeddedRust(tmp); err != nil {
		return fmt.Errorf("writing Rust extractor source: %w", err)
	}

	cmd := exec.Command(cargoBin(), "build", "--release", "--locked")
	cmd.Dir = tmp
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if _, look := exec.LookPath(cargoBin()); look != nil {
			return fmt.Errorf("cargo not found — needed to build the Rust extractor on first .rs scan (install rustup, or set CALQUE_RUST_EXTRACTOR to a prebuilt binary)")
		}
		return fmt.Errorf("building Rust extractor: %v: %s", err, strings.TrimSpace(errb.String()))
	}

	built := filepath.Join(tmp, "target", "release", "calque-rust-extractor"+exeSuffix())
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return err
	}
	return installBinary(built, binPath)
}

// writeEmbeddedRust writes the embedded rust-extractor/ tree into dst as the crate
// root (stripping the "rust-extractor/" prefix).
func writeEmbeddedRust(dst string) error {
	return fs.WalkDir(rustExtractorFS, "rust-extractor", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("rust-extractor", p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := rustExtractorFS.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// installBinary copies src to dst atomically (write a sibling temp, then rename).
func installBinary(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicWrite(dst, in, 0o755)
}
