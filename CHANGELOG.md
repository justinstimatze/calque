# Changelog

All notable changes to calque. The version string itself comes from the git tag
(`git describe`), not this file — see `Makefile` / `cmd/calque/main.go`.

## [Unreleased]

### Changed
- **Rewrite in Go, underway.** calque becomes a substrate-general drift engine
  (code · prose · planned config) sharing one spine (recall → registry → check →
  calibrate). Decision + rationale in `docs/DESIGN_NOTES.md` §16. Scaffolded the
  Go module (`cmd/calque`, hindcast-style git-tag versioning); subcommands land
  one axis at a time. The prose axis, calibration, and hook are consolidated from
  the sibling project `cupel` (MIT → Apache-2.0, attribution preserved); the code
  axis is ported from the original Python nose.

### Added
- `.calque/registry.md` is now git-tracked for calque's own self-dogfood
  (`.gitignore`: `.calque/*` + `!.calque/registry.md`).
